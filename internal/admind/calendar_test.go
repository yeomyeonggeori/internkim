package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
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
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"
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
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/me" && strings.Contains(request.Header.Get("Cookie"), "MMAUTHTOKEN=session-token") {
			return jsonResponse(http.StatusOK, `{"id":"user-1","email":"creator@example.com","username":"creator","nickname":"등록자"}`, nil), nil
		}
		if request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/me" && strings.Contains(request.Header.Get("Cookie"), "MMAUTHTOKEN=editor-token") {
			return jsonResponse(http.StatusOK, `{"id":"user-2","email":"editor@example.com","username":"editor","display_name":"수정자"}`, nil), nil
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

	updateRequest := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+createdEvent.ID, strings.NewReader(`{
		"title":"Actor audit edited",
		"startISO":"2026-05-08T03:00:00Z",
		"endISO":"2026-05-08T04:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#2563eb"
	}`))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("Cookie", "MMAUTHTOKEN=editor-token")
	updateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
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
}

func TestCalendarDAVAcceptsTokenBasicAuth(t *testing.T) {
	service := newCalendarTestService(t)
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

	authorizedRequest := httptest.NewRequest("PROPFIND", calendarCollectionPath, nil)
	authorizedRequest.RemoteAddr = "203.0.113.10:49152"
	authorizedRequest.SetBasicAuth(calendarDAVUsername, syncDocument.CalDAVPassword)
	if !service.authorizeCalendarRequest(authorizedRequest) {
		t.Fatal("calendar token basic auth was rejected")
	}

	rejectedRequest := httptest.NewRequest("PROPFIND", calendarCollectionPath, nil)
	rejectedRequest.RemoteAddr = "203.0.113.10:49152"
	rejectedRequest.SetBasicAuth(calendarDAVUsername, "wrong-token")
	if service.authorizeCalendarRequest(rejectedRequest) {
		t.Fatal("wrong calendar token was accepted")
	}
}

