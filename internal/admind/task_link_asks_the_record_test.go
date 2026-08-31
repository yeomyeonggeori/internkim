package admind

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestAnOldTaskLinkAsksTheRecordWhichTaskCarriesIt(t *testing.T) {
	const deviceTaskID = "task-1756628400001"
	const recordTaskID = "38df3c78-19d9-4b83-995c-eca2b13c44f6"
	company := startCompanyCarryingOneDeviceTask(t, deviceTaskID, recordTaskID)
	service := newRecordLinkTestService(t, company.URL)

	if linkedID := service.linkedTaskID(deviceTaskID); linkedID != recordTaskID {
		t.Fatalf("the record carries this task as %s, got %q", recordTaskID, linkedID)
	}
}

func TestATaskTheRecordNeverTookLinksToItsWeekAlone(t *testing.T) {
	company := startCompanyCarryingOneDeviceTask(t, "task-carried", "record-task")
	service := newRecordLinkTestService(t, company.URL)

	if linkedID := service.linkedTaskID("task-never-carried"); linkedID != "" {
		t.Fatalf("the record never took this task, got %q", linkedID)
	}
}

func newRecordLinkTestService(t *testing.T, companyURL string) *Service {
	t.Helper()
	service := NewService(Configuration{
		AdminEmailPath:     writeTestFile(t, "admin@example.com"),
		CentralPlaneAppURL: companyURL,
	})
	useCompanyForTest(service, companyURL)
	return service
}

func startCompanyCarryingOneDeviceTask(t *testing.T, deviceTaskID string, recordTaskID string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/agent/session" {
			responseWriter.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":4102444800}`))
			return
		}
		if request.URL.Path != "/rest/v1/task" {
			http.NotFound(responseWriter, request)
			return
		}
		query, errorValue := url.ParseQuery(request.URL.RawQuery)
		if errorValue != nil {
			t.Error(errorValue)
		}
		if !strings.Contains(query.Get("calendar"), `"externalID":"`+deviceTaskID+`"`) {
			responseWriter.Write([]byte(`[]`))
			return
		}
		responseWriter.Write([]byte(`[{"id":"` + recordTaskID + `"}]`))
	}))
	t.Cleanup(server.Close)
	return server
}
