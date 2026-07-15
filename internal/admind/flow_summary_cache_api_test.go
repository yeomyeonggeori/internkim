package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
