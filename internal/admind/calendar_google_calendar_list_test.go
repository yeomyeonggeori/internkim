package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func TestServeGoogleCalendarListReturnsWritableCalendars(t *testing.T) {
	service := newCalendarTestService(t)
	account := seedGoogleCalendarListAccount(t, service)
	requests := 0
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Scheme+"://"+request.URL.Host+request.URL.Path != googleCalendarListEndpoint {
			t.Fatalf("unexpected request URL path: %s", request.URL.String())
		}
		if request.Header.Get("Authorization") != "Bearer access-1" {
			t.Fatalf("authorization header = %q", request.Header.Get("Authorization"))
		}
		if request.URL.Query().Get("maxResults") != googleCalendarListMaxResults {
			t.Fatalf("maxResults = %q", request.URL.Query().Get("maxResults"))
		}
		requests++
		switch request.URL.Query().Get("pageToken") {
		case "":
			return jsonResponse(http.StatusOK, `{"nextPageToken":"page-2","items":[{"id":"primary","summary":"최견본","accessRole":"owner","primary":true,"backgroundColor":"#1a73e8"},{"id":"readonly@example.com","summary":"읽기 전용","accessRole":"reader"}]}`, nil), nil
		case "page-2":
			return jsonResponse(http.StatusOK, `{"items":[{"id":"company@example.com","summary":"회사 공용","accessRole":"writer","backgroundColor":"#0f9d58"},{"id":"busy@example.com","summary":"바쁨","accessRole":"freeBusyReader"},{"summary":"ID 없음","accessRole":"owner"}]}`, nil), nil
		default:
			t.Fatalf("unexpected page token: %q", request.URL.Query().Get("pageToken"))
		}
		return nil, nil
	})}
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/calendar/api/google-calendars", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.serveGoogleCalendarList(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body googleCalendarListResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if body.AccountEmail != account.AccountEmail {
		t.Fatalf("accountEmail = %q, want %q", body.AccountEmail, account.AccountEmail)
	}
	if len(body.Calendars) != 2 {
		t.Fatalf("calendars = %#v, want 2 writable calendars", body.Calendars)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
	if body.Calendars[0].CalendarID != "primary" || body.Calendars[0].AccessRole != "owner" || !body.Calendars[0].Primary {
		t.Fatalf("primary calendar = %#v", body.Calendars[0])
	}
	if body.Calendars[1].CalendarID != "company@example.com" || body.Calendars[1].Summary != "회사 공용" || body.Calendars[1].BackgroundColor != "#0f9d58" {
		t.Fatalf("company calendar = %#v", body.Calendars[1])
	}
}

func TestServeGoogleCalendarListRequiresAdmin(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	request := httptest.NewRequest(http.MethodGet, "http://admind.local/calendar/api/google-calendars", nil)
	recorder := httptest.NewRecorder()

	service.serveGoogleCalendarList(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
}

func TestServeGoogleCalendarListRequiresConnectedAccount(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/calendar/api/google-calendars", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.serveGoogleCalendarList(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestServeGoogleCalendarListStoresGoogleError(t *testing.T) {
	service := newCalendarTestService(t)
	account := seedGoogleCalendarListAccount(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnauthorized, `{"error":"invalid_grant"}`, nil), nil
	})}
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/calendar/api/google-calendars", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.serveGoogleCalendarList(recorder, request)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadGateway)
	}
	reloaded, found, errorValue := service.readRemoteCalendarAccountByProvider(context.Background(), remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("reload account: %v", errorValue)
	}
	if !found || reloaded.ID != account.ID {
		t.Fatalf("reloaded account = %#v, found = %v", reloaded, found)
	}
	if !strings.Contains(reloaded.LastAuthError, "google calendar list status 401") {
		t.Fatalf("LastAuthError = %q", reloaded.LastAuthError)
	}
}

