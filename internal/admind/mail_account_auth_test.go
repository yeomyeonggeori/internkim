package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMailAccountRequiresAuthenticatedActor(t *testing.T) {
	service := newMailTestService(t)
	saveConfiguredMailTestAccount(t, service)

	request := httptest.NewRequest(http.MethodGet, "/mail/api/account", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	service.handleMail(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestMailAccountUsesRequesterHeaderForLocalRequests(t *testing.T) {
	service := newMailTestService(t)

	request := httptest.NewRequest(http.MethodGet, "/mail/api/account", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set(flowRequesterEmailHeader, "staff@example.com")
	response := httptest.NewRecorder()
	service.handleMail(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var account mailAccountResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &account); errorValue != nil {
		t.Fatal(errorValue)
	}
	if account.Email != "staff@example.com" {
		t.Fatalf("account = %#v", account)
	}
}

func TestMailAccountIgnoresRequesterHeaderForRemoteRequests(t *testing.T) {
	service := newMailTestService(t)

	request := httptest.NewRequest(http.MethodGet, "/mail/api/account", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	request.Header.Set(flowRequesterEmailHeader, "staff@example.com")
	response := httptest.NewRecorder()
	service.handleMail(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
}

func TestMailAccountUsesWebSessionForRemoteRequests(t *testing.T) {
	service := newMailTestService(t)
	writeUsersSyncTestCache(t, service, "staff@example.com")

	request := httptest.NewRequest(http.MethodGet, "/mail/api/account", nil)
	request.RemoteAddr = "203.0.113.10:12345"
	request.AddCookie(&http.Cookie{
		Name:  webSessionCookieName,
		Value: webSessionCookieForTest(t, service, "staff@example.com"),
	})
	response := httptest.NewRecorder()
	service.handleMail(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	var account mailAccountResponse
	if errorValue := json.Unmarshal(response.Body.Bytes(), &account); errorValue != nil {
		t.Fatal(errorValue)
	}
	if account.Email != "staff@example.com" {
		t.Fatalf("account = %#v", account)
	}
}

func TestMailAccountDoesNotFallbackToAdminForAuthenticatedUser(t *testing.T) {
	service := newMailTestService(t)
	saveConfiguredMailTestAccount(t, service)

	accountResponse := performMailRequestAs(t, service, "staff@example.com", http.MethodGet, "/mail/api/account", "")
	if accountResponse.Code != http.StatusOK {
		t.Fatalf("account status = %d body = %s", accountResponse.Code, accountResponse.Body.String())
	}
	var account mailAccountResponse
	if errorValue := json.Unmarshal(accountResponse.Body.Bytes(), &account); errorValue != nil {
		t.Fatal(errorValue)
	}
	if account.IsConfigured || account.Email != "staff@example.com" {
		t.Fatalf("account should be an unconfigured staff account: %#v", account)
	}

	mailboxesResponse := performMailRequestAs(t, service, "staff@example.com", http.MethodGet, "/mail/api/mailboxes", "")
	if mailboxesResponse.Code != http.StatusBadRequest {
		t.Fatalf("mailboxes status = %d body = %s", mailboxesResponse.Code, mailboxesResponse.Body.String())
	}
}

func TestMailRequiresConfiguredAccountForMailboxReads(t *testing.T) {
	service := newMailTestService(t)
	response := performMailRequest(t, service, http.MethodGet, "/mail/api/mailboxes", "")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", response.Code, response.Body.String())
	}
	accountResponse := performMailRequest(t, service, http.MethodGet, "/mail/api/account", "")
	if accountResponse.Code != http.StatusOK {
		t.Fatalf("account status = %d", accountResponse.Code)
	}
	var account mailAccountResponse
	if errorValue := json.Unmarshal(accountResponse.Body.Bytes(), &account); errorValue != nil {
		t.Fatal(errorValue)
	}
	if account.IsConfigured {
		t.Fatalf("new account should not be configured: %#v", account)
	}
}
