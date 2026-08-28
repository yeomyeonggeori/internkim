package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestTaskAPICanonicalizesCompatibleWeekCodesBeforeStorage(t *testing.T) {
	for _, weekCode := range []string{"2026-W28", "26w28"} {
		t.Run(weekCode, func(t *testing.T) {
			service := newTaskAuthorizationTestService(t)
			staffID := stableTaskID("staff@example.com")
			payload := newTaskPayload("staff@example.com", "canonical week "+weekCode, taskStatusInProgress, 0, []string{staffID})
			payload.WeekCode = weekCode

			createdTask := createTaskForTest(t, service.router(), "staff@example.com", payload)

			if createdTask.WeekCode != "26W28" {
				t.Fatalf("response week = %q, want 26W28", createdTask.WeekCode)
			}
			database, errorValue := service.openTaskDatabase(context.Background())
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			var storedWeekCode string
			if errorValue := database.QueryRowContext(context.Background(), "SELECT week_code FROM flow_tasks WHERE id = ?", createdTask.ID).Scan(&storedWeekCode); errorValue != nil {
				database.Close()
				t.Fatal(errorValue)
			}
			if errorValue := database.Close(); errorValue != nil {
				t.Fatal(errorValue)
			}
			if storedWeekCode != "26W28" {
				t.Fatalf("stored week = %q, want 26W28", storedWeekCode)
			}
			tasks, errorValue := service.readTasks(context.Background(), "26W28", nil)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(tasks) != 1 || tasks[0].WeekCode != "26W28" {
				t.Fatalf("tasks = %+v", tasks)
			}
		})
	}
}

func TestTaskAPIRejectsMalformedWeekCode(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	staffID := stableTaskID("staff@example.com")
	payload := newTaskPayload("staff@example.com", "malformed week", taskStatusInProgress, 0, []string{staffID})
	payload.WeekCode = "garbage25W52suffix"
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", bytes.NewReader(document))
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d body = %s", response.Code, http.StatusBadRequest, response.Body.String())
	}
}

func TestTaskAPIRejectsMalformedDates(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	handler := service.router()
	staffID := stableTaskID("staff@example.com")
	for _, testCase := range []struct {
		name      string
		startDate string
		endDate   string
	}{
		{name: "start date", startDate: "2026-02-30"},
		{name: "end date", endDate: "July"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			payload := newTaskPayload("staff@example.com", "malformed "+testCase.name, taskStatusInProgress, 0, []string{staffID})
			payload.StartDate = testCase.startDate
			payload.EndDate = testCase.endDate
			document, errorValue := json.Marshal(payload)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			request := httptest.NewRequest(http.MethodPost, "/flow/api/tasks", bytes.NewReader(document))
			request.RemoteAddr = "198.51.100.10:443"
			request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d body = %s", response.Code, http.StatusBadRequest, response.Body.String())
			}
		})
	}
}

func TestTaskSummarySourceKeysUsesWeekAndEndpointMonths(t *testing.T) {
	task := Task{ID: "task-1", WeekCode: "26W28", StartDate: "2026-06-30", EndDate: "2026-07-02"}
	keys := taskSummarySourceKeys(task)
	want := []taskSummarySourceKey{
		{Kind: taskSummarySourceMonth, Key: "2026-06"},
		{Kind: taskSummarySourceMonth, Key: "2026-07"},
		{Kind: taskSummarySourceWeek, Key: "26W28"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %+v, want %+v", keys, want)
	}
}

func TestTaskSummarySourceKeysSkipsMalformedLegacyFragments(t *testing.T) {
	testCases := []struct {
		name string
		task Task
		want []taskSummarySourceKey
	}{
		{
			name: "malformed week",
			task: Task{ID: "bad-week", WeekCode: "garbage25W52suffix", StartDate: "2026-07-01"},
			want: []taskSummarySourceKey{{Kind: taskSummarySourceMonth, Key: "2026-07"}},
		},
		{
			name: "malformed date",
			task: Task{ID: "bad-date", WeekCode: "26W28", StartDate: "July"},
			want: []taskSummarySourceKey{{Kind: taskSummarySourceWeek, Key: "26W28"}},
		},
		{
			name: "all malformed",
			task: Task{ID: "all-bad", WeekCode: "week", StartDate: "July", EndDate: "2026-13-01"},
			want: []taskSummarySourceKey{},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			keys := taskSummarySourceKeys(testCase.task)
			if !reflect.DeepEqual(keys, testCase.want) {
				t.Fatalf("keys = %+v, want %+v", keys, testCase.want)
			}
		})
	}
}

