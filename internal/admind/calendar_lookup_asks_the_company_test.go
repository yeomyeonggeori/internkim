package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func TestReadingOneEventFindsTheOneTheCompanyJustListed(t *testing.T) {
	const eventID = "38df3c78-19d9-4b83-995c-eca2b13c44f6"
	service := newCalendarTestService(t)
	company := startCompanyHoldingOneEvent(t, eventID, "모나 개발자 미팅")
	useCompanyForTest(service, company.URL)

	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events/"+eventID, nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "someone@example.com")
	response := httptest.NewRecorder()

	service.getCalendarEvent(response, request, eventID)

	if response.Code != http.StatusOK {
		t.Fatalf("the company holds this event, so reading it must find it, got %d", response.Code)
	}
	var answer calendarEvent
	if errorValue := json.Unmarshal(response.Body.Bytes(), &answer); errorValue != nil {
		t.Fatal(errorValue)
	}
	if answer.ID != eventID || answer.Title != "모나 개발자 미팅" {
		t.Fatalf("answer = %+v", answer)
	}
}

func TestReadingOneEventTheCompanyDoesNotHoldStaysOnTheDevice(t *testing.T) {
	service := newCalendarTestService(t)
	company := startCompanyHoldingOneEvent(t, "held-elsewhere", "다른 일정")
	useCompanyForTest(service, company.URL)

	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events/tool-1", nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "someone@example.com")
	response := httptest.NewRecorder()

	service.getCalendarEvent(response, request, "tool-1")

	if response.Code != http.StatusNotFound {
		t.Fatalf("this device holds no such event either, got %d", response.Code)
	}
}

func useCompanyForTest(service *Service, companyURL string) {
	service.centralPlaneOnce = sync.Once{}
	service.centralPlaneOnce.Do(func() {})
	service.centralPlaneClient = centralplane.New(centralplane.Settings{
		AppURL:         companyURL,
		AgentAPIKey:    "agent-key",
		ProjectURL:     companyURL,
		PublishableKey: "publishable",
	})
}

func startCompanyHoldingOneEvent(t *testing.T, eventID string, title string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/agent/session" {
			responseWriter.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":4102444800}`))
			return
		}
		if !strings.Contains(request.URL.RawQuery, "id=eq."+eventID) {
			responseWriter.Write([]byte(`[]`))
			return
		}
		responseWriter.Write([]byte(`[{"id":"` + eventID + `","title":"` + title + `","note":"","location":"",` +
			`"starts_at":"2026-08-25T02:00:00+00:00","ends_at":"2026-08-25T03:00:00+00:00","is_whole_day":false,` +
			`"updated_at":"2026-08-25T15:45:41.994735+00:00","task_participant":[{"member":{"email":"someone@example.com"}}]}]`))
	}))
	t.Cleanup(server.Close)
	return server
}
