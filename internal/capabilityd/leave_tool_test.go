package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func leaveRequestOf(toolName string, input string) capabilities.ToolInvokeRequest {
	return capabilities.ToolInvokeRequest{
		ToolName: toolName,
		Input:    json.RawMessage(input),
		Context:  capabilityprotocol.ToolInvokeContext{RequesterEmail: "staff@example.com"},
	}
}

func TestLeaveCallsReachTheRecordAsTheRequester(t *testing.T) {
	var reachedPath, reachedRequester, reachedBody string
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		reachedPath = request.URL.Path
		reachedRequester = request.Header.Get(admindRequesterEmailHeader)
		reachedBody = string(body)
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"tool":"leave_balance","result":{"remainingDays":13}}`))
	}))
	service := Service{Configuration: Configuration{
		AdmindBaseURL:    admindOnLoopbackThatFailsTheTest(t),
		AdmindSocketPath: socketPath,
	}}

	answer, errorValue := service.invokeLeaveTool(context.Background(), leaveRequestOf("leave_balance", `{"year":2026}`))
	if errorValue != nil {
		t.Fatalf("leave_balance: %v", errorValue)
	}
	if reachedPath != "/record/api/tools/leave_balance/invoke" {
		t.Fatalf("the call went to %q", reachedPath)
	}
	if reachedRequester != "staff@example.com" {
		t.Fatalf("admind was asked as %q", reachedRequester)
	}
	if reachedBody != `{"year":2026}` {
		t.Fatalf("the input arrived as %q", reachedBody)
	}
	if answer.Outcome != capabilities.ToolOutcomeSucceeded || answer.SelectedBackend != "record" {
		t.Fatalf("answered %+v", answer)
	}
	if string(answer.Result) != `{"remainingDays":13}` {
		t.Fatalf("the result came back as %s", answer.Result)
	}
}

// A hint the record could not resolve comes back with the candidates to choose
// from, and the model needs both, so neither is summarised away.
func TestAnUnresolvedLeaveHintReachesTheModelWithItsCandidates(t *testing.T) {
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusConflict)
		_, _ = responseWriter.Write([]byte(`{"error":"연차 could be two of them","candidates":["이샘플 · 연차 · 2026-11-02","이샘플 · 연차 · 2026-11-09"]}`))
	}))
	service := Service{Configuration: Configuration{
		AdmindBaseURL:    admindOnLoopbackThatFailsTheTest(t),
		AdmindSocketPath: socketPath,
	}}

	answer, errorValue := service.invokeLeaveTool(context.Background(), leaveRequestOf("leave_decide", `{"leaveHint":"연차","decision":"approved"}`))
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

func TestALeaveCallRefusesRatherThanSendARequesterNobodyHonours(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	_, errorValue := service.invokeLeaveTool(context.Background(), leaveRequestOf("leave_list", `{}`))
	if errorValue == nil {
		t.Fatal("the call went somewhere without an admind socket to go to")
	}
	if !strings.Contains(errorValue.Error(), "socket") {
		t.Fatalf("the failure does not say the socket is missing: %v", errorValue)
	}
}

func TestOnlyTheFourLeaveToolsAreServedHere(t *testing.T) {
	service := Service{Configuration: Configuration{AdmindBaseURL: admindOnLoopbackThatFailsTheTest(t)}}

	_, errorValue := service.invokeLeaveTool(context.Background(), leaveRequestOf("leave_forget", `{}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "leave_forget") {
		t.Fatalf("a tool this handler does not serve answered anyway: %v", errorValue)
	}
}