func TestTaskBoardMoveSummarySourceKeysIncludePreviousAndUpdatedScopes(t *testing.T) {
	previousTasks := []Task{
		{ID: "moved", WeekCode: "26W30", StartDate: "2026-07-20", EndDate: "2026-07-21", Status: taskStatusPlanned, StatusRank: 1024},
		{ID: "reranked", WeekCode: "26W31", StartDate: "2026-08-01", EndDate: "2026-08-02", Status: taskStatusInProgress, StatusRank: 1024},
	}
	updates := []Task{
		{ID: "moved", WeekCode: "26W31", StartDate: "2026-08-01", EndDate: "2026-08-02", Status: taskStatusInProgress, StatusRank: 512},
		{ID: "reranked", WeekCode: "26W31", StartDate: "2026-08-01", EndDate: "2026-08-02", Status: taskStatusInProgress, StatusRank: 2048},
	}

	keys := taskBoardMoveSummarySourceKeys(previousTasks, updates)
	want := []taskSummarySourceKey{
		{Kind: taskSummarySourceMonth, Key: "2026-07"},
		{Kind: taskSummarySourceMonth, Key: "2026-08"},
		{Kind: taskSummarySourceWeek, Key: "26W30"},
		{Kind: taskSummarySourceWeek, Key: "26W31"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %+v, want %+v", keys, want)
	}
}

func TestTaskSummaryDependencyKeysForWeek(t *testing.T) {
	weekStart := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	keys := taskSummaryDependencyKeysForWeek("26W28", weekStart)
	if keys.RequestedWeek != (taskSummarySourceKey{Kind: taskSummarySourceWeek, Key: "26W28"}) {
		t.Fatalf("requested week = %+v", keys.RequestedWeek)
	}
	if keys.PreviousWeek.Key != "26W27" || keys.CurrentMonth.Key != "2026-07" || keys.PreviousMonth.Key != "2026-06" {
		t.Fatalf("dependency keys = %+v", keys)
	}
	if keys.Definitions != taskSummaryDefinitionsSourceKey() {
		t.Fatalf("definitions = %+v", keys.Definitions)
	}
}

func TestTaskSummaryMemberFingerprintIgnoresOrderAndPresentationFields(t *testing.T) {
	left, errorValue := taskSummaryMemberFingerprint([]taskMember{
		{ID: "two", Name: "Two", Email: "two@example.com", Role: "admin"},
		{ID: "one", Name: "One", Email: "one@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	right, errorValue := taskSummaryMemberFingerprint([]taskMember{
		{ID: "one", Name: "One", Email: "changed@example.com", Role: "member"},
		{ID: "two", Name: "Two", Email: "two@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if left != right {
		t.Fatalf("fingerprints differ: %q != %q", left, right)
	}
}

func TestTaskSummaryMemberFingerprintChangesWithAlignedIdentity(t *testing.T) {
	left, errorValue := taskSummaryMemberFingerprint([]taskMember{{ID: "one", Name: "One"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	right, errorValue := taskSummaryMemberFingerprint([]taskMember{{ID: "one", Name: "Changed"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if left == right {
		t.Fatalf("fingerprints matched: %q", left)
	}
}
