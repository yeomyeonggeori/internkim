package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

func TestGoogleOAuthCallbackErrorUsesAcceptLanguage(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodGet, "http://admind.local"+googleOAuthCallbackPath+"?error=access_denied", nil)
	request.Header.Set("Accept-Language", "en-US,en;q=0.9")
	recorder := httptest.NewRecorder()
	service.handleGoogleOAuthCallback(recorder, request)
	body := recorder.Body.String()
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, body: %s", recorder.Code, body)
	}
	if !strings.Contains(body, `<html lang="en">`) || !strings.Contains(body, "Google Calendar connection failed") {
		t.Fatalf("expected English error page, got: %s", body)
	}
	if !strings.Contains(body, "Google returned an error: access_denied") {
		t.Fatalf("expected localized Google error message, got: %s", body)
	}
}

func TestGoogleOAuthCallbackPersistsTokenAndAccount(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1","client_secret":"secret-1"}}`)
	const issuedAccessToken = "access-from-google"
	const issuedRefreshToken = "refresh-from-google"
	const issuedEmail = "user@example.com"

	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if errorValue := request.ParseForm(); errorValue != nil {
			t.Fatalf("token form parse: %v", errorValue)
		}
		if grant := request.PostForm.Get("grant_type"); grant != "authorization_code" {
			t.Errorf("grant_type: got %q", grant)
		}
		writer.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"access_token":  issuedAccessToken,
			"refresh_token": issuedRefreshToken,
			"token_type":    "Bearer",
			"expires_in":    3600,
		}
		_ = json.NewEncoder(writer).Encode(response)
	}))
	defer tokenServer.Close()

	userinfoServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if header := request.Header.Get("Authorization"); header != "Bearer "+issuedAccessToken {
			t.Errorf("authorization header: got %q", header)
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{"email": issuedEmail})
	}))
	defer userinfoServer.Close()

	t.Setenv(googleTokenURLOverrideEnv, tokenServer.URL)
	t.Setenv(googleUserinfoURLOverrideEnv, userinfoServer.URL)

	startRequest := httptest.NewRequest(http.MethodGet, "http://admind.local"+googleOAuthStartPath, nil)
	startRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	startRecorder := httptest.NewRecorder()
	service.handleGoogleOAuthStart(startRecorder, startRequest)
	if startRecorder.Code != http.StatusFound {
		t.Fatalf("start status: %d", startRecorder.Code)
	}
	startLocation, errorValue := url.Parse(startRecorder.Header().Get("Location"))
	if errorValue != nil {
		t.Fatalf("parse start location: %v", errorValue)
	}
	state := startLocation.Query().Get("state")
	if state == "" {
		t.Fatal("state missing from start")
	}

	callbackURL := "http://admind.local" + googleOAuthCallbackPath + "?state=" + url.QueryEscape(state) + "&code=auth-code-1"
	callbackRequest := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	callbackRequest.Header.Set("Accept-Language", "en-US,en;q=0.9")
	callbackRecorder := httptest.NewRecorder()
	service.handleGoogleOAuthCallback(callbackRecorder, callbackRequest)
	if callbackRecorder.Code != http.StatusOK {
		t.Fatalf("callback status: %d, body: %s", callbackRecorder.Code, callbackRecorder.Body.String())
	}
	if !strings.Contains(callbackRecorder.Body.String(), issuedEmail) ||
		!strings.Contains(callbackRecorder.Body.String(), `<html lang="en">`) ||
		!strings.Contains(callbackRecorder.Body.String(), "Google Calendar connected") {
		t.Errorf("success page missing email: %s", callbackRecorder.Body.String())
	}
	if _, stillStored := service.googleOAuthStates.Load(state); stillStored {
		t.Error("state should be consumed after callback")
	}

	ctx := context.Background()
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}
	if !found {
		t.Fatal("account not persisted")
	}
	if account.AccountEmail != issuedEmail {
		t.Errorf("account email: got %q", account.AccountEmail)
	}
	if account.TokenFilePath == "" {
		t.Fatal("token path not set")
	}

	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		t.Fatalf("load key: %v", errorValue)
	}
	payload, errorValue := readCalendarTokenFile(account.TokenFilePath, key)
	if errorValue != nil {
		t.Fatalf("read token file: %v", errorValue)
	}
	if payload.AccessToken != issuedAccessToken {
		t.Errorf("access token: got %q", payload.AccessToken)
	}
	if payload.RefreshToken != issuedRefreshToken {
		t.Errorf("refresh token: got %q", payload.RefreshToken)
	}
}

func TestSaveGoogleOAuthTokenAndAccountResetsDiscoveryForDifferentEmail(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	existing := remoteCalendarAccount{
		ID:                  googleOAuthAccountPrefix + "old_example_com",
		Provider:            remoteCalendarProviderGoogle,
		AccountEmail:        "old@example.com",
		PrincipalURL:        "https://apidata.googleusercontent.com/caldav/v2/old@example.com/user",
		HomeSetURL:          "https://apidata.googleusercontent.com/caldav/v2/old@example.com/",
		DefaultCalendarURL:  "https://apidata.googleusercontent.com/caldav/v2/old@example.com/events/",
		DefaultCalendarCTag: "old-ctag",
		TokenFilePath:       "/tmp/old-google-token.enc",
	}
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, existing); errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	token := &oauth2.Token{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}

	account, errorValue := service.saveGoogleOAuthTokenAndAccount(ctx, token, "new@example.com")
	if errorValue != nil {
		t.Fatalf("save account: %v", errorValue)
	}

	if account.AccountEmail != "new@example.com" {
		t.Fatalf("account email: got %q", account.AccountEmail)
	}
	if account.PrincipalURL != "" || account.HomeSetURL != "" || account.DefaultCalendarURL != "" || account.DefaultCalendarCTag != "" {
		t.Fatalf("discovery fields should reset for a different email: %+v", account)
	}
}

