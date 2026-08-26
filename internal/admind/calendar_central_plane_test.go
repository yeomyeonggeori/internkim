package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func TestACompanyEventBecomesACalendarEvent(t *testing.T) {
	events := calendarEventsOfCompanyEvents([]centralplane.Event{{
		CentralID:        "task-1",
		Title:            "포틀랜드 출장",
		Note:             "",
		Location:         "Portland",
		StartsAt:         "2026-08-24T00:00:00+00:00",
		EndsAt:           "2026-08-28T00:00:00+00:00",
		IsWholeDay:       true,
		UpdatedAt:        "2026-08-24T10:00:00+00:00",
		ParticipantMails: []string{"kimyesi@example.com"},
	}}, "Asia/Seoul")

	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	event := events[0]
	if event.ID != "task-1" || event.Title != "포틀랜드 출장" || event.Location != "Portland" {
		t.Fatalf("event = %+v", event)
	}
	if !event.IsAllDay || event.TimeZone != "Asia/Seoul" {
		t.Fatalf("event = %+v", event)
	}
	if len(event.Participants) != 1 || event.Participants[0].Email != "kimyesi@example.com" {
		t.Fatalf("the people on a company event come back as its participants, got %+v", event.Participants)
	}
}

func TestACalendarRequestWithoutARequesterStaysOnTheDevice(t *testing.T) {
	service := &Service{}
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events", nil)

	if _, answered := service.centralCalendarEvents(request, time.Time{}, time.Time{}); answered {
		t.Fatal("the company answers as the person who asked, so a request naming nobody cannot ask it")
	}
}

func TestAnOpenCalendarWindowAsksForNoBound(t *testing.T) {
	if bound := calendarWindowBound(time.Time{}); bound != "" {
		t.Fatalf("an absent bound is not a moment, got %q", bound)
	}
	moment := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	if bound := calendarWindowBound(moment); bound != "2026-08-23T00:00:00Z" {
		t.Fatalf("bound = %q", bound)
	}
}
