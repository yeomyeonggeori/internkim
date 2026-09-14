package admind

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

const recordToolPathPrefix = "/record/api/tools/"

// A tool whose rows live in the record runs on the plane. capabilityd reaches
// this over the requester socket, which is the only place an asserted
// requester is honoured, and the plane runs the tool as that member.
func (service *Service) handleRecordTool(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(responseWriter, "a record tool is invoked with POST", http.StatusMethodNotAllowed)
		return
	}
	toolName, verb := recordToolCallOf(request.URL.Path)
	if toolName == "" {
		http.Error(responseWriter, "this path names no tool", http.StatusNotFound)
		return
	}
	requesterEmail := assertedRequesterEmail(request)
	if requesterEmail == "" {
		http.Error(responseWriter, "a record tool runs as the person who asked, and this call asserted nobody", http.StatusForbidden)
		return
	}
	client := service.centralPlane()
	if client == nil {
		http.Error(responseWriter, "this device is not attached to a central plane, so the record is out of reach", http.StatusServiceUnavailable)
		return
	}

	input, errorValue := io.ReadAll(http.MaxBytesReader(responseWriter, request.Body, recordToolInputCeiling))
	if errorValue != nil {
		http.Error(responseWriter, "that input is larger than a tool call carries", http.StatusRequestEntityTooLarge)
		return
	}

	answer, errorValue := client.InvokeRecordTool(request.Context(), centralplane.RecordToolCall{
		RequesterEmail: requesterEmail,
		ToolName:       toolName,
		Verb:           verb,
		Input:          json.RawMessage(input),
		IdempotencyKey: strings.TrimSpace(request.Header.Get(idempotencyKeyHeader)),
	})
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(answer.Status)
	responseWriter.Write(answer.Body)
}

const recordToolInputCeiling = 1 << 20

var recordToolVerbs = map[string]bool{"invoke": true, "target": true}

func recordToolCallOf(path string) (string, string) {
	rest := strings.TrimPrefix(path, recordToolPathPrefix)
	if rest == path {
		return "", ""
	}
	name, verb, found := strings.Cut(rest, "/")
	if !found || !recordToolVerbs[verb] {
		return "", ""
	}
	return strings.TrimSpace(name), verb
}