func TestGoogleOAuthCallbackRejectsUnknownState(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1","client_secret":"secret-1"}}`)
	request := httptest.NewRequest(http.MethodGet,
		"http://admind.local"+googleOAuthCallbackPath+"?state=never-stored&code=x", nil)
	recorder := httptest.NewRecorder()
	service.handleGoogleOAuthCallback(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d", recorder.Code)
	}
}

func TestGoogleOAuthCallbackRejectsExpiredState(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1","client_secret":"secret-1"}}`)
	staleState := "stale-state"
	service.googleOAuthStates.Store(staleState, &googleOAuthStateRecord{
		CreatedAt:   time.Now().UTC().Add(-2 * googleOAuthStateTTL),
		RedirectURI: "http://admind.local" + googleOAuthCallbackPath,
	})
	request := httptest.NewRequest(http.MethodGet,
		"http://admind.local"+googleOAuthCallbackPath+"?state="+staleState+"&code=x", nil)
	recorder := httptest.NewRecorder()
	service.handleGoogleOAuthCallback(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d", recorder.Code)
	}
}

func TestGoogleOAuthCallbackReportsGoogleError(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodGet,
		"http://admind.local"+googleOAuthCallbackPath+"?error=access_denied", nil)
	recorder := httptest.NewRecorder()
	service.handleGoogleOAuthCallback(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status: got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "access_denied") {
		t.Errorf("body missing google error: %s", recorder.Body.String())
	}
}

func TestCleanupExpiredGoogleOAuthStatesRemovesOnlyOldRecords(t *testing.T) {
	service := newCalendarTestService(t)
	now := time.Now().UTC()
	service.googleOAuthStates.Store("fresh", &googleOAuthStateRecord{
		CreatedAt:   now.Add(-1 * time.Minute),
		RedirectURI: "http://x/callback",
	})
	service.googleOAuthStates.Store("expired", &googleOAuthStateRecord{
		CreatedAt:   now.Add(-2 * googleOAuthStateTTL),
		RedirectURI: "http://x/callback",
	})
	service.cleanupExpiredGoogleOAuthStates(now)
	if _, found := service.googleOAuthStates.Load("fresh"); !found {
		t.Error("fresh state should remain")
	}
	if _, found := service.googleOAuthStates.Load("expired"); found {
		t.Error("expired state should be removed")
	}
}

func TestPersistingGoogleOAuthTokenSourceWritesBackRefreshedToken(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1","client_secret":"secret-1"}}`)
	key, errorValue := service.loadOrCreateCalendarTokenEncryptionKey()
	if errorValue != nil {
		t.Fatalf("load key: %v", errorValue)
	}
	initial := &oauth2.Token{
		AccessToken:  "expired-access",
		RefreshToken: "refresh-stable",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-time.Hour),
	}
	accountID := googleOAuthAccountPrefix + "refresh"
	tokenPath := service.calendarTokenFilePath(accountID)
	if errorValue := writeCalendarTokenFile(tokenPath, key, oauthTokenPayloadFromOAuth2Token(initial)); errorValue != nil {
		t.Fatalf("seed token: %v", errorValue)
	}
	account := remoteCalendarAccount{
		ID:            accountID,
		Provider:      remoteCalendarProviderGoogle,
		AccountEmail:  "refresh@example.com",
		TokenFilePath: tokenPath,
	}
	if _, errorValue := service.upsertRemoteCalendarAccount(context.Background(), account); errorValue != nil {
		t.Fatalf("upsert: %v", errorValue)
	}

	tokenServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"access_token": "fresh-access",
			"token_type":   "Bearer",
			"expires_in":   3600,
		})
	}))
	defer tokenServer.Close()
	t.Setenv(googleTokenURLOverrideEnv, tokenServer.URL)

	source, errorValue := service.googleOAuthTokenSource(context.Background(), account,
		"http://admind.local"+googleOAuthCallbackPath)
	if errorValue != nil {
		t.Fatalf("token source: %v", errorValue)
	}
	refreshed, errorValue := source.Token()
	if errorValue != nil {
		t.Fatalf("refresh: %v", errorValue)
	}
	if refreshed.AccessToken != "fresh-access" {
		t.Errorf("access token: got %q", refreshed.AccessToken)
	}
	stored, errorValue := readCalendarTokenFile(tokenPath, key)
	if errorValue != nil {
		t.Fatalf("read token file: %v", errorValue)
	}
	if stored.AccessToken != "fresh-access" {
		t.Errorf("persisted access token: got %q", stored.AccessToken)
	}
	if stored.RefreshToken != "refresh-stable" {
		t.Errorf("refresh token should survive: got %q", stored.RefreshToken)
	}
}

func writeGoogleClientFile(t *testing.T, service *Service, content string) {
	t.Helper()
	directory := service.calendarSecretsDirectory()
	if errorValue := os.MkdirAll(directory, 0o700); errorValue != nil {
		t.Fatalf("mkdir: %v", errorValue)
	}
	path := filepath.Join(directory, googleOAuthClientFileName)
	if errorValue := os.WriteFile(path, []byte(content), 0o600); errorValue != nil {
		t.Fatalf("write client.json: %v", errorValue)
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