func TestCalendarDAVBackendStoresCalendarObjects(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	calendar := newCalendarDocument()
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, "client-event@example.com")
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC))
	event.Props.SetText(ical.PropSummary, "Client event")
	event.Props.SetDateTime(ical.PropDateTimeStart, time.Date(2026, 5, 8, 3, 0, 0, 0, time.UTC))
	event.Props.SetDateTime(ical.PropDateTimeEnd, time.Date(2026, 5, 8, 4, 0, 0, 0, time.UTC))
	calendar.Children = append(calendar.Children, event.Component)

	object, errorValue := backend.PutCalendarObject(context.Background(), calendarCollectionPath+"client-event.ics", calendar, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if object.Path != calendarCollectionPath+"client-event.ics" || object.ETag == "" {
		t.Fatalf("stored object = %#v", object)
	}
	storedObject, errorValue := backend.GetCalendarObject(context.Background(), calendarCollectionPath+"client-event.ics", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedObject.Data.Events()[0].Props.Get(ical.PropSummary).Value != "Client event" {
		t.Fatalf("stored summary = %#v", storedObject.Data.Events()[0].Props.Get(ical.PropSummary))
	}
	if errorValue := backend.DeleteCalendarObject(context.Background(), calendarCollectionPath+"client-event.ics"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := backend.GetCalendarObject(context.Background(), calendarCollectionPath+"client-event.ics", nil); errorValue == nil {
		t.Fatal("deleted calendar object was returned")
	}
}

func TestCalendarPeopleLineParsing(t *testing.T) {
	people, hasPeopleLine := calendarPeopleFromDescription("샘플, 수민\nBring passport")
	if !hasPeopleLine || strings.Join(people, "|") != "샘플|수민" {
		t.Fatalf("people=%+v hasPeopleLine=%v", people, hasPeopleLine)
	}
	users := []mattermostUserRecord{
		{ID: "user-1", Username: "dongha", Nickname: "샘플", Email: "dongha@example.com"},
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

func TestCalendarNotificationPostsAnnouncementsForAllHands(t *testing.T) {
	var createdChannel map[string]any
	var postedMessage string
	postCount := 0
	service := newCalendarMattermostTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, handled := mattermostCalendarLogTestResponse(t, request); handled {
			return response, nil
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users":
			return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"dongha","nickname":"샘플","email":"dongha@example.com"}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/name/internkim":
			return jsonResponse(http.StatusOK, `{"id":"team-1"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/teams/team-1/channels/name/announcements":
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels":
			if errorValue := json.NewDecoder(request.Body).Decode(&createdChannel); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusCreated, `{"id":"announcements-channel"}`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users/username/internkim":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/announcements-channel/members":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusCreated, `{}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts":
			assertMattermostBearerToken(t, request, "bot-token")
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			postCount++
			postedMessage, _ = payload["message"].(string)
			return jsonResponse(http.StatusCreated, `{"id":"post-1"}`, nil), nil
		default:
			t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})

	event := calendarTestEvent("all-hands", "Company offsite", "Travel prep")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.processDueCalendarNotifications(context.Background(), time.Now().UTC().Add(time.Second))
	service.processDueCalendarNotifications(context.Background(), time.Now().UTC().Add(2*time.Second))

	if createdChannel["name"] != calendarAnnouncementsChannelName || createdChannel["display_name"] != announcementsChannelDisplayName(workspaceLanguageKorean) {
		t.Fatalf("created channel = %+v", createdChannel)
	}
	if postCount != 1 {
		t.Fatalf("postCount = %d", postCount)
	}
	if !strings.Contains(postedMessage, "Company offsite") || !strings.Contains(postedMessage, "Travel prep") || strings.Contains(postedMessage, "일정 열기") {
		t.Fatalf("posted message = %q", postedMessage)
	}
}

func TestCalendarNotificationPostsDirectMessageForPeopleLine(t *testing.T) {
	var directChannelMembers []string
	var postedMessage string
	service := newCalendarMattermostTestService(t, func(request *http.Request) (*http.Response, error) {
		if response, handled := mattermostCalendarLogTestResponse(t, request); handled {
			return response, nil
		}
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users":
			return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"dongha","nickname":"샘플","email":"dongha@example.com"}]`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/api/v4/users/username/internkim":
			assertMattermostBearerToken(t, request, "admin-token")
			return jsonResponse(http.StatusOK, `{"id":"bot-1","username":"internkim"}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/channels/direct":
			if errorValue := json.NewDecoder(request.Body).Decode(&directChannelMembers); errorValue != nil {
				t.Fatal(errorValue)
			}
			return jsonResponse(http.StatusCreated, `{"id":"dm-channel"}`, nil), nil
		case request.Method == http.MethodPut && request.URL.Path == "/api/v4/users/user-1/preferences":
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		case request.Method == http.MethodPost && request.URL.Path == "/api/v4/posts":
			assertMattermostBearerToken(t, request, "bot-token")
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatal(errorValue)
			}
			postedMessage, _ = payload["message"].(string)
			return jsonResponse(http.StatusCreated, `{"id":"post-1"}`, nil), nil
		default:
			t.Fatalf("unexpected Mattermost request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})

	event := calendarTestEvent("targeted", "Online sync", "샘플\nBring agenda")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.processDueCalendarNotifications(context.Background(), time.Now().UTC().Add(time.Second))

	if strings.Join(directChannelMembers, "|") != "user-1|bot-1" {
		t.Fatalf("direct members = %+v", directChannelMembers)
	}
	if !strings.Contains(postedMessage, "Online sync") || !strings.Contains(postedMessage, "Bring agenda") || strings.Contains(postedMessage, "샘플") || strings.Contains(postedMessage, "일정 열기") {
		t.Fatalf("posted message = %q", postedMessage)
	}
}

func TestCalendarMattermostLogCreatesUpdatesAndDeletesPost(t *testing.T) {
	requests := calendarMattermostLogRequests{}
	service := newCalendarMattermostTestService(t, func(request *http.Request) (*http.Response, error) {
		return mattermostCalendarLogLifecycleResponse(t, request, &requests)
	})
	event := calendarTestEvent("logged", "Design review", "샘플\nBring agenda")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	reloadedEvent, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID)
	if errorValue != nil || !found {
		t.Fatalf("expected reloaded event: found=%v error=%v", found, errorValue)
	}
	if reloadedEvent.MattermostPostID != "calendar-post-1" {
		t.Fatalf("post id = %q", reloadedEvent.MattermostPostID)
	}
	if len(requests.createdMessages) != 1 || !strings.Contains(requests.createdMessages[0], "Design review") || !strings.Contains(requests.createdMessages[0], "대상: 샘플") || strings.Contains(requests.createdMessages[0], "일정 열기") {
		t.Fatalf("created messages = %+v", requests.createdMessages)
	}
	if len(requests.createTokens) != 1 || requests.createTokens[0] != "Bearer bot-token" {
		t.Fatalf("create tokens = %+v", requests.createTokens)
	}
	reloadedEvent.Title = "Updated review"
	if errorValue := service.writeCalendarEvent(context.Background(), reloadedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(requests.updatedMessages) != 1 || !strings.Contains(requests.updatedMessages[0], "Updated review") || strings.Contains(requests.updatedMessages[0], "일정 열기") {
		t.Fatalf("updated messages = %+v", requests.updatedMessages)
	}
	if len(requests.updateTokens) != 1 || requests.updateTokens[0] != "Bearer bot-token" {
		t.Fatalf("update tokens = %+v", requests.updateTokens)
	}
	if errorValue := service.softDeleteCalendarEvent(context.Background(), reloadedEvent.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(requests.deletedPostIDs) != 1 || requests.deletedPostIDs[0] != "calendar-post-1" {
		t.Fatalf("deleted posts = %+v", requests.deletedPostIDs)
	}
}

func TestCalendarNotificationCancelsWhenEventIsDeleted(t *testing.T) {
	service := newCalendarTestService(t)
	event := calendarTestEvent("cancel-me", "Canceled meeting", "")
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(context.Background(), event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var status string
	row := database.QueryRowContext(context.Background(), "SELECT status FROM calendar_event_notifications WHERE event_id = ?", event.ID)
	if errorValue := row.Scan(&status); errorValue != nil {
		t.Fatal(errorValue)
	}
	if status != "canceled" {
		t.Fatalf("status = %q", status)
	}
}

func TestCalendarPropPatchStoresAppleColor(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/">
  <D:set><D:prop><A:calendar-color>#FF0000</A:calendar-color></D:prop></D:set>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 1 {
		t.Fatalf("properties = %#v", properties)
	}
	if properties[0].XMLName.Space != "http://apple.com/ns/ical/" || properties[0].XMLName.Local != "calendar-color" || properties[0].Value != "#FF0000" {
		t.Fatalf("stored property mismatch: %#v", properties[0])
	}
}

func TestCalendarPropPatchStoresDAVDisplayName(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:">
  <D:set><D:prop><D:displayname>Team Calendar</D:displayname></D:prop></D:set>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 1 || properties[0].XMLName.Space != "DAV:" || properties[0].XMLName.Local != "displayname" || properties[0].Value != "Team Calendar" {
		t.Fatalf("stored properties mismatch: %#v", properties)
	}
}

func TestCalendarPropPatchRejectsNonWhitelistedNamespace(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:M="http://mozilla.org/ns/calendar/">
  <D:set><D:prop><M:calendar-color>#00FF00</M:calendar-color></D:prop></D:set>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "403 Forbidden") {
		t.Fatalf("response missing 403 propstat for non-whitelisted property: %s", response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 0 {
		t.Fatalf("non-whitelisted property must not be persisted: %#v", properties)
	}
}

func TestCalendarPropPatchRejectsNestedXMLValue(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/">
  <D:set><D:prop><A:calendar-color><A:child>x</A:child></A:calendar-color></D:prop></D:set>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "403 Forbidden") {
		t.Fatalf("response missing 403 propstat for nested xml value: %s", response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 0 {
		t.Fatalf("calendar-color with nested element must not be persisted: %#v", properties)
	}
}

func TestCalendarPropPatchDecodesXMLEntitiesAndRoundTrips(t *testing.T) {
	service := newCalendarTestService(t)
	patchBody := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:">
  <D:set><D:prop><D:displayname>R&amp;D Team</D:displayname></D:prop></D:set>
</D:propertyupdate>`
	patchRequest := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(patchBody))
	patchRequest.Header.Set("Content-Type", "application/xml")
	patchRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	patchResponse := httptest.NewRecorder()
	service.router().ServeHTTP(patchResponse, patchRequest)
	if patchResponse.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", patchResponse.Code, patchResponse.Body.String())
	}

	storedProperties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(storedProperties) != 1 || storedProperties[0].Value != "R&D Team" {
		t.Fatalf("stored value mismatch: %#v", storedProperties)
	}

	findBody := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`
	findRequest := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(findBody))
	findRequest.Header.Set("Content-Type", "application/xml")
	findRequest.Header.Set("Depth", "0")
	findRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	findResponse := httptest.NewRecorder()
	service.router().ServeHTTP(findResponse, findRequest)
	if findResponse.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", findResponse.Code, findResponse.Body.String())
	}
	if strings.Contains(findResponse.Body.String(), "&amp;amp;") {
		t.Fatalf("propfind response double-escaped XML entity:\n%s", findResponse.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, findResponse.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	displayName := xml.Name{Space: "DAV:", Local: "displayname"}
	rawValue, ok := results[0].OK[displayName]
	if !ok {
		t.Fatalf("displayname missing from 200 OK propstat: %#v", results[0])
	}
	if rawValue != "R&amp;D Team" {
		t.Fatalf("displayname inner XML = %q, want %q (single-escaped)", rawValue, "R&amp;D Team")
	}
}

func TestCalendarPropPatchRemovesProperty(t *testing.T) {
	service := newCalendarTestService(t)
	if errorValue := service.writeCalendarProperty(context.Background(), calendarCollectionPath, "http://apple.com/ns/ical/", "calendar-color", "#FF0000"); errorValue != nil {
		t.Fatal(errorValue)
	}
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propertyupdate xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/">
  <D:remove><D:prop><A:calendar-color/></D:prop></D:remove>
</D:propertyupdate>`
	request := httptest.NewRequest("PROPPATCH", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("proppatch status = %d body = %s", response.Code, response.Body.String())
	}
	properties, errorValue := service.readCalendarProperties(context.Background(), calendarCollectionPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(properties) != 0 {
		t.Fatalf("property was not removed: %#v", properties)
	}
}

func TestCalendarPropFindIncludesStoredProperties(t *testing.T) {
	service := newCalendarTestService(t)
	if errorValue := service.writeCalendarProperty(context.Background(), calendarCollectionPath, "http://apple.com/ns/ical/", "calendar-color", "#FF0000"); errorValue != nil {
		t.Fatal(errorValue)
	}
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/">
  <D:prop><A:calendar-color/></D:prop>
</D:propfind>`
	request := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("Depth", "0")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", response.Code, response.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, response.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	if results[0].Href != calendarCollectionPath {
		t.Fatalf("href mismatch: %q", results[0].Href)
	}
	colorName := xml.Name{Space: "http://apple.com/ns/ical/", Local: "calendar-color"}
	value, ok := results[0].OK[colorName]
	if !ok {
		t.Fatalf("calendar-color missing from 200 OK propstat: %#v", results[0])
	}
	if value != "#FF0000" {
		t.Fatalf("calendar-color value = %q, want %q", value, "#FF0000")
	}
}

func TestCalendarSyncCollectionRejectedAsUnsupported(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:sync-collection xmlns:D="DAV:">
  <D:sync-token/><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop>
</D:sync-collection>`
	request := httptest.NewRequest("REPORT", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("sync-collection status = %d body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "supported-report") {
		t.Fatalf("response missing supported-report precondition: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "valid-sync-token") {
		t.Fatalf("response must not signal valid-sync-token (would loop client retries): %s", response.Body.String())
	}
}

func TestCalendarDAVPutAcceptsCurrentIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	objectPath := calendarCollectionPath + "ifmatch-current.ics"

	initial := newCalendarDocumentWithEvent("ifmatch-current@example.com", "Original")
	stored, errorValue := backend.PutCalendarObject(context.Background(), objectPath, initial, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	updated := newCalendarDocumentWithEvent("ifmatch-current@example.com", "Updated")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, updated, &caldav.PutCalendarObjectOptions{
		IfMatch: webdav.ConditionalMatch(`"` + stored.ETag + `"`),
	}); errorValue != nil {
		t.Fatalf("update with current ETag failed: %v", errorValue)
	}

	current, errorValue := backend.GetCalendarObject(context.Background(), objectPath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if current.Data.Events()[0].Props.Get(ical.PropSummary).Value != "Updated" {
		t.Fatalf("expected updated summary, got %s", current.Data.Events()[0].Props.Get(ical.PropSummary).Value)
	}
}

func TestCalendarDAVPutRejectsStaleIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	objectPath := calendarCollectionPath + "ifmatch-stale.ics"

	initial := newCalendarDocumentWithEvent("ifmatch-stale@example.com", "Original")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, initial, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	updated := newCalendarDocumentWithEvent("ifmatch-stale@example.com", "Overwrite")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, updated, &caldav.PutCalendarObjectOptions{
		IfMatch: webdav.ConditionalMatch(`"stale-etag-xyz"`),
	}); errorValue == nil {
		t.Fatal("expected stale If-Match to be rejected")
	}

	stored, errorValue := backend.GetCalendarObject(context.Background(), objectPath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored.Data.Events()[0].Props.Get(ical.PropSummary).Value != "Original" {
		t.Fatalf("stale PUT overwrote data: got summary %s", stored.Data.Events()[0].Props.Get(ical.PropSummary).Value)
	}
}

func TestCalendarDAVPutHTTPRejectsStaleIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	objectPath := calendarCollectionPath + "http-stale.ics"

	initial := encodeCalendarTestICS(t, "http-stale@example.com", "Original")
	createRequest := httptest.NewRequest(http.MethodPut, objectPath, strings.NewReader(initial))
	createRequest.Header.Set("Content-Type", "text/calendar")
	createRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	createResponse := httptest.NewRecorder()
	service.router().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated && createResponse.Code != http.StatusNoContent && createResponse.Code != http.StatusOK {
		t.Fatalf("initial PUT status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}

	updated := encodeCalendarTestICS(t, "http-stale@example.com", "Overwrite")
	updateRequest := httptest.NewRequest(http.MethodPut, objectPath, strings.NewReader(updated))
	updateRequest.Header.Set("Content-Type", "text/calendar")
	updateRequest.Header.Set("If-Match", `"stale-etag-xyz"`)
	updateRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	updateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusPreconditionFailed {
		t.Fatalf("stale If-Match: expected 412, got %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}

	stored, errorValue := calendarDAVBackend{service: service}.GetCalendarObject(context.Background(), objectPath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored.Data.Events()[0].Props.Get(ical.PropSummary).Value != "Original" {
		t.Fatalf("stale PUT must not overwrite event")
	}
}

func TestCalendarDAVPutHTTPRejectsIfNoneMatchWildcardWhenExists(t *testing.T) {
	service := newCalendarTestService(t)
	objectPath := calendarCollectionPath + "http-ifnonematch.ics"

	initial := encodeCalendarTestICS(t, "http-ifnonematch@example.com", "Original")
	createRequest := httptest.NewRequest(http.MethodPut, objectPath, strings.NewReader(initial))
	createRequest.Header.Set("Content-Type", "text/calendar")
	createRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	createResponse := httptest.NewRecorder()
	service.router().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated && createResponse.Code != http.StatusNoContent && createResponse.Code != http.StatusOK {
		t.Fatalf("initial PUT status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}

	replacement := encodeCalendarTestICS(t, "http-ifnonematch@example.com", "Replacement")
	replaceRequest := httptest.NewRequest(http.MethodPut, objectPath, strings.NewReader(replacement))
	replaceRequest.Header.Set("Content-Type", "text/calendar")
	replaceRequest.Header.Set("If-None-Match", "*")
	replaceRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	replaceResponse := httptest.NewRecorder()
	service.router().ServeHTTP(replaceResponse, replaceRequest)
	if replaceResponse.Code != http.StatusPreconditionFailed {
		t.Fatalf("If-None-Match: * with existing object: expected 412, got %d body = %s", replaceResponse.Code, replaceResponse.Body.String())
	}
}

func encodeCalendarTestICS(t *testing.T, uid string, summary string) string {
	t.Helper()
	cal := newCalendarDocumentWithEvent(uid, summary)
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(cal); errorValue != nil {
		t.Fatal(errorValue)
	}
	return buffer.String()
}

func TestCalendarDAVPutRejectsIfNoneMatchWildcardWhenExists(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	objectPath := calendarCollectionPath + "ifnonematch.ics"

	initial := newCalendarDocumentWithEvent("ifnonematch@example.com", "Original")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, initial, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	replacement := newCalendarDocumentWithEvent("ifnonematch@example.com", "Replacement")
	if _, errorValue := backend.PutCalendarObject(context.Background(), objectPath, replacement, &caldav.PutCalendarObjectOptions{
		IfNoneMatch: webdav.ConditionalMatch("*"),
	}); errorValue == nil {
		t.Fatal("expected If-None-Match wildcard to fail for existing event")
	}
}

func TestCalendarPropFindIncludesGetCTag(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:CS="http://calendarserver.org/ns/">
  <D:prop><CS:getctag/></D:prop>
</D:propfind>`
	request := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("Depth", "0")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", response.Code, response.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, response.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	ctagName := xml.Name{Space: "http://calendarserver.org/ns/", Local: "getctag"}
	value, ok := results[0].OK[ctagName]
	if !ok {
		t.Fatalf("getctag missing from 200 OK propstat: %#v", results[0])
	}
	if !strings.HasPrefix(value, "v1-") {
		t.Fatalf("getctag value %q missing v1- prefix", value)
	}
}

func TestCalendarPropFindAdvertisesCalendarHomeSet(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop><C:calendar-home-set/><C:calendar-user-address-set/><D:current-user-principal/></D:prop>
</D:propfind>`
	request := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("Depth", "0")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", response.Code, response.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, response.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	homeSetName := xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "calendar-home-set"}
	homeSetValue, ok := results[0].OK[homeSetName]
	if !ok {
		t.Fatalf("calendar-home-set missing from 200 OK propstat: %#v", results[0])
	}
	if !strings.Contains(homeSetValue, calendarHomeSetPath) {
		t.Fatalf("calendar-home-set href %q missing %s", homeSetValue, calendarHomeSetPath)
	}
	addressSetName := xml.Name{Space: "urn:ietf:params:xml:ns:caldav", Local: "calendar-user-address-set"}
	addressSetValue, ok := results[0].OK[addressSetName]
	if !ok {
		t.Fatalf("calendar-user-address-set missing from 200 OK propstat: %#v", results[0])
	}
	if !strings.Contains(addressSetValue, calendarPrincipalPath) {
		t.Fatalf("calendar-user-address-set href %q missing %s", addressSetValue, calendarPrincipalPath)
	}
	principalName := xml.Name{Space: "DAV:", Local: "current-user-principal"}
	if _, ok := results[0].OK[principalName]; !ok {
		t.Fatalf("current-user-principal missing from 200 OK propstat: %#v", results[0])
	}
}

func TestCalendarPropFindReportsUnknownPropertyAs404(t *testing.T) {
	service := newCalendarTestService(t)
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:X="http://example.org/custom/">
  <D:prop><D:displayname/><X:made-up-property/></D:prop>
</D:propfind>`
	request := httptest.NewRequest("PROPFIND", calendarCollectionPath, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/xml")
	request.Header.Set("Depth", "0")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusMultiStatus {
		t.Fatalf("propfind status = %d body = %s", response.Code, response.Body.String())
	}
	results := parseCalendarMultistatusOrFatal(t, response.Body.Bytes())
	if len(results) != 1 {
		t.Fatalf("expected 1 response entry, got %d", len(results))
	}
	knownName := xml.Name{Space: "DAV:", Local: "displayname"}
	if _, ok := results[0].OK[knownName]; !ok {
		t.Fatalf("displayname missing from 200 OK propstat: %#v", results[0])
	}
	unknownName := xml.Name{Space: "http://example.org/custom/", Local: "made-up-property"}
	if !containsName(results[0].NotFound, unknownName) {
		t.Fatalf("unknown property missing from 404 propstat: %#v", results[0])
	}
	if _, ok := results[0].OK[unknownName]; ok {
		t.Fatalf("unknown property must not appear in 200 propstat: %#v", results[0])
	}
}

type calendarPropFindResult struct {
	Href     string
	OK       map[xml.Name]string
	NotFound []xml.Name
}

func parseCalendarMultistatusOrFatal(t *testing.T, body []byte) []calendarPropFindResult {
	t.Helper()
	results, errorValue := parseCalendarMultistatus(body)
	if errorValue != nil {
		t.Fatalf("parse multistatus failed: %v\nbody=%s", errorValue, string(body))
	}
	return results
}

func parseCalendarMultistatus(body []byte) ([]calendarPropFindResult, error) {
	type rawPropertyXML struct {
		XMLName  xml.Name
		InnerXML string `xml:",innerxml"`
	}
	type propXML struct {
		Properties []rawPropertyXML `xml:",any"`
	}
	type propstatXML struct {
		Prop   propXML `xml:"DAV: prop"`
		Status string  `xml:"DAV: status"`
	}
	type responseXML struct {
		Href      string        `xml:"DAV: href"`
		Propstats []propstatXML `xml:"DAV: propstat"`
	}
	type multistatusXML struct {
		XMLName   xml.Name      `xml:"DAV: multistatus"`
		Responses []responseXML `xml:"DAV: response"`
	}
	var document multistatusXML
	if errorValue := xml.Unmarshal(body, &document); errorValue != nil {
		return nil, errorValue
	}
	results := make([]calendarPropFindResult, 0, len(document.Responses))
	for _, response := range document.Responses {
		entry := calendarPropFindResult{
			Href: strings.TrimSpace(response.Href),
			OK:   map[xml.Name]string{},
		}
		for _, propstat := range response.Propstats {
			isOK, isNotFound := false, false
			if statusFields := strings.Fields(propstat.Status); len(statusFields) >= 2 {
				switch statusFields[1] {
				case "200":
					isOK = true
				case "404":
					isNotFound = true
				}
			}
			for _, property := range propstat.Prop.Properties {
				switch {
				case isOK:
					entry.OK[property.XMLName] = strings.TrimSpace(property.InnerXML)
				case isNotFound:
					entry.NotFound = append(entry.NotFound, property.XMLName)
				}
			}
		}
		results = append(results, entry)
	}
	return results, nil
}

func containsName(names []xml.Name, target xml.Name) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}

func TestCalendarCTagChangesAfterEventWrite(t *testing.T) {
	service := newCalendarTestService(t)
	ctagBefore, errorValue := service.computeCalendarCTag(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	backend := calendarDAVBackend{service: service}
	cal := newCalendarDocumentWithEvent("ctag-change@example.com", "Trigger")
	if _, errorValue := backend.PutCalendarObject(context.Background(), calendarCollectionPath+"ctag-change.ics", cal, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	ctagAfter, errorValue := service.computeCalendarCTag(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if ctagBefore == ctagAfter {
		t.Fatalf("ctag did not change after event write: before=%s after=%s", ctagBefore, ctagAfter)
	}
}

func TestCalendarCTagReflectsActiveEventsOnly(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}
	ctx := context.Background()

	persistent := newCalendarDocumentWithEvent("ctag-active@example.com", "Persistent")
	if _, errorValue := backend.PutCalendarObject(ctx, calendarCollectionPath+"ctag-active.ics", persistent, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	ctagWithOne, errorValue := service.computeCalendarCTag(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	transient := newCalendarDocumentWithEvent("ctag-transient@example.com", "Transient")
	if _, errorValue := backend.PutCalendarObject(ctx, calendarCollectionPath+"ctag-transient.ics", transient, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	ctagWithTwo, errorValue := service.computeCalendarCTag(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if ctagWithOne == ctagWithTwo {
		t.Fatalf("ctag should change when an active event is added: %s", ctagWithOne)
	}

	if errorValue := backend.DeleteCalendarObject(ctx, calendarCollectionPath+"ctag-transient.ics"); errorValue != nil {
		t.Fatal(errorValue)
	}
	ctagAfterDelete, errorValue := service.computeCalendarCTag(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if ctagAfterDelete != ctagWithOne {
		t.Fatalf("ctag should reflect active events only: after-delete=%s want=%s", ctagAfterDelete, ctagWithOne)
	}
}

func TestCalendarWebUpdatePreservesCalDAVUID(t *testing.T) {
	service := newCalendarTestService(t)
	backend := calendarDAVBackend{service: service}

	const appleStyleID = "37552144-BF3D-45A1-BD41-339C97034310"
	calendarDocument := newCalendarDocument()
	caldavEvent := ical.NewEvent()
	caldavEvent.Props.SetText(ical.PropUID, appleStyleID)
	caldavEvent.Props.SetDateTime(ical.PropDateTimeStamp, time.Date(2026, 5, 14, 0, 0, 0, 0, time.UTC))
	caldavEvent.Props.SetText(ical.PropSummary, "점심1")
	caldavEvent.Props.SetDateTime(ical.PropDateTimeStart, time.Date(2026, 5, 15, 0, 0, 0, 0, time.UTC))
	caldavEvent.Props.SetDateTime(ical.PropDateTimeEnd, time.Date(2026, 5, 15, 1, 0, 0, 0, time.UTC))
	calendarDocument.Children = append(calendarDocument.Children, caldavEvent.Component)
	if _, errorValue := backend.PutCalendarObject(context.Background(), calendarCollectionPath+appleStyleID+".ics", calendarDocument, nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	storedAfterPut, found, errorValue := service.readCalendarEventByID(context.Background(), appleStyleID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup after CalDAV PUT: found=%v error=%v", found, errorValue)
	}
	if storedAfterPut.UID != appleStyleID {
		t.Fatalf("uid column after CalDAV PUT = %q want %q", storedAfterPut.UID, appleStyleID)
	}
	if !strings.Contains(storedAfterPut.RawICS, "UID:"+appleStyleID+"\r\n") {
		t.Fatalf("RawICS UID after CalDAV PUT should not have suffix; raw=%s", storedAfterPut.RawICS)
	}

	updatePayload := `{
		"title":"저녁1",
		"startISO":"2026-05-22T00:00:00Z",
		"endISO":"2026-05-22T01:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#3b82f6"
	}`
	updateRequest := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+appleStyleID, strings.NewReader(updatePayload))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	updateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}

	storedAfterUpdate, found, errorValue := service.readCalendarEventByID(context.Background(), appleStyleID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup after web update: found=%v error=%v", found, errorValue)
	}
	if storedAfterUpdate.UID != appleStyleID {
		t.Fatalf("uid column after web update = %q want %q (preserved)", storedAfterUpdate.UID, appleStyleID)
	}
	if !strings.Contains(storedAfterUpdate.RawICS, "UID:"+appleStyleID+"\r\n") {
		t.Fatalf("RawICS UID after web update should equal uid column without suffix\nraw=%s", storedAfterUpdate.RawICS)
	}
	if strings.Contains(storedAfterUpdate.RawICS, "UID:"+appleStyleID+"@internkim") {
		t.Fatalf("RawICS UID after web update must not gain @internkim suffix\nraw=%s", storedAfterUpdate.RawICS)
	}

	caldavObject, errorValue := calendarObjectFromEvent(storedAfterUpdate)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	caldavUID, _ := caldavObject.Data.Events()[0].Props.Text(ical.PropUID)
	icsFeed, errorValue := buildCalendarFeed([]calendarEvent{storedAfterUpdate})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	icsUID, _ := icsFeed.Events()[0].Props.Text(ical.PropUID)
	if caldavUID != icsUID {
		t.Fatalf("CalDAV UID %q must equal ICS feed UID %q after web update", caldavUID, icsUID)
	}
}

func newCalendarDocumentWithEvent(uid string, summary string) *ical.Calendar {
	calendar := newCalendarDocument()
	event := ical.NewEvent()
	event.Props.SetText(ical.PropUID, uid)
	event.Props.SetDateTime(ical.PropDateTimeStamp, time.Date(2026, 5, 12, 0, 0, 0, 0, time.UTC))
	event.Props.SetText(ical.PropSummary, summary)
	event.Props.SetDateTime(ical.PropDateTimeStart, time.Date(2026, 5, 12, 3, 0, 0, 0, time.UTC))
	event.Props.SetDateTime(ical.PropDateTimeEnd, time.Date(2026, 5, 12, 4, 0, 0, 0, time.UTC))
	calendar.Children = append(calendar.Children, event.Component)
	return calendar
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
		return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"dongha","nickname":"샘플","email":"dongha@example.com"}]`, nil), true
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
		return jsonResponse(http.StatusOK, `[{"id":"user-1","username":"dongha","nickname":"샘플","email":"dongha@example.com"}]`, nil), nil
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

func TestCalendarEventRemoteFieldsRoundTrip(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	startTime := time.Now().UTC().Add(2 * time.Hour)
	endTime := startTime.Add(time.Hour)
	event := calendarEvent{
		ID:                "remote-roundtrip-1",
		UID:               "remote-roundtrip-1@google",
		Title:             "Remote event",
		Description:       "Pulled from Google",
		StartISO:          startTime.Format(time.RFC3339),
		EndISO:            endTime.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#10b981",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
		RemoteSource:      remoteCalendarProviderGoogle,
		RemoteETag:        `"abc-123"`,
		RemoteHref:        "/calendars/v1/example@gmail.com/events/abc.ics",
	}
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("writeCalendarEvent: %v", errorValue)
	}
	stored, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatalf("readCalendarEventByID: %v", errorValue)
	}
	if !found {
		t.Fatal("event not found after write")
	}
	if stored.RemoteSource != event.RemoteSource {
		t.Errorf("RemoteSource: got %q, want %q", stored.RemoteSource, event.RemoteSource)
	}
	if stored.RemoteETag != event.RemoteETag {
		t.Errorf("RemoteETag: got %q, want %q", stored.RemoteETag, event.RemoteETag)
	}
	if stored.RemoteHref != event.RemoteHref {
		t.Errorf("RemoteHref: got %q, want %q", stored.RemoteHref, event.RemoteHref)
	}
}

func TestCalendarEventLocalEventHasEmptyRemoteFields(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	startTime := time.Now().UTC().Add(time.Hour)
	endTime := startTime.Add(time.Hour)
	event := calendarEvent{
		ID:                "local-1",
		UID:               "local-1@internkim",
		Title:             "Local event",
		StartISO:          startTime.Format(time.RFC3339),
		EndISO:            endTime.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
	}
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("writeCalendarEvent: %v", errorValue)
	}
	stored, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatalf("readCalendarEventByID: %v", errorValue)
	}
	if !found {
		t.Fatal("event not found after write")
	}
	if stored.RemoteSource != "" || stored.RemoteETag != "" || stored.RemoteHref != "" {
		t.Errorf("expected empty remote fields, got source=%q etag=%q href=%q",
			stored.RemoteSource, stored.RemoteETag, stored.RemoteHref)
	}
}

func TestRemoteCalendarAccountCRUD(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := remoteCalendarAccount{
		ID:                  "account-google-1",
		Provider:            remoteCalendarProviderGoogle,
		AccountEmail:        "user@example.com",
		PrincipalURL:        "https://apidata.googleusercontent.com/caldav/v2/user@example.com/user",
		HomeSetURL:          "https://apidata.googleusercontent.com/caldav/v2/user@example.com/",
		DefaultCalendarURL:  "https://apidata.googleusercontent.com/caldav/v2/user@example.com/events/",
		DefaultCalendarCTag: "ctag-initial",
		TokenFilePath:       "/tmp/example.token.enc",
	}
	saved, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatalf("upsert insert: %v", errorValue)
	}
	if saved.CreatedAt == "" || saved.UpdatedAt == "" {
		t.Fatalf("expected timestamps populated, got %+v", saved)
	}

	loaded, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read: %v", errorValue)
	}
	if !found {
		t.Fatal("expected account found")
	}
	if loaded.AccountEmail != account.AccountEmail {
		t.Errorf("AccountEmail: got %q, want %q", loaded.AccountEmail, account.AccountEmail)
	}
	if loaded.DefaultCalendarCTag != "ctag-initial" {
		t.Errorf("DefaultCalendarCTag: got %q, want %q", loaded.DefaultCalendarCTag, "ctag-initial")
	}

	loaded.DefaultCalendarCTag = "ctag-updated"
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, loaded); errorValue != nil {
		t.Fatalf("upsert update: %v", errorValue)
	}
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("reload: %v", errorValue)
	}
	if reloaded.DefaultCalendarCTag != "ctag-updated" {
		t.Errorf("ctag not updated: got %q", reloaded.DefaultCalendarCTag)
	}

	if errorValue := service.deleteRemoteCalendarAccount(ctx, account.ID); errorValue != nil {
		t.Fatalf("delete: %v", errorValue)
	}
	_, found, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read after delete: %v", errorValue)
	}
	if found {
		t.Fatal("expected account not found after delete")
	}
}

func TestRemoteCalendarAccountSchemaMigratesLegacyColumns(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	database, errorValue := service.openSQLiteDatabase(ctx, service.Configuration.CalendarDatabasePath, nil)
	if errorValue != nil {
		t.Fatalf("open legacy database: %v", errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE calendar_remote_accounts (
	id TEXT PRIMARY KEY,
	provider TEXT NOT NULL,
	account_email TEXT NOT NULL,
	principal_url TEXT NOT NULL DEFAULT '',
	home_set_url TEXT NOT NULL DEFAULT '',
	default_calendar_url TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE(provider, account_email)
)`)
	if errorValue != nil {
		t.Fatalf("create legacy remote accounts: %v", errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatalf("close legacy database: %v", errorValue)
	}

	account := remoteCalendarAccount{
		ID:                  "legacy-account-google",
		Provider:            remoteCalendarProviderGoogle,
		AccountEmail:        "legacy@example.com",
		DefaultCalendarCTag: "legacy-ctag",
		TokenFilePath:       "/tmp/legacy-token.enc",
		LastAuthError:       "expired token",
		LastAuthErrorAt:     "2026-06-01T00:00:00Z",
	}
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, account); errorValue != nil {
		t.Fatalf("upsert migrated account: %v", errorValue)
	}
	loaded, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read migrated account: %v", errorValue)
	}
	if !found {
		t.Fatal("expected migrated account found")
	}
	if loaded.DefaultCalendarCTag != account.DefaultCalendarCTag {
		t.Errorf("DefaultCalendarCTag: got %q, want %q", loaded.DefaultCalendarCTag, account.DefaultCalendarCTag)
	}
	if loaded.TokenFilePath != account.TokenFilePath {
		t.Errorf("TokenFilePath: got %q, want %q", loaded.TokenFilePath, account.TokenFilePath)
	}
	if loaded.LastAuthError != account.LastAuthError {
		t.Errorf("LastAuthError: got %q, want %q", loaded.LastAuthError, account.LastAuthError)
	}
	if loaded.LastAuthErrorAt != account.LastAuthErrorAt {
		t.Errorf("LastAuthErrorAt: got %q, want %q", loaded.LastAuthErrorAt, account.LastAuthErrorAt)
	}
}

func TestUpdateCalendarEventPreservesRemoteIdentity(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()

	seeded := calendarEvent{
		ID:                "remote-id-1",
		UID:               "remote-id-1@internkim",
		Title:             "original",
		Description:       "",
		Location:          "",
		StartISO:          "2026-05-27T00:00:00Z",
		EndISO:            "2026-05-27T01:00:00Z",
		TimeZone:          "Asia/Seoul",
		IsAllDay:          false,
		Color:             "#3b82f6",
		RawICS:            "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
		RemoteSource:      remoteCalendarProviderGoogle,
		RemoteETag:        `"etag-original"`,
		RemoteHref:        "/calendars/me/events/remote-id-1.ics",
	}
	if errorValue := service.writeCalendarEventWithSource(ctx, seeded, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed: %v", errorValue)
	}

	updatePayload := `{
		"title":"edited",
		"startISO":"2026-05-27T03:00:00Z",
		"endISO":"2026-05-27T04:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#3b82f6"
	}`
	updateRequest := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+seeded.ID, strings.NewReader(updatePayload))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	updateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}

	stored, found, errorValue := service.readCalendarEventByID(ctx, seeded.ID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup after update: found=%v error=%v", found, errorValue)
	}
	if stored.Title != "edited" {
		t.Errorf("title not updated: got %q", stored.Title)
	}
	if stored.StartISO != "2026-05-27T03:00:00Z" {
		t.Errorf("startISO not updated: got %q", stored.StartISO)
	}
	if stored.RemoteSource != remoteCalendarProviderGoogle {
		t.Errorf("RemoteSource lost: got %q want %q", stored.RemoteSource, remoteCalendarProviderGoogle)
	}
	if stored.RemoteETag != `"etag-original"` {
		t.Errorf("RemoteETag lost: got %q want %q", stored.RemoteETag, `"etag-original"`)
	}
	if stored.RemoteHref != "/calendars/me/events/remote-id-1.ics" {
		t.Errorf("RemoteHref lost: got %q", stored.RemoteHref)
	}
}
