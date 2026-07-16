package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

func TestCalendarEventLifecycleAndICS(t *testing.T) {
	service := newCalendarTestService(t)
	createRequest := httptest.NewRequest(http.MethodPost, "/calendar/api/events", strings.NewReader(`{
		"title":"Design review",
		"description":"Calendar polish",
		"location":"Studio",
		"startISO":"2026-05-08T01:00:00Z",
		"endISO":"2026-05-08T02:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#2563eb"
	}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	createResponse := httptest.NewRecorder()
	service.router().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}
	var createdEvent calendarEvent
	if errorValue := json.Unmarshal(createResponse.Body.Bytes(), &createdEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if createdEvent.ID == "" || createdEvent.UID == "" {
		t.Fatalf("created event identifiers missing: %#v", createdEvent)
	}
	if createdEvent.CreatedByEmail != "admin@example.com" || createdEvent.CreatedByName != "admin@example.com" {
		t.Fatalf("created actor = %q/%q", createdEvent.CreatedByEmail, createdEvent.CreatedByName)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/events?startISO=2026-05-01T00:00:00Z&endISO=2026-06-01T00:00:00Z", nil)
	listRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	listResponse := httptest.NewRecorder()
	service.router().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", listResponse.Code, listResponse.Body.String())
	}
	var eventsResponse calendarEventsResponse
	if errorValue := json.Unmarshal(listResponse.Body.Bytes(), &eventsResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(eventsResponse.Events) != 1 || eventsResponse.Events[0].Title != "Design review" {
		t.Fatalf("events response = %#v", eventsResponse)
	}

	syncRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/sync", nil)
	syncRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	syncResponse := httptest.NewRecorder()
	service.router().ServeHTTP(syncResponse, syncRequest)
	if syncResponse.Code != http.StatusOK {
		t.Fatalf("sync status = %d body = %s", syncResponse.Code, syncResponse.Body.String())
	}
	var syncDocument calendarSyncResponse
	if errorValue := json.Unmarshal(syncResponse.Body.Bytes(), &syncDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if syncDocument.CalDAVUsername != calendarDAVUsername || syncDocument.CalDAVPassword == "" {
		t.Fatalf("caldav credentials = %#v", syncDocument)
	}
	parsedCalDAVURL, errorValue := url.Parse(syncDocument.CalDAVURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	calDAVPassword, hasCalDAVPassword := parsedCalDAVURL.User.Password()
	if parsedCalDAVURL.User.Username() != calendarDAVUsername || !hasCalDAVPassword || calDAVPassword != syncDocument.CalDAVPassword {
		t.Fatalf("caldav url credentials = %q", syncDocument.CalDAVURL)
	}
	parsedICSURL, errorValue := url.Parse(syncDocument.ICSURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	token := strings.TrimSuffix(strings.TrimPrefix(parsedICSURL.Path, "/calendar/ics/"), ".ics")
	if token != syncDocument.CalDAVPassword {
		t.Fatalf("ics token and caldav password differ")
	}
	remoteSyncRequest := httptest.NewRequest(http.MethodPost, "/calendar/api/remote-sync", nil)
	remoteSyncRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	remoteSyncResponse := httptest.NewRecorder()
	service.router().ServeHTTP(remoteSyncResponse, remoteSyncRequest)
	if remoteSyncResponse.Code != http.StatusOK {
		t.Fatalf("remote sync status = %d body = %s", remoteSyncResponse.Code, remoteSyncResponse.Body.String())
	}
	var remoteSyncDocument calendarRemoteSyncResponse
	if errorValue := json.Unmarshal(remoteSyncResponse.Body.Bytes(), &remoteSyncDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if remoteSyncDocument.PullCacheTTLSeconds != int(calendarPullCacheTTL.Seconds()) {
		t.Fatalf("remote sync response missing pull cache ttl: %#v", remoteSyncDocument)
	}
	icsRequest := httptest.NewRequest(http.MethodGet, "/calendar/ics/"+token+".ics", nil)
	icsResponse := httptest.NewRecorder()
	service.router().ServeHTTP(icsResponse, icsRequest)
	if icsResponse.Code != http.StatusOK {
		t.Fatalf("ics status = %d body = %s", icsResponse.Code, icsResponse.Body.String())
	}
	if !strings.Contains(icsResponse.Body.String(), "SUMMARY:Design review") || !strings.Contains(icsResponse.Body.String(), "LOCATION:Studio") {
		t.Fatalf("ics body missing event: %s", icsResponse.Body.String())
	}
	if !strings.Contains(icsResponse.Body.String(), "CALSCALE:GREGORIAN") || !strings.Contains(icsResponse.Body.String(), "METHOD:PUBLISH") || !strings.Contains(icsResponse.Body.String(), "X-WR-CALNAME") {
		t.Fatalf("ics body missing calendar metadata: %s", icsResponse.Body.String())
	}
	if _, errorValue := ical.NewDecoder(strings.NewReader(icsResponse.Body.String())).Decode(); errorValue != nil {
		t.Fatal(errorValue)
	}

	rotateRequest := httptest.NewRequest(http.MethodPost, "/calendar/api/ics-token", nil)
	rotateRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	rotateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(rotateResponse, rotateRequest)
	if rotateResponse.Code != http.StatusOK {
		t.Fatalf("rotate status = %d body = %s", rotateResponse.Code, rotateResponse.Body.String())
	}
	oldICSResponse := httptest.NewRecorder()
	service.router().ServeHTTP(oldICSResponse, icsRequest)
	if oldICSResponse.Code != http.StatusNotFound {
		t.Fatalf("old token status = %d", oldICSResponse.Code)
	}
}

func TestCalendarEventStoresMattermostActorNames(t *testing.T) {
	service := newCalendarTestService(t)
	service.Configuration.MattermostBaseURL = "http://mattermost.local"
	service.Configuration.MattermostAdminPasswordPath = writeTestFile(t, "admin-password")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "http://127.0.0.1:8080/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"people":[{"emails":["creator@example.com"]},{"emails":["editor@example.com"]}]}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/me" && strings.Contains(request.Header.Get("Cookie"), "MMAUTHTOKEN=session-token"):
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"creator@example.com","username":"creator","nickname":"등록자"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/me" && strings.Contains(request.Header.Get("Cookie"), "MMAUTHTOKEN=editor-token"):
			return jsonResponse(http.StatusOK, `{"id":"user-2","email":"editor@example.com","username":"editor","display_name":"수정자"}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/email/creator@example.com":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"creator@example.com","username":"creator","nickname":"등록자","last_picture_update":1710000000000}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/email/editor@example.com":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"user-2","email":"editor@example.com","username":"editor","display_name":"수정자","last_picture_update":1710000000000}`, nil), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/user-1/image":
			assertMattermostBearerToken(t, request, "admin-token")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("creator-image")),
				Header:     http.Header{"Content-Type": []string{"image/png"}},
			}, nil
		}
		return jsonResponse(http.StatusNotFound, `{}`, nil), nil
	})}

	createRequest := httptest.NewRequest(http.MethodPost, "/calendar/api/events", strings.NewReader(`{
		"title":"Actor audit",
		"startISO":"2026-05-08T01:00:00Z",
		"endISO":"2026-05-08T02:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#2563eb"
	}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("Cookie", "MMAUTHTOKEN=session-token")
	createResponse := httptest.NewRecorder()
	service.router().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}
	var createdEvent calendarEvent
	if errorValue := json.Unmarshal(createResponse.Body.Bytes(), &createdEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if createdEvent.CreatedByEmail != "creator@example.com" || createdEvent.CreatedByName != "등록자" {
		t.Fatalf("created actor = %q/%q", createdEvent.CreatedByEmail, createdEvent.CreatedByName)
	}
	createdActorImage := "/calendar/api/events/" + url.PathEscape(createdEvent.ID) + "/actor-image?actor=created"
	if createdEvent.CreatedByImage != createdActorImage {
		t.Fatalf("created actor image = %q", createdEvent.CreatedByImage)
	}

	updatePayload := fmt.Sprintf(`{
		"title":"Actor audit edited",
		"startISO":"2026-05-08T03:00:00Z",
		"endISO":"2026-05-08T04:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#2563eb",
		"expectedUpdatedAt":%q
	}`, createdEvent.UpdatedAt)
	updateRequest := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+createdEvent.ID, strings.NewReader(updatePayload))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("Cookie", "MMAUTHTOKEN=editor-token")
	updateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	var updatedEvent calendarEvent
	if errorValue := json.Unmarshal(updateResponse.Body.Bytes(), &updatedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedActorImage := "/calendar/api/events/" + url.PathEscape(createdEvent.ID) + "/actor-image?actor=updated"
	if updatedEvent.CreatedByImage != createdActorImage || updatedEvent.UpdatedByImage != updatedActorImage {
		t.Fatalf("updated actor images = %q/%q", updatedEvent.CreatedByImage, updatedEvent.UpdatedByImage)
	}

	stored, found, errorValue := service.readCalendarEventByID(context.Background(), createdEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup after update: found=%v error=%v", found, errorValue)
	}
	if stored.CreatedByEmail != "creator@example.com" || stored.CreatedByName != "등록자" {
		t.Fatalf("stored created actor = %q/%q", stored.CreatedByEmail, stored.CreatedByName)
	}
	if stored.UpdatedByEmail != "editor@example.com" || stored.UpdatedByName != "수정자" || stored.UpdatedByAt == "" {
		t.Fatalf("stored updated actor = %q/%q at %q", stored.UpdatedByEmail, stored.UpdatedByName, stored.UpdatedByAt)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/events?startISO=2026-05-01T00:00:00Z&endISO=2026-06-01T00:00:00Z", nil)
	listRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	listResponse := httptest.NewRecorder()
	service.router().ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d body = %s", listResponse.Code, listResponse.Body.String())
	}
	var eventsResponse calendarEventsResponse
	if errorValue := json.Unmarshal(listResponse.Body.Bytes(), &eventsResponse); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(eventsResponse.Events) != 1 || eventsResponse.Events[0].CreatedByName != "등록자" || eventsResponse.Events[0].UpdatedByName != "수정자" {
		t.Fatalf("event actor names = %#v", eventsResponse.Events)
	}

	imageRequest := httptest.NewRequest(http.MethodGet, createdActorImage, nil)
	imageRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	imageResponse := httptest.NewRecorder()
	service.router().ServeHTTP(imageResponse, imageRequest)
	if imageResponse.Code != http.StatusOK || imageResponse.Body.String() != "creator-image" || imageResponse.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("image response = %d %q %q", imageResponse.Code, imageResponse.Body.String(), imageResponse.Header().Get("Content-Type"))
	}

	if _, errorValue := service.writeCalendarICSToken(context.Background(), "calendar-token"); errorValue != nil {
		t.Fatal(errorValue)
	}
	tokenImageRequest := httptest.NewRequest(http.MethodGet, createdActorImage, nil)
	tokenImageRequest.RemoteAddr = "203.0.113.10:49152"
	tokenImageRequest.SetBasicAuth(calendarDAVUsername, "calendar-token")
	tokenImageResponse := httptest.NewRecorder()
	service.router().ServeHTTP(tokenImageResponse, tokenImageRequest)
	if tokenImageResponse.Code != http.StatusForbidden {
		t.Fatalf("token image status = %d body = %s", tokenImageResponse.Code, tokenImageResponse.Body.String())
	}

	legacyImageRequest := httptest.NewRequest(http.MethodGet, "/calendar/api/actor-image?email="+url.QueryEscape("creator@example.com"), nil)
	legacyImageRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	legacyImageResponse := httptest.NewRecorder()
	service.router().ServeHTTP(legacyImageResponse, legacyImageRequest)
	if legacyImageResponse.Code != http.StatusNotFound {
		t.Fatalf("legacy image status = %d body = %s", legacyImageResponse.Code, legacyImageResponse.Body.String())
	}
}

