package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func recordRequestOf(toolName string, input string) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{
		ToolName: toolName,
		Input:    json.RawMessage(input),
		Context:  capabilityprotocol.ToolInvokeContext{RequesterEmail: "member@example.com"},
	}
}

func TestRecordCallsReachTheRecordAsTheRequester(t *testing.T) {
	var reachedPath, reachedRequester, reachedBody string
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		reachedPath = request.URL.Path
		reachedRequester = request.Header.Get(admindRequesterEmailHeader)
		reachedBody = string(body)
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"tool":"leave_balance","result":{"scope":"person","year":2026,"count":1,"balances":[{"personID":"p1","personName":"이샘플","grantedDays":15,"remainingDays":13,"usedDays":2,"tracking":"managed"}]}}`))
	}))
	service := Service{Configuration: Configuration{
		AdmindBaseURL:    admindOnLoopbackThatFailsTheTest(t),
		AdmindSocketPath: socketPath,
	}}

	answer, errorValue := service.invokeRecordTool(context.Background(), recordRequestOf("leave_balance", `{"year":2026}`))
	if errorValue != nil {
		t.Fatalf("leave_balance: %v", errorValue)
	}
	if reachedPath != "/record/api/tools/leave_balance/invoke" {
		t.Fatalf("the call went to %q", reachedPath)
	}
	if reachedRequester != "member@example.com" {
		t.Fatalf("admind was asked as %q", reachedRequester)
	}
	if reachedBody != `{"year":2026}` {
		t.Fatalf("the input arrived as %q", reachedBody)
	}
	if answer.Outcome != capabilities.ToolOutcomeSucceeded || answer.SelectedBackend != "record" {
		t.Fatalf("answered %+v", answer)
	}
	if !strings.Contains(string(answer.Result), `"remainingDays":13`) {
		t.Fatalf("the result came back as %s", answer.Result)
	}
}

// A hint the record could not resolve comes back with the candidates to choose
// from, and the model needs both, so neither is summarised away.
func TestAnUnresolvedRecordHintReachesTheModelWithItsCandidates(t *testing.T) {
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusConflict)
		_, _ = responseWriter.Write([]byte(`{"error":"연차 could be two of them","candidates":["이샘플 · 연차 · 2026-11-02","이샘플 · 연차 · 2026-11-09"]}`))
	}))
	service := Service{Configuration: Configuration{
		AdmindBaseURL:    admindOnLoopbackThatFailsTheTest(t),
		AdmindSocketPath: socketPath,
	}}

	answer, errorValue := service.invokeRecordTool(context.Background(), recordRequestOf("leave_decide", `{"leaveHint":"연차","decision":"approved"}`))
	if errorValue != nil {
		t.Fatalf("leave_decide: %v", errorValue)
	}
	if answer.Outcome != capabilities.ToolOutcomeFailed {
		t.Fatalf("a refusal answered as %s", answer.Outcome)
	}
	if answer.ErrorCode != "interaction_required" || answer.FailureStage != "target_resolution" {
		t.Fatalf("answered %s at %s", answer.ErrorCode, answer.FailureStage)
	}
	if !strings.Contains(answer.Message, "could be two of them") {
		t.Fatalf("the record's own words did not survive: %q", answer.Message)
	}
	if !strings.Contains(string(answer.Result), "2026-11-09") {
		t.Fatalf("the candidates did not survive: %s", answer.Result)
	}
}

func TestARecordCallRefusesRatherThanSendARequesterNobodyHonours(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	_, errorValue := service.invokeRecordTool(context.Background(), recordRequestOf("leave_list", `{}`))
	if errorValue == nil {
		t.Fatal("the call went somewhere without an admind socket to go to")
	}
	if !strings.Contains(errorValue.Error(), "socket") {
		t.Fatalf("the failure does not say the socket is missing: %v", errorValue)
	}
}

func TestOnlyTheRecordsOwnToolsAreServedHere(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	_, errorValue := service.invokeRecordTool(context.Background(), recordRequestOf("leave_forget", `{}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "leave_forget") {
		t.Fatalf("a tool this handler does not serve answered anyway: %v", errorValue)
	}
}

func TestEveryToolTheRecordRunsIsRoutedToTheRecord(t *testing.T) {
	recordHandler := reflect.ValueOf(capabilityToolHandler(Service.invokeRecordTool)).Pointer()
	for _, toolName := range toolNamesAnsweredBy(capabilityprotocol.AnsweredByRecord) {
		route, hasRoute := capabilityToolRouteFor(toolName)
		if !hasRoute {
			t.Fatalf("%s is answered by the record and nothing routes to it", toolName)
		}
		if reflect.ValueOf(route.Handler).Pointer() != recordHandler {
			t.Fatalf("%s is answered by the record and its route reaches somewhere else", toolName)
		}
	}
}

func admindOnASocket(t *testing.T, handler http.Handler) string {
	t.Helper()
	directory, errorValue := os.MkdirTemp("", "admind")
	if errorValue != nil {
		t.Fatalf("make the socket directory: %v", errorValue)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	socketPath := filepath.Join(directory, "admind.sock")
	listener, errorValue := net.Listen("unix", socketPath)
	if errorValue != nil {
		t.Fatalf("listen on %s: %v", socketPath, errorValue)
	}
	server := httptest.NewUnstartedServer(handler)
	_ = server.Listener.Close()
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return socketPath
}

func admindOnLoopbackThatFailsTheTest(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		t.Errorf("%s %s reached admind over TCP, where the requester header is ignored", request.Method, request.URL.Path)
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)
	return server.URL
}
