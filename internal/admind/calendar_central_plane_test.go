package admind

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestACalendarRequestOnADeviceWithNoCompanyStaysOnTheDevice(t *testing.T) {
	service := &Service{}
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events", nil)

	events, answered, errorValue := service.centralCalendarEvents(request, time.Time{}, time.Time{})
	if answered || errorValue != nil || events != nil {
		t.Fatalf("a device that names no company keeps its own calendar, got answered=%v error=%v", answered, errorValue)
	}
}

func TestACompanyCalendarRefusesACallItCannotName(t *testing.T) {
	service := serviceWithACompanyForTest(t)
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events", nil)

	_, answered, errorValue := service.centralCalendarEvents(request, time.Time{}, time.Time{})
	if !answered {
		t.Fatal("a company device answers for its calendar rather than handing back its own copy")
	}
	if !errors.Is(errorValue, errCalendarReaderUnnamed) {
		t.Fatalf("a call the company cannot attribute is refused, got %v", errorValue)
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

func TestAnOpenCalendarWindowAsksForNoBound(t *testing.T) {
	if bound := calendarWindowBound(time.Time{}); bound != "" {
		t.Fatalf("an absent bound is not a moment, got %q", bound)
	}
	moment := time.Date(2026, 8, 23, 0, 0, 0, 0, time.UTC)
	if bound := calendarWindowBound(moment); bound != "2026-08-23T00:00:00Z" {
		t.Fatalf("bound = %q", bound)
	}
}
