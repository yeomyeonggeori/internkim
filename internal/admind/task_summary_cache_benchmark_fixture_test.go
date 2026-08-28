package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTaskSummaryBenchmarkService(testContext testing.TB) *Service {
	testContext.Helper()
	stateDirectory := testContext.TempDir()
	writeBenchmarkFile := func(name string, document string) string {
		path := filepath.Join(stateDirectory, name)
		if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
			testContext.Fatal(errorValue)
		}
		return path
	}
	service := NewService(Configuration{
		StateDirectory:        filepath.Join(stateDirectory, "state"),
		APIBaseURL:            "https://api.example.test",
		AdminEmailPath:        writeBenchmarkFile("admin-email", "admin@example.com"),
		ClaimedAdminEmailPath: writeBenchmarkFile("claimed-admin-email", "admin@example.com"),
		FleetIDPath:           writeBenchmarkFile("fleet-id", "device-1"),
		FleetSecretPath:       writeBenchmarkFile("fleet-secret", "secret-1"),
		TaskDatabasePath:      filepath.Join(stateDirectory, "flow.sqlite"),
	})
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "https://api.example.test/api/users?fleet_id=device-1" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"records":[{"memberID":"user-admin","email":"admin@example.com","name":"Admin","role":"admin","status":"active"},{"memberID":"user-staff","email":"staff@example.com","name":"Staff","role":"member","status":"active"},{"memberID":"user-other","email":"other@example.com","name":"Other","role":"member","status":"active"}]}`, nil), nil
		}
		if request.URL.Path == "/admin/api/policy" && request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		}
		if request.URL.Path == "/api/agent/key" {
			return jsonResponse(http.StatusNotFound, `{}`, nil), nil
		}
		testContext.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
		return nil, nil
	})}
	seedTaskSummaryBenchmarkFixture(testContext, service)
	return service
}

func seedTaskSummaryBenchmarkFixture(testContext testing.TB, service *Service) {
	testContext.Helper()
	staffID := stableTaskID("staff@example.com")
	otherID := stableTaskID("other@example.com")
	tasks := []Task{
		taskReportTestTask("requested-week", "26W28", []string{staffID}, []string{"Staff"}, "M", "완료", "2026-07-06", "2026-07-08"),
		taskReportTestTask("previous-week", "26W27", []string{otherID}, []string{"Other"}, "S", "완료", "2026-06-29", "2026-07-01"),
		taskReportTestTask("current-month", "26W30", []string{staffID}, []string{"Staff"}, "XS", "완료", "2026-07-20", "2026-07-20"),
		taskReportTestTask("previous-month", "26W24", []string{otherID}, []string{"Other"}, "XS", "완료", "2026-06-10", "2026-06-10"),
	}
	for _, task := range tasks {
		if errorValue := service.writeTask(context.Background(), task); errorValue != nil {
			testContext.Fatal(errorValue)
		}
	}
}

func taskSummaryBenchmarkMembers() []taskMember {
	return []taskMember{
		{ID: stableTaskID("admin@example.com"), Name: "Admin", Email: "admin@example.com"},
		{ID: stableTaskID("other@example.com"), Name: "Other", Email: "other@example.com"},
		{ID: stableTaskID("staff@example.com"), Name: "Staff", Email: "staff@example.com"},
	}
}

func requestTaskSummaryBenchmark(testContext testing.TB, service *Service, weekCode string) taskSummaryResponse {
	testContext.Helper()
	request := httptest.NewRequest(http.MethodGet, "/flow/api/summary?week="+weekCode, nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		testContext.Fatalf("summary status = %d body = %s", response.Code, response.Body.String())
	}
	var summary taskSummaryResponse
	if errorValue := json.NewDecoder(bytes.NewReader(response.Body.Bytes())).Decode(&summary); errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if strings.TrimSpace(summary.Source) == "" {
		testContext.Fatal("summary source was empty")
	}
	return summary
}

func TestTaskSummaryBenchmarkPreflight(t *testing.T) {
	service := newTaskSummaryBenchmarkService(t)
	response := requestTaskSummaryBenchmark(t, service, "26W28")
	if len(response.WeeklyTasks) != 1 || response.WeeklyTasks[0].ID != "requested-week" {
		t.Fatalf("weekly tasks = %+v", response.WeeklyTasks)
	}
	if response.Report.WeeklyDistanceTrend.PreviousTotal == 0 {
		t.Fatalf("weekly report = %+v", response.Report.WeeklyDistanceTrend)
	}
	if response.Report.MonthlyDistanceTrend.CurrentTotal == 0 || response.Report.MonthlyDistanceTrend.PreviousTotal == 0 {
		t.Fatalf("monthly report = %+v", response.Report.MonthlyDistanceTrend)
	}
}