func TestCalendarPeopleLineParsing(t *testing.T) {
	people, hasPeopleLine := calendarPeopleFromDescription("동하, 수민\nBring passport")
	if !hasPeopleLine || strings.Join(people, "|") != "동하|수민" {
		t.Fatalf("people=%+v hasPeopleLine=%v", people, hasPeopleLine)
	}
	users := []mattermostUserRecord{
		{ID: "user-1", Username: "dongha", Nickname: "동하", Email: "dongha@example.com"},
		{ID: "user-2", Username: "sumin", DisplayName: "수민", Email: "sumin@example.com"},
	}
	targets := calendarTargetsForPeople(people, users)
	if len(targets) != 2 || targets[0].Key != "dm:user-1" || targets[1].Key != "dm:user-2" {
		t.Fatalf("targets = %+v", targets)
	}
	if normalizeCalendarReminderLeadHours(5) != calendarDefaultReminderLeadHours || normalizeCalendarReminderLeadHours(48) != 48 {
		t.Fatal("reminder lead normalization failed")
	}
	if _, hasPeopleLine = calendarPeopleFromDescription("Calendar polish\nBring agenda"); hasPeopleLine {
		t.Fatal("general note line was parsed as people")
	}
}

func TestCalendarNotificationTimeMovesMorningReminderToPreviousEvening(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	startTime := time.Date(2026, 6, 16, 9, 0, 0, 0, location)
	event := calendarEvent{
		StartISO:          startTime.UTC().Format(time.RFC3339),
		TimeZone:          "Asia/Seoul",
		ReminderLeadHours: 3,
	}
	now := time.Date(2026, 6, 15, 12, 0, 0, 0, location).UTC()
	notifyAt, shouldNotify := calendarNotificationTime(event, now)
	expectedNotifyAt := time.Date(2026, 6, 15, 21, 0, 0, 0, location).UTC()
	if !shouldNotify || !notifyAt.Equal(expectedNotifyAt) {
		t.Fatalf("notifyAt=%s shouldNotify=%v", notifyAt.Format(time.RFC3339), shouldNotify)
	}
}

