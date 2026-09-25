package capabilityd

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestTaskLabelGetAsksAdmindAsTheRequesterAndKeepsItsLabels(t *testing.T) {
	var reachedPath, reachedRequester, reachedBody string
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		reachedPath = request.URL.Path
		reachedRequester = request.Header.Get(admindRequesterEmailHeader)
		reachedBody = string(body)
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"business":"영업","type":"","size":"M"}`))
	}))
	service := Service{Configuration: Configuration{AdmindSocketPath: socketPath}}

	answer, errorValue := service.invokeTaskLabelTool(context.Background(), recordRequestOf("task_label_get", `{"title":"제안서","note":"다음 주 발표"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if reachedPath != "/task/api/labels" || reachedRequester != "member@example.com" || reachedBody != `{"title":"제안서","note":"다음 주 발표"}` {
		t.Fatalf("task_label_get reached path=%q requester=%q body=%q", reachedPath, reachedRequester, reachedBody)
	}
	if answer.Outcome != capabilities.ToolOutcomeSucceeded || !strings.Contains(string(answer.Result), `"size":"M"`) {
		t.Fatalf("unexpected answer: %+v", answer)
	}
}

func TestTaskLabelGetFailsWhenAdmindCannotDecide(t *testing.T) {
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		http.Error(responseWriter, "the decision model is unreachable", http.StatusBadGateway)
	}))
	service := Service{Configuration: Configuration{AdmindSocketPath: socketPath}}

	answer, errorValue := service.invokeTaskLabelTool(context.Background(), recordRequestOf("task_label_get", `{"title":"제안서"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if answer.Outcome == capabilities.ToolOutcomeSucceeded {
		t.Fatalf("a refused decision was answered as labels: %+v", answer)
	}
}
