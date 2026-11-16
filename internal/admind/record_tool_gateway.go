package admind

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
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

	body, errorValue := io.ReadAll(http.MaxBytesReader(responseWriter, request.Body, recordToolCallCeiling))
	if errorValue != nil {
		http.Error(responseWriter, "that call is larger than a tool call carries", http.StatusRequestEntityTooLarge)
		return
	}
	var call capabilityprotocol.RecordToolCall
	if errorValue := json.Unmarshal(body, &call); len(bytes.TrimSpace(body)) > 0 && errorValue != nil {
		http.Error(responseWriter, "a record tool call is a json object carrying its input", http.StatusBadRequest)
		return
	}

	answer, errorValue := client.InvokeRecordToolCall(request.Context(), requesterEmail, toolName, verb, call)
	if isTimeout(errorValue) {
		writeRecordToolOutcomeUnknown(responseWriter, errorValue)
		return
	}
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(answer.Status)
	responseWriter.Write(answer.Body)
}

func isTimeout(errorValue error) bool {
	var networkError net.Error
	return errors.As(errorValue, &networkError) && networkError.Timeout()
}

func writeRecordToolOutcomeUnknown(responseWriter http.ResponseWriter, errorValue error) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusGatewayTimeout)
	json.NewEncoder(responseWriter).Encode(map[string]any{
		"error":        "the record did not answer in time, so the call may have taken effect even though no result came back: " + errorValue.Error(),
		"errorCode":    "timeout",
		"failureStage": "record",
		"retryable":    false,
	})
}

const recordToolInputCeiling = 1 << 20

const recordToolCallCeiling = 24 << 20

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
