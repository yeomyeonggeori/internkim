package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newCompanyCalendarTestService(t *testing.T) *Service {
	t.Helper()
	service := newCalendarTestService(t)
	company := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/agent/session" {
			responseWriter.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":4102444800}`))
			return
		}
		responseWriter.Write([]byte(`[]`))
	}))
	t.Cleanup(company.Close)
	useCompanyForTest(service, company.URL)
	return service
}

func TestACompanyDeviceMintsNoLocalTaskForACalendarEvent(t *testing.T) {
	service := newCompanyCalendarTestService(t)
	request := httptest.NewRequest(http.MethodPost, "/calendar/api/events", nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "someone@example.com")
	event := calendarEvent{
		ID:       "5f0d0f7e-2b1c-4d5e-8f9a-1b2c3d4e5f60",
		Title:    "회사 미팅",
		StartISO: "2036-05-08T01:00:00Z",
		EndISO:   "2036-05-08T02:00:00Z",
	}

	if service.createPairedTaskForCalendarEvent(request, event) {
		t.Fatal("a company device paired a local task onto an event the company holds")
	}
	if _, found, errorValue := service.readTaskByCalendarEventID(context.Background(), event.ID); errorValue != nil || found {
		t.Fatalf("the local store took a paired task on a company device (found=%v error=%v)", found, errorValue)
	}
}

func TestACompanyDeviceLeavesTheLocalStoreAloneWhenAnEventIsDeleted(t *testing.T) {
	service := newCompanyCalendarTestService(t)
	ctx := context.Background()
	leftover := taskSummaryInvalidationTask("leftover-paired-task", "26W28", "2026-07-06", "2026-07-07", taskStatusPlanned, 1024)
	leftover.CalendarEventID = "event-paired-before-the-company"
	if errorValue := service.writeTask(ctx, leftover); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.deletePairedTaskForCalendarEvent(ctx, leftover.CalendarEventID)

	if _, found, errorValue := service.readTaskByID(ctx, leftover.ID); errorValue != nil || !found {
		t.Fatalf("a company device reached into the local task store through pairing (found=%v error=%v)", found, errorValue)
	}
}

func TestACompanyDeviceMintsNoLocalEventForATaskWithTheCalendarFlag(t *testing.T) {
	service := newCompanyCalendarTestService(t)
	request := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "someone@example.com")
	task := Task{Content: "고객 미팅 준비", StartDate: "2036-05-09", EndDate: "2036-05-09"}
	payload := taskWriteRequest{
		IsCalendarEvent: true,
		EventStartISO:   "2036-05-09T01:00:00Z",
		EventEndISO:     "2036-05-09T03:00:00Z",
	}

	if eventID := service.createPairedCalendarEventForTask(request, task, payload); eventID != "" {
		t.Fatalf("a company device paired a local event onto a task the company holds: %q", eventID)
	}
}

func TestACompanyDeviceLeavesTheLocalStoreAloneWhenATaskIsDeleted(t *testing.T) {
	service := newCompanyCalendarTestService(t)
	ctx := context.Background()
	seeded, errorValue := service.normalizeCalendarEventWriteRequest(
		httptest.NewRequest(http.MethodPost, "/calendar/api/events", nil),
		calendarEventWriteRequest{
			Title:    "회사 이전의 로컬 일정",
			StartISO: "2036-05-10T01:00:00Z",
			EndISO:   "2036-05-10T02:00:00Z",
		}, "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeCalendarEvent(ctx, seeded); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.deletePairedCalendarEventForTask(ctx, Task{ID: "any-task", CalendarEventID: seeded.ID})

	if _, found, errorValue := service.readCalendarEventByID(ctx, seeded.ID); errorValue != nil || !found {
		t.Fatalf("a company device reached into the local calendar store through pairing (found=%v error=%v)", found, errorValue)
	}
}
