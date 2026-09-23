package admind

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

func TestTaskDetailInlinesExactLLMFailureEvidenceForAdmin(t *testing.T) {
	workspacePath := t.TempDir()
	exchangeServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"answer":"synthetic provider response"}`))
	}))
	defer exchangeServer.Close()
	capturedClient, capture := llmbackend.NewExchangeCapture(nil)
	response, errorValue := capturedClient.Post(exchangeServer.URL, "application/json", strings.NewReader(`{"prompt":"synthetic provider request"}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := io.ReadAll(response.Body); errorValue != nil {
		t.Fatal(errorValue)
	}
	_ = response.Body.Close()
	identifier, errorValue := llmbackend.WriteFailureEvidence(workspacePath, map[string]string{"prompt": "private request"}, capture)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedDocument, errorValue := os.ReadFile(llmbackend.FailureEvidenceDirectory(workspacePath) + "/" + identifier + ".json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var expectedEvidence struct {
		CreatedAt string `json:"createdAt"`
	}
	if errorValue := json.Unmarshal(expectedDocument, &expectedEvidence); errorValue != nil {
		t.Fatal(errorValue)
	}
	blueclaw := taskDetailServer(`llm failure evidence: /admin/api/diagnostics/llm-failure?id=` + identifier + `; provider failed`)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL, BlueclawWorkspacePath: workspacePath})

	responseRecorder := httptest.NewRecorder()
	service.proxyScopedTaskDetail(responseRecorder, httptest.NewRequest(http.MethodGet, "/runs/api/detail?taskRunID=run-1", nil), "admin@example.com", true)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("task detail answered %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var detail map[string]any
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &detail); errorValue != nil {
		t.Fatal(errorValue)
	}
	events := detail["taskEvents"].([]any)
	if events[0].(map[string]any)["name"] != "agent.failure" {
		t.Fatalf("existing task event was discarded: %#v", events)
	}
	event := events[len(events)-1].(map[string]any)
	if event["name"] != llmFailureEvidenceEventName || event["body"] != string(expectedDocument) {
		t.Fatalf("unexpected evidence event: %#v", event)
	}
	if event["createdAt"] != expectedEvidence.CreatedAt {
		t.Fatalf("evidence event createdAt %v, want %q", event["createdAt"], expectedEvidence.CreatedAt)
	}
	if !strings.Contains(string(expectedDocument), "synthetic provider response") {
		t.Fatalf("saved evidence omitted the captured provider response: %s", expectedDocument)
	}
}

func TestTaskDetailReportsCorruptedLLMFailureEvidence(t *testing.T) {
	workspacePath := t.TempDir()
	_, capture := llmbackend.NewExchangeCapture(nil)
	identifier, errorValue := llmbackend.WriteFailureEvidence(workspacePath, map[string]string{"prompt": "private request"}, capture)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	evidencePath := llmbackend.FailureEvidenceDirectory(workspacePath) + "/" + identifier + ".json"
	if errorValue := os.WriteFile(evidencePath, []byte(`{"createdAt":`), 0600); errorValue != nil {
		t.Fatal(errorValue)
	}
	blueclaw := taskDetailServer(`llm failure evidence: /admin/api/diagnostics/llm-failure?id=` + identifier + `; provider failed`)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL, BlueclawWorkspacePath: workspacePath})

	responseRecorder := httptest.NewRecorder()
	service.proxyScopedTaskDetail(responseRecorder, httptest.NewRequest(http.MethodGet, "/runs/api/detail?taskRunID=run-1", nil), "admin@example.com", true)
	var detail map[string]any
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &detail); errorValue != nil {
		t.Fatal(errorValue)
	}
	events := detail["taskEvents"].([]any)
	event := events[len(events)-1].(map[string]any)
	var unavailableBody map[string]string
	if errorValue := json.Unmarshal([]byte(event["body"].(string)), &unavailableBody); errorValue != nil {
		t.Fatal(errorValue)
	}
	if unavailableBody["error"] != "unexpected end of JSON input" || unavailableBody["evidenceID"] == "" {
		t.Fatalf("corruption reason was not retained: %#v", unavailableBody)
	}
}

func TestTaskDetailIgnoresFailureReasonsWithoutEvidenceMarker(t *testing.T) {
	blueclaw := taskDetailServer("provider failed")
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL, BlueclawWorkspacePath: t.TempDir()})

	responseRecorder := httptest.NewRecorder()
	service.proxyScopedTaskDetail(responseRecorder, httptest.NewRequest(http.MethodGet, "/runs/api/detail?taskRunID=run-1", nil), "admin@example.com", true)
	if strings.Contains(responseRecorder.Body.String(), "llm.failure_evidence") {
		t.Fatalf("failure reason without marker gained evidence event: %s", responseRecorder.Body.String())
	}
}

func TestTaskDetailDoesNotInlineLLMFailureEvidenceForNonAdmin(t *testing.T) {
	workspacePath := t.TempDir()
	_, capture := llmbackend.NewExchangeCapture(nil)
	identifier, errorValue := llmbackend.WriteFailureEvidence(workspacePath, map[string]string{"prompt": "private request"}, capture)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	blueclaw := taskDetailServer(`llm failure evidence: /admin/api/diagnostics/llm-failure?id=` + identifier + `; provider failed`)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL, BlueclawWorkspacePath: workspacePath})

	responseRecorder := httptest.NewRecorder()
	service.proxyScopedTaskDetail(responseRecorder, httptest.NewRequest(http.MethodGet, "/runs/api/detail?taskRunID=run-1", nil), "member@example.com", false)
	if strings.Contains(responseRecorder.Body.String(), "private request") {
		t.Fatal("non-admin task detail exposed failure evidence")
	}
	if strings.Contains(responseRecorder.Body.String(), llmFailureEvidenceEventName) {
		t.Fatal("non-admin task detail gained an evidence event")
	}
}

func TestTaskDetailReportsUnavailableLLMFailureEvidence(t *testing.T) {
	blueclaw := taskDetailServer("llm failure evidence: /admin/api/diagnostics/llm-failure?id=0123456789abcdef0123456789abcdef; provider failed")
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL, BlueclawWorkspacePath: t.TempDir()})

	responseRecorder := httptest.NewRecorder()
	service.proxyScopedTaskDetail(responseRecorder, httptest.NewRequest(http.MethodGet, "/runs/api/detail?taskRunID=run-1", nil), "admin@example.com", true)
	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("task detail answered %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if !strings.Contains(responseRecorder.Body.String(), llmFailureEvidenceUnavailableEventName) {
		t.Fatal("missing evidence did not produce an unavailable event")
	}
}

func taskDetailServer(failureReason string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		_, _ = responseWriter.Write([]byte(`{"taskRun":{"taskRunID":"run-1","failureReason":"` + failureReason + `"},"taskEvents":[{"name":"agent.failure","body":"{}"}]}`))
	}))
}