func TestCalendarNotificationTimeKeepsDaytimeReminder(t *testing.T) {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	startTime := time.Date(2026, 6, 16, 15, 0, 0, 0, location)
	event := calendarEvent{
		StartISO:          startTime.UTC().Format(time.RFC3339),
		TimeZone:          "Asia/Seoul",
		ReminderLeadHours: 3,
	}
	now := time.Date(2026, 6, 16, 8, 0, 0, 0, location).UTC()
	notifyAt, shouldNotify := calendarNotificationTime(event, now)
	expectedNotifyAt := time.Date(2026, 6, 16, 12, 0, 0, 0, location).UTC()
	if !shouldNotify || !notifyAt.Equal(expectedNotifyAt) {
		t.Fatalf("notifyAt=%s shouldNotify=%v", notifyAt.Format(time.RFC3339), shouldNotify)
	}
}

func assertCalendarProjectionOutboxCount(t *testing.T, service *Service, expectedCount int) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_channel_outbox").Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	if count != expectedCount {
		t.Fatalf("calendar projection outbox count = %d, want %d", count, expectedCount)
	}
}

func newCalendarTestService(t *testing.T) *Service {
	t.Helper()
	rootPath := t.TempDir()
	adminUIPath := filepath.Join(rootPath, "admin-ui")
	if errorValue := os.MkdirAll(adminUIPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "index.html"), []byte("<script></script>"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return NewService(Configuration{
		StateDirectory:           filepath.Join(rootPath, "state", "admin"),
		CompanionJobPath:         filepath.Join(rootPath, "state", "companion-jobs.json"),
		CalendarDatabasePath:     filepath.Join(rootPath, "state", "calendar.sqlite"),
		CalendarSecretsDirectory: filepath.Join(rootPath, "secrets", "google-oauth"),
		FlowDatabasePath:         filepath.Join(rootPath, "state", "flow.sqlite"),
		AdminEmailPath:           writeTestFile(t, "admin@example.com"),
		AdminUIPath:              adminUIPath,
		CalendarSyncDisabled:     true,
	})
}

func newCalendarMattermostTestService(t *testing.T, transport roundTripFunc) *Service {
	t.Helper()
	service := newCalendarTestService(t)
	service.Configuration.MattermostAdminPasswordPath = writeTestFile(t, "admin-password")
	service.Configuration.MattermostBotTokenPath = writeTestFile(t, "bot-token")
	service.Configuration.MattermostBaseURL = "http://mattermost.local"
	service.HTTPClient = &http.Client{Transport: transport}
	return service
}

type calendarMattermostLogRequests struct {
	createdMessages []string
	updatedMessages []string
	deletedPostIDs  []string
	createTokens    []string
	updateTokens    []string
}

func mattermostCalendarLogTestResponse(t *testing.T, request *http.Request) (*http.Response, bool) {
	switch {
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users":
		return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"dongha","nickname":"동하","email":"dongha@example.com"}]`, nil), true
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users/me":
		return jsonResponse(http.StatusOK, `{"id":"bot-1"}`, nil), true
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/name/internkim":
		return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), true
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/team-1/channels/name/calendar":
		return jsonResponse(http.StatusOK, `{"id":"calendar-channel"}`, nil), true
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/patch":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/channels/calendar-channel/posts":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), true
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/moderations/patch":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/calendar-channel/members":
		return jsonResponse(http.StatusCreated, `{}`, nil), true
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/members/bot-1/schemeRoles":
		return jsonResponse(http.StatusOK, `{}`, nil), true
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts" && mattermostPostChannelID(t, request) == "calendar-channel":
		return jsonResponse(http.StatusCreated, `{"id":"calendar-post-1"}`, nil), true
	default:
		return nil, false
	}
}

func mattermostCalendarLogLifecycleResponse(t *testing.T, request *http.Request, requests *calendarMattermostLogRequests) (*http.Response, error) {
	switch {
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/users/login":
		return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users":
		return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"iam","nickname":"김여명","email":"iam@example.com"}]`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users/me":
		return jsonResponse(http.StatusOK, `{"id":"bot-1"}`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/name/internkim":
		return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/team-1/channels/name/calendar":
		return jsonResponse(http.StatusOK, `{"id":"calendar-channel"}`, nil), nil
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/patch":
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/channels/calendar-channel/posts":
		return jsonResponse(http.StatusOK, `{"order":[],"posts":{}}`, nil), nil
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/moderations/patch":
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/calendar-channel/members":
		return jsonResponse(http.StatusCreated, `{}`, nil), nil
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/channels/calendar-channel/members/bot-1/schemeRoles":
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodGet && request.URL.Path == "/api/v4/posts/calendar-post-1":
		return jsonResponse(http.StatusOK, `{"id":"calendar-post-1","user_id":"bot-1"}`, nil), nil
	case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts":
		requests.createTokens = append(requests.createTokens, request.Header.Get("Authorization"))
		requests.createdMessages = append(requests.createdMessages, mattermostPostMessage(t, request))
		return jsonResponse(http.StatusCreated, `{"id":"calendar-post-1"}`, nil), nil
	case request.Method == http.MethodPut && request.URL.Path == "/api/v4/posts/calendar-post-1/patch":
		requests.updateTokens = append(requests.updateTokens, request.Header.Get("Authorization"))
		requests.updatedMessages = append(requests.updatedMessages, mattermostPostMessage(t, request))
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	case request.Method == http.MethodDelete && request.URL.Path == "/api/v4/posts/calendar-post-1":
		requests.deletedPostIDs = append(requests.deletedPostIDs, "calendar-post-1")
		return jsonResponse(http.StatusOK, `{}`, nil), nil
	default:
		t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.String())
		return nil, nil
	}
}

