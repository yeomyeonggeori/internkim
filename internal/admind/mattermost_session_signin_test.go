package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func aServiceThatTrustsAMattermostSession(t *testing.T, offered bool, userRecord mattermostUserRecord) *Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v4/users/me" || request.Header.Get("Cookie") == "" {
			responseWriter.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(responseWriter).Encode(userRecord)
	}))
	t.Cleanup(server.Close)

	directory := t.TempDir()
	offeredPath := filepath.Join(directory, "mattermost-session-signin")
	if offered {
		if errorValue := os.WriteFile(offeredPath, []byte("1\n"), 0o600); errorValue != nil {
			t.Fatalf("write the switch: %v", errorValue)
		}
	}
	adminPath := filepath.Join(directory, "admin-email")
	if errorValue := os.WriteFile(adminPath, []byte(userRecord.Email+"\n"), 0o600); errorValue != nil {
		t.Fatalf("write the admin email: %v", errorValue)
	}
	return &Service{Configuration: Configuration{
		MattermostBaseURL:           server.URL,
		MattermostSessionSignInPath: offeredPath,
		AdminEmailPath:              adminPath,
	}}
}

func aRequestCarryingAMattermostCookie() *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/admin/api/session", nil)
	request.AddCookie(&http.Cookie{Name: "MMAUTHTOKEN", Value: "a-session-the-browser-already-holds"})
	return request
}

func TestAMattermostSessionSignsSomeoneInWhenOffered(t *testing.T) {
	service := aServiceThatTrustsAMattermostSession(t, true, mattermostUserRecord{
		ID:    "user-1",
		Email: "sample@example.test",
	})
	if email := service.emailOfMattermostSession(aRequestCarryingAMattermostCookie()); email != "sample@example.test" {
		t.Fatalf("expected the Mattermost session to sign them in, got %q", email)
	}
}

func TestAMattermostSessionIsIgnoredUnlessTheDeploymentOffersIt(t *testing.T) {
	service := aServiceThatTrustsAMattermostSession(t, false, mattermostUserRecord{
		ID:    "user-1",
		Email: "sample@example.test",
	})
	if email := service.emailOfMattermostSession(aRequestCarryingAMattermostCookie()); email != "" {
		t.Fatalf("a deployment that never asked for this signed someone in as %q", email)
	}
}

func TestABotSessionSignsNobodyIn(t *testing.T) {
	service := aServiceThatTrustsAMattermostSession(t, true, mattermostUserRecord{
		ID:    "bot-1",
		Email: "sample@example.test",
		IsBot: true,
	})
	if email := service.emailOfMattermostSession(aRequestCarryingAMattermostCookie()); email != "" {
		t.Fatalf("a bot signed in as %q", email)
	}
}

func TestADeactivatedSessionSignsNobodyIn(t *testing.T) {
	service := aServiceThatTrustsAMattermostSession(t, true, mattermostUserRecord{
		ID:       "user-1",
		Email:    "sample@example.test",
		DeleteAt: 1,
	})
	if email := service.emailOfMattermostSession(aRequestCarryingAMattermostCookie()); email != "" {
		t.Fatalf("a deactivated account signed in as %q", email)
	}
}

func TestNoCookieSignsNobodyIn(t *testing.T) {
	service := aServiceThatTrustsAMattermostSession(t, true, mattermostUserRecord{
		ID:    "user-1",
		Email: "sample@example.test",
	})
	request := httptest.NewRequest(http.MethodGet, "/admin/api/session", nil)
	if email := service.emailOfMattermostSession(request); email != "" {
		t.Fatalf("a request with no cookie signed in as %q", email)
	}
}

func TestSomeoneWhoSignedOutStaysSignedOut(t *testing.T) {
	service := aServiceThatTrustsAMattermostSession(t, true, mattermostUserRecord{
		ID:    "user-1",
		Email: "sample@example.test",
	})
	request := aRequestCarryingAMattermostCookie()
	request.AddCookie(webLogoutMarkerCookie(false))
	if email := service.webActorEmail(request); email != "" {
		t.Fatalf("the Mattermost cookie signed a logged-out person back in as %q", email)
	}
}
