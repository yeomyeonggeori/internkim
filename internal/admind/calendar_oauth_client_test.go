package admind

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestLoadGoogleOAuthClientSecretParsesInstalled(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"abc.apps.googleusercontent.com","client_secret":"secret-1"}}`)
	clientID, clientSecret, errorValue := service.loadGoogleOAuthClientSecret()
	if errorValue != nil {
		t.Fatalf("load: %v", errorValue)
	}
	if clientID != "abc.apps.googleusercontent.com" {
		t.Errorf("clientID: got %q", clientID)
	}
	if clientSecret != "secret-1" {
		t.Errorf("clientSecret: got %q", clientSecret)
	}
}

func TestLoadGoogleOAuthClientSecretParsesWeb(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"web":{"client_id":"web.apps","client_secret":"web-secret"}}`)
	clientID, clientSecret, errorValue := service.loadGoogleOAuthClientSecret()
	if errorValue != nil {
		t.Fatalf("load: %v", errorValue)
	}
	if clientID != "web.apps" || clientSecret != "web-secret" {
		t.Errorf("got %q / %q", clientID, clientSecret)
	}
}

func TestLoadGoogleOAuthClientSecretRejectsMissingClientID(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_secret":"only-secret"}}`)
	if _, _, errorValue := service.loadGoogleOAuthClientSecret(); errorValue == nil || !strings.Contains(errorValue.Error(), "missing client_id") {
		t.Fatalf("expected missing client_id error, got %v", errorValue)
	}
}

func TestLoadGoogleOAuthClientSecretRejectsMissingClientSecret(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1"}}`)
	if _, _, errorValue := service.loadGoogleOAuthClientSecret(); errorValue == nil || !strings.Contains(errorValue.Error(), "missing client_secret") {
		t.Fatalf("expected missing client_secret error, got %v", errorValue)
	}
}

func TestOAuth2TokenPayloadRoundTrip(t *testing.T) {
	expiry := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	token := &oauth2.Token{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       expiry,
	}
	payload := oauthTokenPayloadFromOAuth2Token(token)
	if payload.AccessToken != token.AccessToken || payload.RefreshToken != token.RefreshToken || payload.TokenType != token.TokenType {
		t.Fatalf("payload mismatch: %+v", payload)
	}
	rebuilt := oauth2TokenFromPayload(payload)
	if rebuilt.AccessToken != token.AccessToken || rebuilt.RefreshToken != token.RefreshToken {
		t.Fatalf("rebuilt mismatch: %+v", rebuilt)
	}
	if !rebuilt.Expiry.Equal(expiry) {
		t.Errorf("expiry: got %v want %v", rebuilt.Expiry, expiry)
	}
}

func TestGoogleOAuthStartRedirectsWithStateAndScope(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1","client_secret":"secret-1"}}`)
	request := httptest.NewRequest(http.MethodGet, "http://admind.local"+googleOAuthStartPath, nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	recorder := httptest.NewRecorder()
	service.handleGoogleOAuthStart(recorder, request)
	if recorder.Code != http.StatusFound {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusFound)
	}
	location, errorValue := url.Parse(recorder.Header().Get("Location"))
	if errorValue != nil {
		t.Fatalf("parse location: %v", errorValue)
	}
	if !strings.HasPrefix(location.String(), googleAuthURL) {
		t.Errorf("authorize host wrong: %q", location.String())
	}
	if location.Query().Get("client_id") != "client-1" {
		t.Errorf("client_id mismatch: %q", location.Query().Get("client_id"))
	}
	scope := location.Query().Get("scope")
	for _, required := range []string{googleCalendarScope, googleOpenIDScope, googleUserinfoEmailScope} {
		if !strings.Contains(scope, required) {
			t.Errorf("scope missing %q: got %q", required, scope)
		}
	}
	if location.Query().Get("access_type") != "offline" {
		t.Errorf("access_type: got %q", location.Query().Get("access_type"))
	}
	if location.Query().Get("prompt") != "consent" {
		t.Errorf("prompt: got %q", location.Query().Get("prompt"))
	}
	if location.Query().Get("redirect_uri") != "http://admind.local"+googleOAuthCallbackPath {
		t.Errorf("redirect_uri: got %q", location.Query().Get("redirect_uri"))
	}
	state := location.Query().Get("state")
	if state == "" {
		t.Fatal("state missing")
	}
	if _, found := service.googleOAuthStates.Load(state); !found {
		t.Errorf("state %q not stored", state)
	}
}

