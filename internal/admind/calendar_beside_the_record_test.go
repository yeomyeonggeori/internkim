package admind

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func calendarServiceWithACompanyForTest(t *testing.T) *Service {
	t.Helper()
	service := newCalendarTestService(t)
	keyPath := filepath.Join(t.TempDir(), "central-plane-agent-key")
	if errorValue := os.WriteFile(keyPath, []byte("agent-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.Configuration.CentralPlaneAppURL = "http://127.0.0.1:1"
	service.Configuration.CentralPlaneProjectURL = "http://127.0.0.1:1"
	service.Configuration.CentralPlanePublishableKey = "publishable"
	service.Configuration.CentralPlaneAgentKeyPath = keyPath
	if !service.belongsToACompany() {
		t.Fatal("these settings name a company")
	}
	return service
}

func TestACompanyCalendarIsNotServedBesideTheRecord(t *testing.T) {
	service := calendarServiceWithACompanyForTest(t)

	for _, address := range []struct {
		name   string
		method string
		path   string
		serve  func(*Service, http.ResponseWriter, *http.Request)
	}{
		{"the subscription feed", http.MethodGet, "/calendar/ics/anything.ics", (*Service).serveCalendarICS},
		{"the dav collection", "PROPFIND", "/calendar/dav/", (*Service).serveCalendarDAV},
	} {
		t.Run(address.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			address.serve(service, recorder, httptest.NewRequest(address.method, address.path, nil))

			if recorder.Code != http.StatusGone {
				t.Fatalf("%s answered %d, and answering out of this device's own store is what it must not do", address.path, recorder.Code)
			}
			if !strings.Contains(recorder.Body.String(), "company") {
				t.Fatalf("the refusal says why, got %q", recorder.Body.String())
			}
		})
	}
}

func TestADeviceWithNoCompanyStillServesItsOwnCalendar(t *testing.T) {
	service := newCalendarTestService(t)

	recorder := httptest.NewRecorder()
	service.serveCalendarICS(recorder, httptest.NewRequest(http.MethodGet, "/calendar/ics/unknown.ics", nil))

	if recorder.Code == http.StatusGone {
		t.Fatal("a device that names no company keeps serving its own calendar")
	}
}
