package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const aWrittenSchedule = `{"scheduleID":"schedule-1","description":"주간 보고","taskInstruction":"주간 보고서를 정리해 올린다","timeZone":"Asia/Seoul","kind":"cron","cronExpression":"0 9 * * 1","nextRunAt":"2026-09-21T00:00:00Z","conversationID":"channel-1","replyTargetID":"message-1","agentProfileName":"internkim"}`

func scheduleRequestInAConversation(toolName string, input string) capabilities.ToolInvokeRequest {
	request := recordRequestOf(toolName, input)
	request.Context.Platform = "buzz"
	request.Context.ConversationID = "channel-1"
	request.Context.ReplyTargetID = "message-1"
	return request
}

func admindWritingASchedule(t *testing.T, reachedPath *string, reachedBody *string) string {
	t.Helper()
	return admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		*reachedPath = request.URL.Path
		*reachedBody = string(body)
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(aWrittenSchedule))
	}))
}

func TestScheduleListReachesAdmindAsRequesterAndKeepsExactResult(t *testing.T) {
	var reachedPath, reachedRequester, reachedBody string
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		reachedPath = request.URL.Path
		reachedRequester = request.Header.Get(admindRequesterEmailHeader)
		reachedBody = string(body)
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"schedules":[{"scheduleID":"schedule-1","taskInstruction":"full instruction","cadence":"cron","status":"failed"}]}`))
	}))
	service := Service{Configuration: Configuration{AdmindSocketPath: socketPath}}

	answer, errorValue := service.invokeScheduleTool(context.Background(), recordRequestOf("schedule_list", `{"status":"failed","limit":1}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if reachedPath != "/memory/api/schedules/tool-list" || reachedRequester != "member@example.com" || reachedBody != `{"status":"failed","limit":1}` {
		t.Fatalf("schedule call reached path=%q requester=%q body=%q", reachedPath, reachedRequester, reachedBody)
	}
	if answer.Outcome != capabilities.ToolOutcomeSucceeded || !strings.Contains(string(answer.Result), `"taskInstruction":"full instruction"`) {
		t.Fatalf("unexpected answer: %+v", answer)
	}

	if _, errorValue := service.invokeScheduleTool(context.Background(), recordRequestOf("schedule_list", "")); errorValue != nil {
		t.Fatal(errorValue)
	}
	if reachedBody != "{}" {
		t.Fatalf("a schedule_list call with no arguments reached admind as %q", reachedBody)
	}
}

func TestScheduleCreateCarriesTheConversationItWasAskedIn(t *testing.T) {
	var reachedPath, reachedBody string
	service := Service{Configuration: Configuration{AdmindSocketPath: admindWritingASchedule(t, &reachedPath, &reachedBody)}}
	asked := `{"taskInstruction":"주간 보고서를 정리해 올린다","kind":"cron","cronExpression":"0 9 * * 1","repeatPolicy":"unbounded"}`

	answer, errorValue := service.invokeScheduleTool(context.Background(), scheduleRequestInAConversation("schedule_create", asked))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if answer.Outcome != capabilities.ToolOutcomeSucceeded {
		t.Fatalf("answered %s: %s", answer.Outcome, answer.Message)
	}
	if reachedPath != "/memory/api/schedules/tool-create" {
		t.Fatalf("schedule_create reached %q", reachedPath)
	}
	carried := scheduleBodyReaching(t, reachedBody)
	if carried["platform"] != "buzz" || carried["conversationID"] != "channel-1" || carried["replyTargetID"] != "message-1" {
		t.Fatalf("the conversation did not reach admind: %s", reachedBody)
	}
	if carried["taskInstruction"] != "주간 보고서를 정리해 올린다" || carried["cronExpression"] != "0 9 * * 1" {
		t.Fatalf("what the model asked for did not reach admind: %s", reachedBody)
	}
}

func TestScheduleCreateOutsideAConversationBindsNothing(t *testing.T) {
	var reachedPath, reachedBody string
	service := Service{Configuration: Configuration{AdmindSocketPath: admindWritingASchedule(t, &reachedPath, &reachedBody)}}

	if _, errorValue := service.invokeScheduleTool(context.Background(), recordRequestOf("schedule_create", `{"taskInstruction":"주간 보고서를 정리해 올린다","kind":"once","runAt":"2026-09-21T09:00:00+09:00"}`)); errorValue != nil {
		t.Fatal(errorValue)
	}
	carried := scheduleBodyReaching(t, reachedBody)
	for _, field := range []string{"platform", "conversationID", "replyTargetID"} {
		if _, isBound := carried[field]; isBound {
			t.Fatalf("a call outside any conversation bound %s: %s", field, reachedBody)
		}
	}
}

func TestScheduleUpdateCarriesOnlyWhatTheModelAsked(t *testing.T) {
	var reachedPath, reachedBody string
	service := Service{Configuration: Configuration{AdmindSocketPath: admindWritingASchedule(t, &reachedPath, &reachedBody)}}
	asked := `{"scheduleHint":"주간 보고","intervalSecond":3600}`

	if _, errorValue := service.invokeScheduleTool(context.Background(), scheduleRequestInAConversation("schedule_update", asked)); errorValue != nil {
		t.Fatal(errorValue)
	}
	if reachedPath != "/memory/api/schedules/tool-update" || reachedBody != asked {
		t.Fatalf("schedule_update reached path=%q body=%q", reachedPath, reachedBody)
	}
}

func TestScheduleCancelCarriesTheCandidatesBackToTheModel(t *testing.T) {
	refusal := `{"error":"주간 보고 is the description of two schedules","errorCode":"interaction_required","candidates":[{"scheduleID":"schedule-1"},{"scheduleID":"schedule-2"}]}`
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusConflict)
		_, _ = responseWriter.Write([]byte(refusal))
	}))
	service := Service{Configuration: Configuration{AdmindSocketPath: socketPath}}

	answer, errorValue := service.invokeScheduleTool(context.Background(), scheduleRequestInAConversation("schedule_cancel", `{"scheduleHints":["주간 보고"]}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if answer.Outcome != capabilities.ToolOutcomeFailed || answer.ErrorCode != "interaction_required" || answer.FailureStage != "target_resolution" {
		t.Fatalf("a hint that resolves to two schedules answered %+v", answer)
	}
	if !strings.Contains(string(answer.Result), `"scheduleID":"schedule-2"`) {
		t.Fatalf("the candidates did not reach the model: %s", answer.Result)
	}
}

func scheduleBodyReaching(t *testing.T, body string) map[string]any {
	t.Helper()
	document := map[string]any{}
	if errorValue := json.Unmarshal([]byte(body), &document); errorValue != nil {
		t.Fatalf("admind was reached with %q: %v", body, errorValue)
	}
	return document
}
