package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type flowSummaryCacheAPIEntry struct {
	RequestedWeekRevision int64
	DefinitionsRevision   int64
	Payload               string
	StoredAt              string
}

func TestFlowSummaryAPIRejectsInvalidWeekWithoutCacheEntry(t *testing.T) {
	for _, weekCode := range []string{"not-a-week", "garbage25W52suffix", "25W53"} {
		t.Run(weekCode, func(t *testing.T) {
			service := newFlowAuthorizationTestService(t)
			request := newFlowSummaryAPIRequest(weekCode)
			response := httptest.NewRecorder()

			service.router().ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("summary status = %d body = %s", response.Code, response.Body.String())
			}
			if count := flowSummaryCacheEntryCountForTest(t, service); count != 0 {
				t.Fatalf("cache entry count = %d, want 0", count)
			}
		})
	}
}

func TestFlowSummaryAPICanonicalizesValidWeek(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	request := newFlowSummaryAPIRequest("2026-W28")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("summary status = %d body = %s", response.Code, response.Body.String())
	}
	var summary flowSummaryResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&summary); errorValue != nil {
		t.Fatal(errorValue)
	}
	if summary.Week.Code != "26W28" {
		t.Fatalf("summary week = %q, want %q", summary.Week.Code, "26W28")
	}
}

func TestFlowSummaryAPIUsesCurrentCanonicalWeekWhenWeekIsEmpty(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	requestStartedAt := time.Now()
	request := newFlowSummaryAPIRequest("")
	response := httptest.NewRecorder()

	service.router().ServeHTTP(response, request)
	requestFinishedAt := time.Now()

	if response.Code != http.StatusOK {
		t.Fatalf("summary status = %d body = %s", response.Code, response.Body.String())
	}
	var summary flowSummaryResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&summary); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedWeekCodes := map[string]struct{}{
		weekCodeForDate(requestStartedAt):  {},
		weekCodeForDate(requestFinishedAt): {},
	}
	if _, isExpected := expectedWeekCodes[summary.Week.Code]; !isExpected {
		t.Fatalf("summary week = %q, want current canonical week", summary.Week.Code)
	}
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var storedWeekCode string
	if errorValue := database.QueryRowContext(context.Background(), "SELECT week_code FROM flow_summary_cache_entries").Scan(&storedWeekCode); errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedWeekCode != summary.Week.Code {
		t.Fatalf("stored week = %q, summary week = %q", storedWeekCode, summary.Week.Code)
	}
}

func TestFlowSummaryAPIRefreshesCachedResponseAfterSourceMutations(t *testing.T) {
	service := newFlowAuthorizationTestService(t)
	ctx := context.Background()
	staffID := stableFlowID("staff@example.com")
	task := flowTask{
		ID:               "cache-integration-task",
		WeekCode:         "26W28",
		OwnerID:          staffID,
		OwnerName:        "Staff",
		ParticipantIDs:   []string{staffID},
		ParticipantNames: []string{"Staff"},
		Business:         "Development",
		Type:             "Implementation",
		Content:          "Before mutation",
		Goal:             "Verify cache invalidation",
		Size:             "S",
		Status:           flowStatusCompleted,
		StatusRank:       1024,
		StartDate:        "2026-07-06",
		EndDate:          "2026-07-07",
	}
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	first := requestFlowSummaryAPIForTest(t, service, "26W28")
	firstEntry := readFlowSummaryCacheAPIEntry(t, service, "26W28")
	second := requestFlowSummaryAPIForTest(t, service, "26W28")
	secondEntry := readFlowSummaryCacheAPIEntry(t, service, "26W28")
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("cached response changed: first = %+v second = %+v", first, second)
	}
	if firstEntry != secondEntry {
		t.Fatalf("cache entry changed on repeated read: first = %+v second = %+v", firstEntry, secondEntry)
	}
	if count := flowSummaryCacheEntryCountForTest(t, service); count != 1 {
		t.Fatalf("cache entry count = %d, want 1", count)
	}
	task.Content = "After mutation"
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	definitions, errorValue := service.readFlowDefinitions(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for index := range definitions.Sizes {
		if definitions.Sizes[index].Name == task.Size {
			definitions.Sizes[index].DistanceKM += 10
		}
	}
	if errorValue := service.writeFlowDefinitions(ctx, definitions); errorValue != nil {
		t.Fatal(errorValue)
	}
	third := requestFlowSummaryAPIForTest(t, service, "26W28")
	thirdEntry := readFlowSummaryCacheAPIEntry(t, service, "26W28")
	if len(third.WeeklyTasks) != 1 || third.WeeklyTasks[0].Content != task.Content {
		t.Fatalf("weekly tasks = %+v", third.WeeklyTasks)
	}
	if third.Metrics.TotalDistance == first.Metrics.TotalDistance {
		t.Fatalf("total distance did not change: first = %d third = %d", first.Metrics.TotalDistance, third.Metrics.TotalDistance)
	}
	if thirdEntry.Payload == firstEntry.Payload {
		t.Fatal("cached payload did not change")
	}
	if thirdEntry.RequestedWeekRevision <= firstEntry.RequestedWeekRevision {
		t.Fatalf("requested week revision = %d, want greater than %d", thirdEntry.RequestedWeekRevision, firstEntry.RequestedWeekRevision)
	}
	if thirdEntry.DefinitionsRevision <= firstEntry.DefinitionsRevision {
		t.Fatalf("definitions revision = %d, want greater than %d", thirdEntry.DefinitionsRevision, firstEntry.DefinitionsRevision)
	}
}

