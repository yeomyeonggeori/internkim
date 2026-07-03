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

func TestServeGoogleCalendarListReturnsSelectableCalendarMetadataFromCalDAV(t *testing.T) {
	service := newCalendarTestService(t)
	account := seedGoogleCalendarListAccount(t, service)
	requestPaths := []string{}
	service.HTTPClient = &http.Client{Transport: googleCalDAVFixtureTransport(t, &requestPaths, []googleCalDAVCalendarFixture{
		{
			Path:        "/caldav/v2/admin@example.com/events/",
			DisplayName: "최견본",
			Color:       "#1a73e8",
			Components:  []string{"VEVENT"},
			Privileges:  []string{"read", "write"},
		},
		{
			Path:        "/caldav/v2/readonly@example.com/events/",
			DisplayName: "읽기 전용",
			Components:  []string{"VEVENT"},
			Privileges:  []string{"read"},
		},
		{
			Path:        "/caldav/v2/company@example.com/events/",
			DisplayName: "회사 공용",
			Color:       "#0f9d58",
			Components:  []string{"VEVENT"},
			Privileges:  []string{"read", "write-content"},
		},
		{
			Path:        "/caldav/v2/tasks%23group.v.calendar.google.com/events/",
			DisplayName: "Tasks",
			Components:  []string{"VTODO"},
			Privileges:  []string{"read", "write"},
		},
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
	if len(body.Calendars) != 4 {
		t.Fatalf("calendars = %#v, want 4 calendars with selection metadata", body.Calendars)
	}
	if !containsCalendarTestString(requestPaths, "/caldav/v2/admin@example.com/") {
		t.Fatalf("request paths = %#v, want CalDAV home set list", requestPaths)
	}
	if body.Calendars[0].CalendarID != "admin@example.com" || body.Calendars[0].AccessRole != "writer" || !body.Calendars[0].Primary || !body.Calendars[0].CanWrite || !body.Calendars[0].CanSelect {
		t.Fatalf("primary calendar = %#v", body.Calendars[0])
	}
	if body.Calendars[1].CalendarID != "readonly@example.com" || body.Calendars[1].CanWrite || body.Calendars[1].CanSelect || body.Calendars[1].SelectionDisabledReason != googleCalendarSelectionDisabledReasonWritePermissionRequired {
		t.Fatalf("readonly calendar = %#v", body.Calendars[1])
	}
	if body.Calendars[2].CalendarID != "company@example.com" || body.Calendars[2].Summary != "회사 공용" || body.Calendars[2].BackgroundColor != "#0f9d58" || !body.Calendars[2].CanSelect {
		t.Fatalf("company calendar = %#v", body.Calendars[2])
	}
	if body.Calendars[3].CalendarID != "tasks#group.v.calendar.google.com" || !body.Calendars[3].CanWrite || body.Calendars[3].CanSelect || body.Calendars[3].SelectionDisabledReason != googleCalendarSelectionDisabledReasonUnsupportedCalendar {
		t.Fatalf("tasks calendar = %#v", body.Calendars[3])
	}
}

func TestServeGoogleCalendarListUsesGoogleHomeSetURLWithoutPrincipalDiscovery(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	requestPaths := []string{}
	service.HTTPClient = &http.Client{Transport: googleCalDAVFixtureTransportWithProbe(t, []googleCalDAVCalendarFixture{{
		Path:        "/caldav/v2/admin@example.com/events/",
		DisplayName: "최견본",
		Components:  []string{"VEVENT"},
		Privileges:  []string{"read", "write"},
	}}, func(request *http.Request) {
		requestPaths = append(requestPaths, request.URL.EscapedPath())
		if strings.Contains(request.URL.EscapedPath(), "/user") {
			t.Fatalf("unexpected principal discovery request: %s", request.URL.String())
		}
	})}
	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/calendar/api/google-calendars", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.serveGoogleCalendarList(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !containsCalendarTestString(requestPaths, "/caldav/v2/admin@example.com/") {
		t.Fatalf("request paths = %#v, want direct Google CalDAV home set list", requestPaths)
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

func TestServeGoogleCalendarListStoresCalDAVAuthorizationError(t *testing.T) {
	service := newCalendarTestService(t)
	account := seedGoogleCalendarListAccount(t, service)
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return xmlResponse(http.StatusUnauthorized, `<D:error xmlns:D="DAV:">invalid_grant</D:error>`), nil
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
	if !strings.Contains(reloaded.LastAuthError, "401") {
		t.Fatalf("LastAuthError = %q", reloaded.LastAuthError)
	}
}

func TestSelectGoogleCalendarSavesWritableCalDAVCalendar(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	service.HTTPClient = &http.Client{Transport: googleCalDAVFixtureTransport(t, nil, []googleCalDAVCalendarFixture{{
		Path:        "/caldav/v2/company@example.com/events/",
		DisplayName: "회사 공용",
		Color:       "#0f9d58",
		Components:  []string{"VEVENT"},
		Privileges:  []string{"read", "write"},
	}})}
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

func TestSelectGoogleCalendarVerifiesSelectedCalendarWithCalDAVPropFind(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	didVerifySelectedCalendar := false
	service.HTTPClient = &http.Client{Transport: googleCalDAVFixtureTransportWithProbe(t, []googleCalDAVCalendarFixture{{
		Path:        "/caldav/v2/company@example.com/events/",
		DisplayName: "회사 공용",
		Components:  []string{"VEVENT"},
		Privileges:  []string{"read", "write"},
	}}, func(request *http.Request) {
		if request.URL.EscapedPath() == "/caldav/v2/company@example.com/events/" {
			didVerifySelectedCalendar = true
		}
		if strings.HasPrefix(request.URL.Path, "/calendar/v3/") {
			t.Fatalf("unexpected Google Calendar REST request: %s", request.URL.String())
		}
	})}
	request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/calendar/api/google-calendars/selection", strings.NewReader(`{"calendarID":"company@example.com"}`))
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	service.selectGoogleCalendar(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !didVerifySelectedCalendar {
		t.Fatal("selected calendar should be verified through CalDAV PROPFIND")
	}
}

func TestSelectGoogleCalendarStoresDiscoveredCalendarURL(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	service.HTTPClient = &http.Client{Transport: googleCalDAVFixtureTransport(t, nil, []googleCalDAVCalendarFixture{{
		Path:        "/caldav/v2/company%2Fschedule%23shared@example.com/events/",
		DisplayName: "회사 공용",
		Components:  []string{"VEVENT"},
		Privileges:  []string{"read", "write"},
	}})}
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

func TestGoogleCalendarIDFromCalDAVPathDecodesEscapedCalendarSegment(t *testing.T) {
	calendarID := googleCalendarIDFromCalDAVPath("/caldav/v2/company%2Fschedule%23shared@example.com/events/", "admin@example.com")
	if calendarID != "company/schedule#shared@example.com" {
		t.Fatalf("calendarID = %q", calendarID)
	}
}

func TestSelectGoogleCalendarRejectsReadOnlyCalendar(t *testing.T) {
	service := newCalendarTestService(t)
	seedGoogleCalendarListAccount(t, service)
	service.HTTPClient = &http.Client{Transport: googleCalDAVFixtureTransport(t, nil, []googleCalDAVCalendarFixture{{
		Path:        "/caldav/v2/readonly@example.com/events/",
		DisplayName: "읽기 전용",
		Components:  []string{"VEVENT"},
		Privileges:  []string{"read"},
	}})}
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
