package capabilityd

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

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
