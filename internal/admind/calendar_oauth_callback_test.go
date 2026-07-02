package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

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
	const admindLocalURL = "http://127.0.0.1:18180"

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

	const calendarReturnURL = "http://127.0.0.1:5174/calendar/"
	startRequest := httptest.NewRequest(http.MethodGet,
		admindLocalURL+googleOAuthStartPath+"?returnTo="+url.QueryEscape(calendarReturnURL), nil)
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

	callbackURL := admindLocalURL + googleOAuthCallbackPath + "?state=" + url.QueryEscape(state) + "&code=auth-code-1"
	callbackRequest := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	callbackRequest.Header.Set("Accept-Language", "en-US,en;q=0.9")
	callbackRecorder := httptest.NewRecorder()
	service.handleGoogleOAuthCallback(callbackRecorder, callbackRequest)
	if callbackRecorder.Code != http.StatusSeeOther {
		t.Fatalf("callback status: %d, body: %s", callbackRecorder.Code, callbackRecorder.Body.String())
	}
	if location := callbackRecorder.Header().Get("Location"); location != calendarReturnURL+"?googleOAuth=connected" {
		t.Errorf("callback redirect location: got %q", location)
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

func TestGoogleOAuthCallbackRedirectsGoogleErrorToReturnURL(t *testing.T) {
	service := newCalendarTestService(t)
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1","client_secret":"secret-1"}}`)
	const admindLocalURL = "http://127.0.0.1:18180"
	const calendarReturnURL = "http://127.0.0.1:5174/calendar/"
	startRequest := httptest.NewRequest(http.MethodGet,
		admindLocalURL+googleOAuthStartPath+"?returnTo="+url.QueryEscape(calendarReturnURL), nil)
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

	callbackURL := admindLocalURL + googleOAuthCallbackPath + "?state=" + url.QueryEscape(state) + "&error=access_denied"
	callbackRequest := httptest.NewRequest(http.MethodGet, callbackURL, nil)
	callbackRecorder := httptest.NewRecorder()
	service.handleGoogleOAuthCallback(callbackRecorder, callbackRequest)
	if callbackRecorder.Code != http.StatusSeeOther {
		t.Fatalf("callback status: %d, body: %s", callbackRecorder.Code, callbackRecorder.Body.String())
	}
	if location := callbackRecorder.Header().Get("Location"); location != calendarReturnURL+"?googleOAuth=failed" {
		t.Errorf("callback redirect location: got %q", location)
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
