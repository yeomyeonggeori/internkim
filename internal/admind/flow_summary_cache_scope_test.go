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

func TestFlowTaskAPICanonicalizesCompatibleWeekCodesBeforeStorage(t *testing.T) {
	for _, weekCode := range []string{"2026-W28", "26w28"} {
		t.Run(weekCode, func(t *testing.T) {
			service := newFlowAuthorizationTestService(t)
			staffID := stableFlowID("staff@example.com")
			payload := newFlowTaskPayload("staff@example.com", "canonical week "+weekCode, flowStatusInProgress, 0, []string{staffID})
			payload.WeekCode = weekCode

			createdTask := createFlowTaskForTest(t, service.router(), "staff@example.com", payload)

			if createdTask.WeekCode != "26W28" {
				t.Fatalf("response week = %q, want 26W28", createdTask.WeekCode)
			}
			database, errorValue := service.openFlowDatabase(context.Background())
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
			tasks, errorValue := service.readFlowTasks(context.Background(), "26W28", nil)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(tasks) != 1 || tasks[0].WeekCode != "26W28" {
				t.Fatalf("tasks = %+v", tasks)
			}
		})
	}
}

func TestFlowTaskAPIRejectsMalformedWeekCode(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	payload := newFlowTaskPayload("staff@example.com", "malformed week", flowStatusInProgress, 0, []string{staffID})
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

func TestFlowTaskAPIRejectsMalformedDates(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	handler := service.router()
	staffID := stableFlowID("staff@example.com")
	for _, testCase := range []struct {
		name      string
		startDate string
		endDate   string
	}{
		{name: "start date", startDate: "2026-02-30"},
		{name: "end date", endDate: "July"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			payload := newFlowTaskPayload("staff@example.com", "malformed "+testCase.name, flowStatusInProgress, 0, []string{staffID})
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

func TestFlowTaskSummarySourceKeysUsesWeekAndEndpointMonths(t *testing.T) {
	task := flowTask{ID: "task-1", WeekCode: "26W28", StartDate: "2026-06-30", EndDate: "2026-07-02"}
	keys, errorValue := flowTaskSummarySourceKeys(task)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	want := []flowSummarySourceKey{
		{Kind: flowSummarySourceMonth, Key: "2026-06"},
		{Kind: flowSummarySourceMonth, Key: "2026-07"},
		{Kind: flowSummarySourceWeek, Key: "26W28"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %+v, want %+v", keys, want)
	}
}

func TestFlowTaskSummarySourceKeysSkipsMalformedLegacyFragments(t *testing.T) {
	testCases := []struct {
		name string
		task flowTask
		want []flowSummarySourceKey
	}{
		{
			name: "malformed week",
			task: flowTask{ID: "bad-week", WeekCode: "garbage25W52suffix", StartDate: "2026-07-01"},
			want: []flowSummarySourceKey{{Kind: flowSummarySourceMonth, Key: "2026-07"}},
		},
		{
			name: "malformed date",
			task: flowTask{ID: "bad-date", WeekCode: "26W28", StartDate: "July"},
			want: []flowSummarySourceKey{{Kind: flowSummarySourceWeek, Key: "26W28"}},
		},
		{
			name: "all malformed",
			task: flowTask{ID: "all-bad", WeekCode: "week", StartDate: "July", EndDate: "2026-13-01"},
			want: []flowSummarySourceKey{},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			keys, errorValue := flowTaskSummarySourceKeys(testCase.task)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if !reflect.DeepEqual(keys, testCase.want) {
				t.Fatalf("keys = %+v, want %+v", keys, testCase.want)
			}
		})
	}
}

func TestFlowTaskBoardMoveSummarySourceKeysIncludePreviousAndUpdatedScopes(t *testing.T) {
	previousTasks := []flowTask{
		{ID: "moved", WeekCode: "26W30", StartDate: "2026-07-20", EndDate: "2026-07-21", Status: flowStatusPlanned, StatusRank: 1024},
		{ID: "reranked", WeekCode: "26W31", StartDate: "2026-08-01", EndDate: "2026-08-02", Status: flowStatusInProgress, StatusRank: 1024},
	}
	updates := []flowTask{
		{ID: "moved", WeekCode: "26W31", StartDate: "2026-08-01", EndDate: "2026-08-02", Status: flowStatusInProgress, StatusRank: 512},
		{ID: "reranked", WeekCode: "26W31", StartDate: "2026-08-01", EndDate: "2026-08-02", Status: flowStatusInProgress, StatusRank: 2048},
	}

	keys, errorValue := flowTaskBoardMoveSummarySourceKeys(previousTasks, updates)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	want := []flowSummarySourceKey{
		{Kind: flowSummarySourceMonth, Key: "2026-07"},
		{Kind: flowSummarySourceMonth, Key: "2026-08"},
		{Kind: flowSummarySourceWeek, Key: "26W30"},
		{Kind: flowSummarySourceWeek, Key: "26W31"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %+v, want %+v", keys, want)
	}
}

func TestFlowSummaryDependencyKeysForWeek(t *testing.T) {
	weekStart := time.Date(2026, 7, 6, 0, 0, 0, 0, time.UTC)
	keys := flowSummaryDependencyKeysForWeek("26W28", weekStart)
	if keys.RequestedWeek != (flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W28"}) {
		t.Fatalf("requested week = %+v", keys.RequestedWeek)
	}
	if keys.PreviousWeek.Key != "26W27" || keys.CurrentMonth.Key != "2026-07" || keys.PreviousMonth.Key != "2026-06" {
		t.Fatalf("dependency keys = %+v", keys)
	}
	if keys.Definitions != flowSummaryDefinitionsSourceKey() {
		t.Fatalf("definitions = %+v", keys.Definitions)
	}
}

func TestFlowSummaryMemberFingerprintIgnoresOrderAndPresentationFields(t *testing.T) {
	left, errorValue := flowSummaryMemberFingerprint([]flowMember{
		{ID: "two", Name: "Two", Email: "two@example.com", Role: "admin"},
		{ID: "one", Name: "One", Email: "one@example.com"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	right, errorValue := flowSummaryMemberFingerprint([]flowMember{
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

func TestFlowSummaryMemberFingerprintChangesWithAlignedIdentity(t *testing.T) {
	left, errorValue := flowSummaryMemberFingerprint([]flowMember{{ID: "one", Name: "One"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	right, errorValue := flowSummaryMemberFingerprint([]flowMember{{ID: "one", Name: "Changed"}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if left == right {
		t.Fatalf("fingerprints matched: %q", left)
	}
}
