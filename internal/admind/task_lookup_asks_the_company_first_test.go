package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

const companyHeldTaskID = "6f0f5f4e-3a52-4a3f-9b3f-2f7a9b1c0d21"

func TestReadingOneTaskAnswersTheCompanyOverTheDeviceCopy(t *testing.T) {
	service := NewService(Configuration{TaskDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	company := startCompanyHoldingOneTask(t, companyHeldTaskID, "completed")
	useCompanyForTest(service, company.URL)
	ctx := withTaskActor(context.Background(), "someone@example.com")

	staleCopy := taskSummaryInvalidationTask(companyHeldTaskID, "26W28", "2026-07-06", "2026-07-07", taskStatusPlanned, 1024)
	if errorValue := service.writeTask(ctx, staleCopy); errorValue != nil {
		t.Fatal(errorValue)
	}

	task, found, errorValue := service.readTaskAnswering(ctx, companyHeldTaskID, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found || task.Status != "completed" {
		t.Fatalf("the company's row must answer over the device copy, got found=%v status=%q", found, task.Status)
	}
}

func TestReadingOneTaskTheCompanyDoesNotHoldAnswersNothing(t *testing.T) {
	service := NewService(Configuration{TaskDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	company := startCompanyHoldingOneTask(t, companyHeldTaskID, "completed")
	useCompanyForTest(service, company.URL)
	ctx := withTaskActor(context.Background(), "someone@example.com")

	deviceOnly := taskSummaryInvalidationTask("device-only-task", "26W28", "2026-07-06", "2026-07-07", taskStatusPlanned, 1024)
	if errorValue := service.writeTask(ctx, deviceOnly); errorValue != nil {
		t.Fatal(errorValue)
	}

	_, found, errorValue := service.readTaskAnswering(ctx, "device-only-task", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found {
		t.Fatal("a company device answers tasks from the company board alone")
	}
}

func startCompanyHoldingOneTask(t *testing.T, taskID string, status string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		if request.URL.Path == "/api/agent/session" {
			responseWriter.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":4102444800}`))
			return
		}
		if !strings.HasPrefix(request.URL.Path, "/rest/v1/task") {
			responseWriter.Write([]byte(`[]`))
			return
		}
		responseWriter.Write([]byte(`[{"id":"` + taskID + `","title":"회사 업무","status":"` + status + `",` +
			`"business":"개발","type":"회의","size":"XS","starts_at":"2026-07-06T00:00:00+00:00",` +
			`"ends_at":"2026-07-07T00:00:00+00:00","created_at":"2026-07-06T01:00:00+00:00",` +
			`"updated_at":"2026-07-06T02:00:00+00:00","requester":{"email":"someone@example.com"},` +
			`"task_participant":[{"member":{"email":"someone@example.com"}}]}]`))
	}))
	t.Cleanup(server.Close)
	return server
}