func TestSelectGoogleCalendarSavesWritableCalendar(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"items":[{"id":"primary","summary":"최견본","accessRole":"owner","primary":true},{"id":"company@example.com","summary":"회사 공용","accessRole":"writer","backgroundColor":"#0f9d58"}]}`, nil), nil
	})}
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-calendars/selection", strings.NewReader(`{"calendarID":"company@example.com"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.selectGoogleCalendar(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body selectGoogleCalendarResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	expectedCalendarURL := googleCalDAVBaseURL + "/company@example.com/events/"
	if body.SelectedCalendar.CalendarID != "company@example.com" || body.CalendarURL != expectedCalendarURL {
		t.Fatalf("selection response = %#v", body)
	}
	reloaded, found, errorValue := service.readRemoteCalendarAccountByProvider(context.Background(), remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("reload account: %v", errorValue)
	}
	if !found {
		t.Fatal("remote account missing")
	}
	if reloaded.SelectedCalendarID != "company@example.com" || reloaded.SelectedCalendarSummary != "회사 공용" || reloaded.SelectedCalendarAccessRole != "writer" {
		t.Fatalf("selected metadata = %#v", reloaded)
	}
	if reloaded.SelectedCalendarURL != expectedCalendarURL {
		t.Fatalf("SelectedCalendarURL = %q, want %q", reloaded.SelectedCalendarURL, expectedCalendarURL)
	}
	if reloaded.SelectedCalendarSelectedAt == "" {
		t.Fatal("SelectedCalendarSelectedAt missing")
	}
}

func TestSelectGoogleCalendarVerifiesSelectedCalendarEventsList(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	didVerifySelectedCalendar := false
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/calendar/v3/users/me/calendarList" {
			return jsonResponse(http.StatusOK, `{"items":[{"id":"company@example.com","summary":"회사 공용","accessRole":"writer"}]}`, nil), nil
		}
		if request.URL.Path == "/calendar/v3/calendars/company@example.com/events" {
			didVerifySelectedCalendar = true
			if request.URL.Query().Get("maxResults") != "1" {
				t.Fatalf("maxResults = %q", request.URL.Query().Get("maxResults"))
			}
			return jsonResponse(http.StatusOK, `{"items":[]}`, nil), nil
		}
		t.Fatalf("unexpected request path: %s", request.URL.String())
		return nil, nil
	})}
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-calendars/selection", strings.NewReader(`{"calendarID":"company@example.com"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.selectGoogleCalendar(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !didVerifySelectedCalendar {
		t.Fatal("selected calendar should be verified through events.list")
	}
}

func TestSelectGoogleCalendarUsesAccountEmailForPrimaryCalDAVURL(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"items":[{"id":"primary","summary":"최견본","accessRole":"owner","primary":true}]}`, nil), nil
	})}
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-calendars/selection", strings.NewReader(`{"calendarID":"primary"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.selectGoogleCalendar(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(context.Background(), remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("reload account: %v", errorValue)
	}
	expectedCalendarURL := googleCalDAVBaseURL + "/admin@example.com/events/"
	if reloaded.SelectedCalendarURL != expectedCalendarURL {
		t.Fatalf("SelectedCalendarURL = %q, want %q", reloaded.SelectedCalendarURL, expectedCalendarURL)
	}
}

func TestSelectGoogleCalendarEscapesCalDAVCalendarIDPath(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"items":[{"id":"company/schedule#shared@example.com","summary":"회사 공용","accessRole":"writer"}]}`, nil), nil
	})}
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-calendars/selection", strings.NewReader(`{"calendarID":"company/schedule#shared@example.com"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.selectGoogleCalendar(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(context.Background(), remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("reload account: %v", errorValue)
	}
	expectedCalendarURL := googleCalDAVBaseURL + "/company%2Fschedule%23shared@example.com/events/"
	if reloaded.SelectedCalendarURL != expectedCalendarURL {
		t.Fatalf("SelectedCalendarURL = %q, want %q", reloaded.SelectedCalendarURL, expectedCalendarURL)
	}
}

func TestGoogleCalendarEventsProbeURLEscapesCalendarIDPath(t *testing.T) {
	requestURL, errorValue := googleCalendarEventsProbeURL("company/schedule#shared@example.com")
	if errorValue != nil {
		t.Fatalf("probe URL: %v", errorValue)
	}
	if !strings.Contains(requestURL, "/company%2Fschedule%23shared@example.com/events?") {
		t.Fatalf("requestURL = %q", requestURL)
	}
}

func TestSelectGoogleCalendarRejectsReadOnlyCalendar(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"items":[{"id":"readonly@example.com","summary":"읽기 전용","accessRole":"reader"}]}`, nil), nil
	})}
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-calendars/selection", strings.NewReader(`{"calendarID":"readonly@example.com"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.selectGoogleCalendar(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(context.Background(), remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("reload account: %v", errorValue)
	}
	if reloaded.SelectedCalendarID != "" || reloaded.SelectedCalendarURL != "" {
		t.Fatalf("selection should be empty, got %#v", reloaded)
	}
}

func TestSelectGoogleCalendarRequiresCalendarID(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-calendars/selection", strings.NewReader(`{"calendarID":" "}`))
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.selectGoogleCalendar(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func seedGoogleCalendarListAccount(t *testing.T, service *Service) remoteCalendarAccount {
	t.Helper()
	writeGoogleClientFile(t, service, `{"installed":{"client_id":"client-1","client_secret":"secret-1"}}`)
	token := &oauth2.Token{
		AccessToken:  "access-1",
		RefreshToken: "refresh-1",
		TokenType:    "Bearer",
		Expiry:       time.Now().UTC().Add(time.Hour),
	}
	account, errorValue := service.saveGoogleOAuthTokenAndAccount(context.Background(), token, "admin@example.com")
	if errorValue != nil {
		t.Fatalf("save token account: %v", errorValue)
	}
	return account
}
