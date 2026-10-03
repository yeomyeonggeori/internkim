package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskRunRetryRequiresAnAuthenticatedTaskViewer(t *testing.T) {
	service := NewService(taskRetryTestConfiguration(t, "admin@example.com"))
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/runs/api/retry", strings.NewReader(`{"taskRunID":"run-1"}`)))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated retry answered %d, want 401", response.Code)
	}
}

func TestTaskRunRetryRejectsAnUntrustedRequester(t *testing.T) {
	configuration := taskRetryTestConfiguration(t, "admin@example.com")
	configuration.TrustProxyForwardedEmail = true
	service := NewService(configuration)
	request := httptest.NewRequest(http.MethodPost, "/runs/api/retry", strings.NewReader(`{"taskRunID":"run-1"}`))
	request.Header.Set("Cf-Access-Authenticated-User-Email", "member@example.com")
	response := httptest.NewRecorder()
	markRequestsAsAssertedByTheListener(service.router()).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("unauthorized retry answered %d, want 403", response.Code)
	}
}

func TestTaskRunRetryForwardsTrustedViewerAndPreservesBlueclawStatus(t *testing.T) {
	for _, status := range []int{http.StatusAccepted, http.StatusConflict, http.StatusNotFound, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var received struct {
				TaskRunID     string `json:"taskRunID"`
				ViewerEmail   string `json:"viewerEmail"`
				ViewerIsAdmin bool   `json:"viewerIsAdmin"`
			}
			blueclaw := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
				if request.Method != http.MethodPost || request.URL.Path != "/admin/api/run/retry" {
					t.Fatalf("Blueclaw request = %s %s", request.Method, request.URL.Path)
				}
				if json.NewDecoder(request.Body).Decode(&received) != nil {
					t.Fatal("Blueclaw received malformed JSON")
				}
				responseWriter.Header().Set("Content-Type", "application/json")
				responseWriter.WriteHeader(status)
				_, _ = responseWriter.Write([]byte(`{"status":"retry"}`))
			}))
			defer blueclaw.Close()
			configuration := taskRetryTestConfiguration(t, "admin@example.com")
			configuration.BlueclawBaseURL = blueclaw.URL
			service := NewService(configuration)
			seatAdministratorInTheCompanyForTest(t, service, "admin@example.com")
			request := httptest.NewRequest(http.MethodPost, "/runs/api/retry", strings.NewReader(`{"taskRunID":"run-1","viewerEmail":"forged@example.com","viewerIsAdmin":false}`))
			request.Header.Set(requesterEmailHeader, "admin@example.com")
			response := httptest.NewRecorder()
			markRequestsAsAssertedByTheListener(service.router()).ServeHTTP(response, request)
			if response.Code != status {
				t.Fatalf("retry answered %d, want %d", response.Code, status)
			}
			if received.TaskRunID != "run-1" || received.ViewerEmail != "admin@example.com" || !received.ViewerIsAdmin {
				t.Fatalf("Blueclaw received untrusted retry context: %#v", received)
			}
		})
	}
}

func TestTaskRunRetryRejectsMalformedAndMissingTaskRunID(t *testing.T) {
	service := NewService(taskRetryTestConfiguration(t, "admin@example.com"))
	seatAdministratorInTheCompanyForTest(t, service, "admin@example.com")
	for _, body := range []string{"not-json", `{}`} {
		request := httptest.NewRequest(http.MethodPost, "/runs/api/retry", strings.NewReader(body))
		request.Header.Set(requesterEmailHeader, "admin@example.com")
		response := httptest.NewRecorder()
		markRequestsAsAssertedByTheListener(service.router()).ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("retry body %q answered %d, want 400", body, response.Code)
		}
	}
}

func taskRetryTestConfiguration(t *testing.T, adminEmail string) Configuration {
	t.Helper()
	return Configuration{
		AdminEmailPath: writeTestFile(t, adminEmail),
	}
}