func TestGoogleOAuthStartRejectsUnauthorizedRequest(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodGet, "http://admind.local"+googleOAuthStartPath, nil)
	recorder := httptest.NewRecorder()
	service.handleGoogleOAuthStart(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestCalendarConnectionStartIsNotAvailableToStaff(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1","client_secret":"secret-1"}}`)
	request := httptest.NewRequest(http.MethodPost, "http://admind.local/calendar/api/connection/start", nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "staff@example.com")
	recorder := httptest.NewRecorder()

	service.handleCalendar(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status: got %d body: %s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadGoogleOAuthClientFileStoresValidClientJSON(t *testing.T) {
	service := newCalendarTestService(t)
	body, contentType := buildGoogleOAuthClientUploadBody(t, `{"web":{"client_id":"client-1","client_secret":"secret-1"}}`)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-oauth-client", body)
	request.RemoteAddr = "127.0.0.1:34567"
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	service.handleCalendar(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status: got %d body: %s", recorder.Code, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "secret-1") {
		t.Fatalf("response must not expose client secret: %s", recorder.Body.String())
	}
	clientID, clientSecret, errorValue := service.loadGoogleOAuthClientSecret()
	if errorValue != nil {
		t.Fatalf("load stored client: %v", errorValue)
	}
	if clientID != "client-1" || clientSecret != "secret-1" {
		t.Fatalf("stored credentials = %q / %q", clientID, clientSecret)
	}
	fileInformation, errorValue := os.Stat(service.googleOAuthClientFilePath())
	if errorValue != nil {
		t.Fatalf("stat stored client: %v", errorValue)
	}
	if fileInformation.Mode().Perm() != 0o600 {
		t.Fatalf("client file mode = %o", fileInformation.Mode().Perm())
	}
	directoryInformation, errorValue := os.Stat(service.calendarSecretsDirectory())
	if errorValue != nil {
		t.Fatalf("stat secrets directory: %v", errorValue)
	}
	if directoryInformation.Mode().Perm() != 0o700 {
		t.Fatalf("secrets directory mode = %o", directoryInformation.Mode().Perm())
	}
}

func TestUploadGoogleOAuthClientFileRequiresAdmin(t *testing.T) {
	service := newCalendarTestService(t)
	body, contentType := buildGoogleOAuthClientUploadBody(t, `{"web":{"client_id":"client-1","client_secret":"secret-1"}}`)
	request := httptest.NewRequest(http.MethodPost, "http://admind.local/calendar/api/google-oauth-client", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	service.handleCalendar(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status: got %d body: %s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadGoogleOAuthClientFileRejectsInvalidClientJSON(t *testing.T) {
	testCases := map[string]string{
		"invalid json":          `{`,
		"missing client id":     `{"web":{"client_secret":"secret-1"}}`,
		"missing client secret": `{"web":{"client_id":"client-1"}}`,
	}
	for name, document := range testCases {
		t.Run(name, func(t *testing.T) {
			service := newCalendarTestService(t)
			body, contentType := buildGoogleOAuthClientUploadBody(t, document)
			request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-oauth-client", body)
			request.RemoteAddr = "127.0.0.1:34567"
			request.Header.Set("Content-Type", contentType)
			recorder := httptest.NewRecorder()

			service.handleCalendar(recorder, request)

			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status: got %d body: %s", recorder.Code, recorder.Body.String())
			}
			if _, errorValue := os.Stat(service.googleOAuthClientFilePath()); !os.IsNotExist(errorValue) {
				t.Fatalf("client file should not be written, stat error = %v", errorValue)
			}
		})
	}
}

func TestUploadGoogleOAuthClientFileRejectsTooLargeDocument(t *testing.T) {
	service := newCalendarTestService(t)
	body, contentType := buildGoogleOAuthClientUploadBody(t, strings.Repeat("a", googleOAuthClientUploadMaxBytes+1))
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-oauth-client", body)
	request.RemoteAddr = "127.0.0.1:34567"
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	service.handleCalendar(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d body: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "client.json file is too large") {
		t.Fatalf("body should explain size limit: %s", recorder.Body.String())
	}
	if _, errorValue := os.Stat(service.googleOAuthClientFilePath()); !os.IsNotExist(errorValue) {
		t.Fatalf("client file should not be written, stat error = %v", errorValue)
	}
}

func buildGoogleOAuthClientUploadBody(t *testing.T, content string) (*bytes.Buffer, string) {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, errorValue := writer.CreateFormFile("client", "client.json")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := part.Write([]byte(content)); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := writer.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return body, writer.FormDataContentType()
}
