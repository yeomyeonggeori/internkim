package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestAttendanceAddHandlerReturnsValidEnvelopeForNonJSONRecordFailures(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{name: "plain text", statusCode: http.StatusInternalServerError, body: "record unavailable"},
		{name: "html", statusCode: http.StatusBadGateway, body: "<html>record unavailable</html>"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
				responseWriter.Header().Set("Content-Type", "text/plain")
				responseWriter.WriteHeader(testCase.statusCode)
				_, _ = responseWriter.Write([]byte(testCase.body))
			}))
			service := Service{Configuration: Configuration{AdmindSocketPath: socketPath}}
			responseRecorder := invokeAttendanceAdd(t, service)

			if responseRecorder.Code != http.StatusOK {
				t.Fatalf("handler status = %d, want %d", responseRecorder.Code, http.StatusOK)
			}
			var response capabilities.ToolInvokeResponse
			if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
				t.Fatalf("handler returned invalid JSON: %v; body=%q", errorValue, responseRecorder.Body.String())
			}
			if response.Provider != "internkim" || response.SelectedBackend != "record" {
				t.Fatalf("response identity = %q/%q", response.Provider, response.SelectedBackend)
			}
			if response.Outcome != capabilities.ToolOutcomeFailed || response.Status != "error" || !response.IsError {
				t.Fatalf("response failure state = %+v", response)
			}
			if response.Message != testCase.body || response.Content != testCase.body {
				t.Fatalf("record message = %q/%q, want %q", response.Message, response.Content, testCase.body)
			}
			var originalBody string
			if errorValue := json.Unmarshal(response.Result, &originalBody); errorValue != nil || originalBody != testCase.body {
				t.Fatalf("record body was not preserved: %s", response.Result)
			}
			if response.ErrorCode != "record_refused" || response.FailureStage != "execution" {
				t.Fatalf("response error location = %q/%q", response.ErrorCode, response.FailureStage)
			}
		})
	}
}

func TestAttendanceAddHandlerPreservesStructuredRecordRefusal(t *testing.T) {
	refusal := `{"error":"attendance date needs clarification","errorCode":"interaction_required","failureStage":"target_resolution","retryable":true,"candidates":["이샘플 · clock_in · 2026-09-01","이샘플 · clock_out · 2026-09-01"]}`
	socketPath := admindOnASocket(t, http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.WriteHeader(http.StatusConflict)
		_, _ = responseWriter.Write([]byte(refusal))
	}))
	service := Service{Configuration: Configuration{AdmindSocketPath: socketPath}}
	responseRecorder := invokeAttendanceAdd(t, service)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("handler status = %d, want %d", responseRecorder.Code, http.StatusOK)
	}
	var response capabilities.ToolInvokeResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("handler returned invalid JSON: %v; body=%q", errorValue, responseRecorder.Body.String())
	}
	if response.Provider != "internkim" || response.SelectedBackend != "record" || response.Outcome != capabilities.ToolOutcomeFailed {
		t.Fatalf("response identity or outcome = %+v", response)
	}
	if response.Message != "attendance date needs clarification" || response.Status != "error" || response.ErrorCode != "interaction_required" || response.FailureStage != "target_resolution" {
		t.Fatalf("structured refusal fields = %+v", response)
	}
	if !response.Retryable || !strings.Contains(string(response.Result), "2026-09-01") || !strings.Contains(string(response.Result), "clock_out") {
		t.Fatalf("structured refusal result = %s", response.Result)
	}
}

func TestWriteJSONReturnsSerializationFailureForInvalidRawMessage(t *testing.T) {
	responseRecorder := httptest.NewRecorder()
	Service{}.writeJSON(responseRecorder, json.RawMessage("not-json"))

	if responseRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("writeJSON status = %d, want %d", responseRecorder.Code, http.StatusInternalServerError)
	}
	if responseRecorder.Body.Len() == 0 {
		t.Fatal("writeJSON returned an empty body")
	}
	var response map[string]string
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("writeJSON returned invalid JSON: %v; body=%q", errorValue, responseRecorder.Body.String())
	}
	if !strings.Contains(strings.ToLower(response["error"]), "serialization") {
		t.Fatalf("writeJSON error = %q, want serialization failure", response["error"])
	}
}

func invokeAttendanceAdd(t *testing.T, service Service) *httptest.ResponseRecorder {
	t.Helper()
	requestBody := `{"input":{"kind":"clock_in","date":"2026-09-01","time":"09:02","reason":"기록을 잊었습니다"},"context":{"requesterPersonID":"person-sample","requesterEmail":"member@example.com","approvedCallID":"held-4f2a91c0"}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/tools/attendance_add/invoke", bytes.NewBufferString(requestBody))
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request.WithContext(context.Background()))
	return responseRecorder
}
