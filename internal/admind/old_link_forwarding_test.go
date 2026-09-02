package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func forwardingService(appURL string) *Service {
	service := &Service{}
	service.Configuration.CentralPlaneAppURL = appURL
	return service
}

func TestAnOldLinkIsSentOnCarryingTheRecordsIdentifier(t *testing.T) {
	service := forwardingService("https://example.test")
	request := httptest.NewRequest(http.MethodGet, "/calendar/?date=2026-09-22&event=device-one", nil)

	forwarded, canForward := service.forwardedToTheCompany(request, "event", func(string) string { return "record-one" })

	if !canForward {
		t.Fatal("a link nobody can follow was left where it was")
	}
	if forwarded != "https://example.test/calendar/?date=2026-09-22&event=record-one" {
		t.Errorf("forwarded to %q", forwarded)
	}
}

// The record has not taken every event this device holds. A link that names one
// of those would open nothing at all, where the day it is on is still useful.
func TestAnIdentifierTheRecordDoesNotKnowIsDropped(t *testing.T) {
	service := forwardingService("https://example.test")
	request := httptest.NewRequest(http.MethodGet, "/calendar/?date=2026-09-22&event=device-one", nil)

	forwarded, _ := service.forwardedToTheCompany(request, "event", func(string) string { return "" })

	if forwarded != "https://example.test/calendar/?date=2026-09-22" {
		t.Errorf("forwarded to %q", forwarded)
	}
}

func TestADeviceWithNoCompanyKeepsServingItsOwnPages(t *testing.T) {
	service := forwardingService("")
	request := httptest.NewRequest(http.MethodGet, "/calendar/?event=device-one", nil)

	if _, canForward := service.forwardedToTheCompany(request, "event", func(string) string { return "record-one" }); canForward {
		t.Error("a device that belongs to no company sent its reader somewhere that does not exist")
	}
}
