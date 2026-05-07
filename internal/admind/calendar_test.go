package admind

import (
	"context"
	"encoding/json"
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
	parsedICSURL, errorValue := url.Parse(syncDocument.ICSURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	token := strings.TrimSuffix(strings.TrimPrefix(parsedICSURL.Path, "/calendar/ics/"), ".ics")
	icsRequest := httptest.NewRequest(http.MethodGet, "/calendar/ics/"+token+".ics", nil)
	icsResponse := httptest.NewRecorder()
	service.router().ServeHTTP(icsResponse, icsRequest)
	if icsResponse.Code != http.StatusOK {
		t.Fatalf("ics status = %d body = %s", icsResponse.Code, icsResponse.Body.String())
	}
	if !strings.Contains(icsResponse.Body.String(), "SUMMARY:Design review") || !strings.Contains(icsResponse.Body.String(), "LOCATION:Studio") {
		t.Fatalf("ics body missing event: %s", icsResponse.Body.String())
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
		StateDirectory:       filepath.Join(rootPath, "state", "admin"),
		CompanionJobPath:     filepath.Join(rootPath, "state", "companion-jobs.json"),
		CalendarDatabasePath: filepath.Join(rootPath, "state", "calendar.sqlite"),
		FlowDatabasePath:     filepath.Join(rootPath, "state", "flow.sqlite"),
		AdminEmailPath:       writeTestFile(t, "admin@example.com"),
		AdminUIPath:          adminUIPath,
	})
}