func TestFlowSummaryCacheStoreRejectsNoncanonicalWeek(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		weekCode string
	}{
		{name: "empty", weekCode: ""},
		{name: "long year", weekCode: "2026-W28"},
		{name: "invalid", weekCode: "not-a-week"},
		{name: "garbage partial match", weekCode: "garbage25W52suffix"},
		{name: "nonexistent ISO week", weekCode: "25W53"},
		{name: "whitespace", weekCode: " 26W28 "},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := newFlowSummaryCacheTestService(t)
			keys := flowSummaryCacheTestKeys()
			snapshot := readFlowSummaryDependencySnapshotForTest(t, service, keys, "members-v1")

			stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(
				context.Background(),
				testCase.weekCode,
				keys,
				snapshot,
				validFlowSummaryCachePayloadForTest(),
				time.Now(),
			)

			if errorValue == nil {
				t.Fatal("noncanonical cache week was accepted")
			}
			if stored {
				t.Fatal("noncanonical cache entry was stored")
			}
			if count := flowSummaryCacheEntryCountForTest(t, service); count != 0 {
				t.Fatalf("cache entry count = %d, want 0", count)
			}
		})
	}
}

func newFlowSummaryAPIRequest(weekCode string) *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/flow/api/summary?week="+weekCode, nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	return request
}

func requestFlowSummaryAPIForTest(t *testing.T, service *Service, weekCode string) flowSummaryResponse {
	t.Helper()
	response := httptest.NewRecorder()
	service.router().ServeHTTP(response, newFlowSummaryAPIRequest(weekCode))
	if response.Code != http.StatusOK {
		t.Fatalf("summary status = %d body = %s", response.Code, response.Body.String())
	}
	var summary flowSummaryResponse
	if errorValue := json.NewDecoder(response.Body).Decode(&summary); errorValue != nil {
		t.Fatal(errorValue)
	}
	return summary
}

func readFlowSummaryCacheAPIEntry(t *testing.T, service *Service, weekCode string) flowSummaryCacheAPIEntry {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var entry flowSummaryCacheAPIEntry
	if errorValue := database.QueryRowContext(context.Background(), `
		SELECT requested_week_revision, definitions_revision, payload_json, cached_at
		FROM flow_summary_cache_entries
		WHERE week_code = ?`, weekCode).Scan(
		&entry.RequestedWeekRevision,
		&entry.DefinitionsRevision,
		&entry.Payload,
		&entry.StoredAt,
	); errorValue != nil {
		t.Fatal(errorValue)
	}
	return entry
}