func mattermostPostChannelID(t *testing.T, request *http.Request) string {
	t.Helper()
	var payload map[string]any
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request.Body = io.NopCloser(strings.NewReader(string(document)))
	channelID, _ := payload["channel_id"].(string)
	return channelID
}

func calendarTestEvent(eventID string, title string, description string) calendarEvent {
	startTime := time.Now().UTC().Add(30 * time.Minute).Truncate(time.Second)
	endTime := startTime.Add(time.Hour)
	return calendarEvent{
		ID:                eventID,
		UID:               eventID + "@internkim",
		Title:             title,
		Description:       description,
		StartISO:          startTime.Format(time.RFC3339),
		EndISO:            endTime.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
	}
}

func postCalendarEventForDuplicateTest(t *testing.T, service *Service, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/calendar/api/events", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	recorder := httptest.NewRecorder()
	service.router().ServeHTTP(recorder, request)
	return recorder
}

func TestCreateCalendarEventDetectsSameSlotSameParticipantDuplicate(t *testing.T) {
	service := newCalendarTestService(t)
	baseSlot := `"startISO":"2026-07-02T09:30:00+09:00","endISO":"2026-07-02T10:30:00+09:00","timeZone":"Asia/Seoul"`

	created := postCalendarEventForDuplicateTest(t, service, `{"title":"세라에스이 사장님 미팅",`+baseSlot+`}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("first create status = %d body = %s", created.Code, created.Body.String())
	}

	duplicate := postCalendarEventForDuplicateTest(t, service, `{"title":"세라에스이 미팅",`+baseSlot+`}`)
	if duplicate.Code != http.StatusOK {
		t.Fatalf("same-slot same-participant add should return 200 duplicate_candidate, got %d body = %s", duplicate.Code, duplicate.Body.String())
	}
	var payload struct {
		Status     string          `json:"status"`
		Candidates []calendarEvent `json:"candidates"`
	}
	if errorValue := json.Unmarshal(duplicate.Body.Bytes(), &payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if payload.Status != "duplicate_candidate" || len(payload.Candidates) != 1 {
		t.Fatalf("duplicate response = %s", duplicate.Body.String())
	}

	forced := postCalendarEventForDuplicateTest(t, service, `{"title":"세라에스이 미팅",`+baseSlot+`,"allowDuplicate":true}`)
	if forced.Code != http.StatusCreated {
		t.Fatalf("allowDuplicate should bypass detection and create, got %d body = %s", forced.Code, forced.Body.String())
	}

	differentPeople := postCalendarEventForDuplicateTest(t, service, `{"title":"세라에스이 미팅",`+baseSlot+`,"people":["someone@dawn.kim"]}`)
	if differentPeople.Code != http.StatusCreated {
		t.Fatalf("same slot but different participants is not a duplicate; expected create, got %d body = %s", differentPeople.Code, differentPeople.Body.String())
	}
}
