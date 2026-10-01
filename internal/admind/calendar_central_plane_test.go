package admind

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
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

func TestACalendarWriteOnADeviceWithNoCompanyStaysOnTheDevice(t *testing.T) {
	service := &Service{}
	request := httptest.NewRequest(http.MethodPost, "/calendar/api/events", nil)

	client, _, answered, errorValue := service.centralCalendarWriter(request)
	if answered || errorValue != nil || client != nil {
		t.Fatalf("a device that names no company keeps its own calendar, got answered=%v error=%v", answered, errorValue)
	}
}

func TestACompanyCalendarWriterRefusesACallItCannotName(t *testing.T) {
	service := serviceWithACompanyForTest(t)
	request := httptest.NewRequest(http.MethodPost, "/calendar/api/events", nil)

	_, _, answered, errorValue := service.centralCalendarWriter(request)
	if !answered || !errors.Is(errorValue, errCalendarReaderUnnamed) {
		t.Fatalf("a write nobody is named on cannot land on the device instead, got answered=%v error=%v", answered, errorValue)
	}
}

// A company is four settings and a key on disk. Nothing here reaches the
// network: the refusals under test happen before anything is sent.
func serviceWithACompanyForTest(t *testing.T) *Service {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "central-plane-agent-key")
	if errorValue := os.WriteFile(keyPath, []byte("agent-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := &Service{}
	service.Configuration.CentralPlaneAppURL = "http://127.0.0.1:1"
	service.Configuration.CentralPlaneProjectURL = "http://127.0.0.1:1"
	service.Configuration.CentralPlanePublishableKey = "publishable"
	service.Configuration.CentralPlaneAgentKeyPath = keyPath
	if service.centralPlane() == nil {
		t.Fatal("these settings name a company")
	}
	return service
}

func TestAReminderShorterThanAnHourStaysAReminder(t *testing.T) {
	for _, testCase := range []struct {
		minutes int
		hours   int
	}{
		{minutes: 0, hours: 0},
		{minutes: 1, hours: 1},
		{minutes: 30, hours: 1},
		{minutes: 60, hours: 1},
		{minutes: 90, hours: 2},
		{minutes: 1440, hours: 24},
	} {
		if hours := calendarReminderLeadHours(testCase.minutes); hours != testCase.hours {
			t.Fatalf("%d minutes before became %d hours before, wanted %d", testCase.minutes, hours, testCase.hours)
		}
	}
}

func TestACompanyEventSaysWhoAskedForIt(t *testing.T) {
	events := calendarEventsOfCompanyEvents([]centralplane.Event{{
		CentralID:           "task-2",
		Title:               "주간 회의",
		StartsAt:            "2026-08-24T01:00:00+00:00",
		EndsAt:              "2026-08-24T02:00:00+00:00",
		NotifyMinutesBefore: 30,
		RequesterEmail:      "isaempeul@example.com",
		RequesterName:       "이샘플",
	}}, "Asia/Seoul")

	if len(events) != 1 {
		t.Fatalf("events = %+v", events)
	}
	event := events[0]
	if event.CreatedByEmail != "isaempeul@example.com" || event.CreatedByName != "이샘플" {
		t.Fatalf("the person who asked for the event is who it was created by, got %+v", event)
	}
	if event.ReminderLeadHours != 1 {
		t.Fatalf("a half-hour reminder came back as %d hours", event.ReminderLeadHours)
	}
}

func TestACompanyEventWithNoRequesterNamesNobody(t *testing.T) {
	events := calendarEventsOfCompanyEvents([]centralplane.Event{{
		CentralID: "task-3",
		Title:     "전사 공지",
		StartsAt:  "2026-08-24T01:00:00+00:00",
		EndsAt:    "2026-08-24T02:00:00+00:00",
	}}, "Asia/Seoul")

	if len(events) != 1 || events[0].CreatedByEmail != "" || events[0].CreatedByName != "" {
		t.Fatalf("an event nobody asked for names nobody, got %+v", events)
	}
}
