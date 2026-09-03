package admind

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCalendarWriteKeepsAReminderNamedInMinutes(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodPost, "/calendar/api/events", strings.NewReader(`{
		"title":"짧은 회의",
		"startISO":"2036-05-08T01:00:00Z",
		"endISO":"2036-05-08T01:30:00Z",
		"timeZone":"Asia/Seoul",
		"notifyMinutesBefore":30
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")

	event, _, errorValue := service.decodeCalendarEventWriteRequest(request, "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if forwarded := calendarReminderMinutes(event); forwarded != 30 {
		t.Fatalf("the company would be told %d minutes, want 30", forwarded)
	}
	if event.ReminderLeadHours != 1 {
		t.Fatalf("reminder lead = %d hours, want the 1 hour a half hour rounds to", event.ReminderLeadHours)
	}
}

func TestCalendarReminderMinutesFallBackToTheHoursAnEventKeeps(t *testing.T) {
	fromMinutes := calendarReminderMinutes(calendarEvent{ReminderMinutes: 30, ReminderLeadHours: 1})
	if fromMinutes != 30 {
		t.Fatalf("minutes = %d, want the 30 the caller named", fromMinutes)
	}
	fromHours := calendarReminderMinutes(calendarEvent{ReminderLeadHours: 2})
	if fromHours != 120 {
		t.Fatalf("minutes = %d, want 120", fromHours)
	}
}
