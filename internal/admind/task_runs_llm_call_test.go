package admind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func exchangeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/admin/api/run/llm-call" || request.URL.Query().Get("id") != "call-1" {
			http.NotFound(responseWriter, request)
			return
		}
		responseWriter.Header().Set("Content-Type", "application/json")
		responseWriter.Write([]byte(`{"request":{"prompt":"private prompt"}}`))
	}))
}

func TestAnAdminReadsWhatACallWasSent(t *testing.T) {
	blueclaw := exchangeServer(t)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL})

	responseRecorder := httptest.NewRecorder()
	service.proxyAdminOnlyRunsRead(responseRecorder, httptest.NewRequest(http.MethodGet, "/runs/api/llm-call?id=call-1&viewerEmail=spoofed", nil), true, "/admin/api/run/llm-call", "id")

	if responseRecorder.Code != http.StatusOK || !strings.Contains(responseRecorder.Body.String(), "private prompt") {
		t.Fatalf("expected the exchange, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func TestAMemberCannotReadWhatACallWasSent(t *testing.T) {
	blueclaw := exchangeServer(t)
	defer blueclaw.Close()
	service := NewService(Configuration{BlueclawBaseURL: blueclaw.URL})

	responseRecorder := httptest.NewRecorder()
	service.proxyAdminOnlyRunsRead(responseRecorder, httptest.NewRequest(http.MethodGet, "/runs/api/llm-call?id=call-1", nil), false, "/admin/api/run/llm-call", "id")

	if responseRecorder.Code != http.StatusForbidden || strings.Contains(responseRecorder.Body.String(), "private prompt") {
		t.Fatalf("expected a member refused, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}
