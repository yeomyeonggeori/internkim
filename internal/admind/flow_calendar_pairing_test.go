package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFlowSizeForDurationFollowsSizeDefinitions(t *testing.T) {
	definitions := flowDefinitions{Sizes: defaultFlowSizeDefinitions()}
	for _, testCase := range []struct {
		durationHours float64
		expectedSize  string
	}{
		{0.5, "XS"},
		{1, "XS"},
		{1.5, "S"},
		{2, "S"},
		{3, "M"},
		{8, "M"},
		{9, "L"},
		{16, "L"},
		{20, "XL"},
		{32, "XL"},
		{100, "XXL"},
		{500, "XXL"},
	} {
		if size := flowSizeForDurationHours(definitions, testCase.durationHours); size != testCase.expectedSize {
			t.Fatalf("%.1fh size = %q want %q", testCase.durationHours, size, testCase.expectedSize)
		}
	}
}

func TestCalendarEventDurationCountsAllDayAsWorkingHours(t *testing.T) {
	oneDay := calendarEvent{StartISO: "2026-07-30T00:00:00Z", EndISO: "2026-07-31T00:00:00Z", IsAllDay: true}
	if hours := calendarEventDurationHours(oneDay); hours != 8 {
		t.Fatalf("one all-day event = %.1fh want 8h", hours)
	}
	threeDays := calendarEvent{StartISO: "2026-07-30T00:00:00Z", EndISO: "2026-08-02T00:00:00Z", IsAllDay: true}
	if hours := calendarEventDurationHours(threeDays); hours != 24 {
		t.Fatalf("three all-day events = %.1fh want 24h", hours)
	}
	meeting := calendarEvent{StartISO: "2026-07-30T01:00:00Z", EndISO: "2026-07-30T02:30:00Z"}
	if hours := calendarEventDurationHours(meeting); hours != 1.5 {
		t.Fatalf("meeting = %.1fh want 1.5h", hours)
	}
}

func TestCalendarPairedTaskStatusFollowsTheClock(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	past := calendarEvent{StartISO: "2026-07-29T01:00:00Z", EndISO: "2026-07-29T02:00:00Z"}
	future := calendarEvent{StartISO: "2026-07-31T01:00:00Z", EndISO: "2026-07-31T02:00:00Z"}
	if status := calendarPairedTaskStatus(past, now); status != flowStatusCompleted {
		t.Fatalf("past status = %q want %q", status, flowStatusCompleted)
	}
	if status := calendarPairedTaskStatus(future, now); status != flowStatusPlanned {
		t.Fatalf("future status = %q want %q", status, flowStatusPlanned)
	}
}

func TestCalendarEventDateKeysCoverAllDaySpans(t *testing.T) {
	startKey, endKey := calendarEventDateKeys(calendarEvent{
		StartISO: "2026-07-30T00:00:00Z",
		EndISO:   "2026-08-01T00:00:00Z",
		IsAllDay: true,
	})
	if startKey != "2026-07-30" || endKey != "2026-07-31" {
		t.Fatalf("all-day span = %s..%s want 2026-07-30..2026-07-31", startKey, endKey)
	}
	startKey, endKey = calendarEventDateKeys(calendarEvent{StartISO: "2026-07-30T01:00:00Z", EndISO: "2026-07-30T02:00:00Z"})
	if startKey != "2026-07-30" || endKey != "2026-07-30" {
		t.Fatalf("timed span = %s..%s want the same day", startKey, endKey)
	}
}

func TestCalendarEventCreatesPairedTaskAndDeletesTogether(t *testing.T) {
	service := newCalendarTestService(t)
	createRequest := httptest.NewRequest(http.MethodPost, "/calendar/api/events", strings.NewReader(`{
		"title":"제품 리뷰 회의",
		"description":"이번 주 화면 검토",
		"startISO":"2036-05-08T01:00:00Z",
		"endISO":"2036-05-08T02:30:00Z",
		"timeZone":"Asia/Seoul"
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

	pairedTask, found, errorValue := service.readFlowTaskByCalendarEventID(context.Background(), createdEvent.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("calendar event did not create a paired task")
	}
	if pairedTask.Content != "제품 리뷰 회의" || pairedTask.Size != "S" || pairedTask.Status != flowStatusPlanned {
		t.Fatalf("paired task = %#v", pairedTask)
	}
	if pairedTask.StartDate != "2036-05-08" || pairedTask.EndDate != "2036-05-08" {
		t.Fatalf("paired task dates = %s..%s", pairedTask.StartDate, pairedTask.EndDate)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/calendar/api/events/"+createdEvent.ID, strings.NewReader(`{}`))
	deleteRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	deleteResponse := httptest.NewRecorder()
	service.router().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if _, found, errorValue := service.readFlowTaskByID(context.Background(), pairedTask.ID); errorValue != nil || found {
		t.Fatalf("paired task survived the event delete (found=%v error=%v)", found, errorValue)
	}
}

func TestFlowTaskWithCalendarFlagCreatesEventAndDeletesTogether(t *testing.T) {
	service := newCalendarTestService(t)
	members := service.flowMembers(httptest.NewRequest(http.MethodGet, "/flow/api/state", nil))
	if len(members) == 0 {
		t.Skip("no flow members available in this fixture")
	}
	createRequest := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", strings.NewReader(`{
		"ownerID":"`+members[0].ID+`",
		"content":"고객 미팅 준비",
		"isCalendarEvent":true,
		"eventStartISO":"2036-05-09T01:00:00Z",
		"eventEndISO":"2036-05-09T03:00:00Z",
		"startDate":"2036-05-09",
		"endDate":"2036-05-09"
	}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	createResponse := httptest.NewRecorder()
	service.router().ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", createResponse.Code, createResponse.Body.String())
	}
	var createdTask flowTask
	if errorValue := json.Unmarshal(createResponse.Body.Bytes(), &createdTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	if createdTask.CalendarEventID == "" {
		t.Fatalf("task did not create a calendar event: %#v", createdTask)
	}
	event, found, errorValue := service.readCalendarEventByID(context.Background(), createdTask.CalendarEventID)
	if errorValue != nil || !found {
		t.Fatalf("paired event missing (found=%v error=%v)", found, errorValue)
	}
	if event.Title != "고객 미팅 준비" {
		t.Fatalf("paired event = %#v", event)
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, "/flow/api/tasks/"+createdTask.ID, nil)
	deleteRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	deleteResponse := httptest.NewRecorder()
	service.router().ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusOK && deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if _, found, errorValue := service.readCalendarEventByID(context.Background(), createdTask.CalendarEventID); errorValue != nil || found {
		t.Fatalf("paired event survived the task delete (found=%v error=%v)", found, errorValue)
	}
}
