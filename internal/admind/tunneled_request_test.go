package admind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const tunneledAdminEmail = "admin@example.com"

func tunneledRequestTestService(t *testing.T) *Service {
	t.Helper()
	configuration := taskRetryTestConfiguration(t, tunneledAdminEmail)
	configuration.ClaimedAdminEmailPath = writeTestFile(t, tunneledAdminEmail)
	return NewService(configuration)
}

func loopbackRequest(method string, path string, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:52144"
	return request
}

func tunneledRequest(method string, path string, body string) *http.Request {
	request := loopbackRequest(method, path, body)
	request.Header.Set("Cf-Connecting-Ip", "203.0.113.7")
	request.Header.Set("Cf-Ray", "8c2f1e0a1b2c3d4e-ICN")
	request.Header.Set("Cdn-Loop", "cloudflare")
	request.Header.Set("X-Forwarded-For", "203.0.113.7")
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("Cf-Visitor", `{"scheme":"https"}`)
	return request
}

func answeredStatus(service *Service, request *http.Request) int {
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	return response.Code
}

func TestATunneledRequestIsNeverLocal(t *testing.T) {
	if !isLocalRequest(loopbackRequest(http.MethodGet, "/", "")) {
		t.Fatal("a loopback caller with no proxy marker must stay local")
	}
	if isLocalRequest(tunneledRequest(http.MethodGet, "/", "")) {
		t.Fatal("a loopback connection carrying the Cloudflare edge's headers came from the internet")
	}
	for _, header := range proxyMarkerHeaders {
		request := loopbackRequest(http.MethodGet, "/", "")
		request.Header.Set(header, "")
		if isLocalRequest(request) {
			t.Errorf("a loopback request carrying %s is forwarded, so it must not be local", header)
		}
	}
}

func TestTunneledAgentAPIRequestsAreRefused(t *testing.T) {
	service := tunneledRequestTestService(t)
	for _, path := range []string{"/agent/api/buzz-admin-wipe", "/agent/api/buzz-admin-reset"} {
		request := tunneledRequest(http.MethodPost, path, `{}`)
		request.Header.Set("X-Forwarded-Email", tunneledAdminEmail)
		if status := answeredStatus(service, request); status != http.StatusForbidden {
			t.Errorf("a tunneled %s naming the admin in a header answered %d, want 403", path, status)
		}
	}
	for _, path := range []string{
		"/agent/api/company-profile-carry",
		"/agent/api/crm-record-coverage",
		"/agent/api/company-ledger-coverage",
		"/agent/api/mail-account-carry",
		"/agent/api/organization-record-coverage",
		"/agent/api/buzz-channel-visibility-repair",
	} {
		if status := answeredStatus(service, tunneledRequest(http.MethodPost, path, "")); status != http.StatusForbidden {
			t.Errorf("a tunneled %s answered %d, want 403", path, status)
		}
	}
	request := tunneledRequest(http.MethodGet, "/agent/api/channels", "")
	request.Header.Set("X-Forwarded-Email", tunneledAdminEmail)
	if status := answeredStatus(service, request); status != http.StatusUnauthorized {
		t.Errorf("a tunneled request naming someone in a header was taken as them, answered %d, want 401", status)
	}
}

func TestLocalAgentAPICallersStillPass(t *testing.T) {
	service := tunneledRequestTestService(t)
	for _, path := range []string{"/agent/api/buzz-admin-wipe", "/agent/api/buzz-admin-reset"} {
		request := loopbackRequest(http.MethodPost, path, `{}`)
		request.Header.Set("X-Forwarded-Email", tunneledAdminEmail)
		if status := answeredStatus(service, request); status != http.StatusBadRequest {
			t.Errorf("a local admin's %s with no email answered %d, want 400 from past the gate", path, status)
		}
	}
}

func TestTunneledDirectoryAndAdminRequestsAreRefused(t *testing.T) {
	service := tunneledRequestTestService(t)
	for _, call := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/admin/api/buzz/signing-secret"},
		{http.MethodPost, "/admin/api/directory/direct-message"},
		{http.MethodPost, "/admin/api/directory/buzz-key"},
		{http.MethodPost, "/admin/api/directory/person"},
		{http.MethodGet, "/admin/api/directory/people"},
		{http.MethodPost, "/admin/api/directory/changed"},
		{http.MethodGet, "/admin/api/diagnostics/requests"},
		{http.MethodGet, "/bridge/api/channel"},
	} {
		if status := answeredStatus(service, tunneledRequest(call.method, call.path, `{}`)); status != http.StatusForbidden {
			t.Errorf("a tunneled %s %s answered %d, want 403", call.method, call.path, status)
		}
	}
}

func TestLocalDirectoryAndAdminCallersStillPass(t *testing.T) {
	service := tunneledRequestTestService(t)
	for _, call := range []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodPost, "/admin/api/buzz/signing-secret", http.StatusBadRequest},
		{http.MethodPost, "/admin/api/directory/direct-message", http.StatusBadRequest},
		{http.MethodPost, "/admin/api/directory/buzz-key", http.StatusBadRequest},
		{http.MethodGet, "/admin/api/diagnostics/requests", http.StatusOK},
	} {
		if status := answeredStatus(service, loopbackRequest(call.method, call.path, `{}`)); status != call.want {
			t.Errorf("a local %s %s answered %d, want %d", call.method, call.path, status, call.want)
		}
	}
}

func TestLocalCallersOnlyAnswersOnlyLocalCallers(t *testing.T) {
	reached := false
	handler := localCallersOnly(func(http.ResponseWriter, *http.Request) { reached = true })
	handler(httptest.NewRecorder(), tunneledRequest(http.MethodGet, "/", ""))
	if reached {
		t.Fatal("a tunneled request reached a local-only handler")
	}
	handler(httptest.NewRecorder(), loopbackRequest(http.MethodGet, "/", ""))
	if !reached {
		t.Fatal("a local caller was turned away from a local-only handler")
	}
}
