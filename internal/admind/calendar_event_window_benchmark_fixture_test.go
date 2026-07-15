package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	calendarEventWindowBenchmarkEventCount        = 120
	calendarEventWindowBenchmarkHistoryEventCount = calendarEventWindowBenchmarkEventCount * 12
	calendarEventWindowBenchmarkRangeIndexName    = "calendar_events_active_end_start_benchmark_idx"
)

func newCalendarEventWindowBenchmarkService(testContext testing.TB) *Service {
	return newCalendarEventWindowBenchmarkServiceWithHistory(testContext, 0)
}

func newCalendarEventWindowHistoryBenchmarkService(testContext testing.TB) *Service {
	return newCalendarEventWindowBenchmarkServiceWithHistory(testContext, calendarEventWindowBenchmarkHistoryEventCount)
}

func newCalendarEventWindowBenchmarkServiceWithHistory(testContext testing.TB, historyEventCount int) *Service {
	testContext.Helper()
	rootPath := testContext.TempDir()
	writeBenchmarkFile := func(name string, document string) string {
		path := filepath.Join(rootPath, name)
		if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
			testContext.Fatal(errorValue)
		}
		return path
	}
	service := NewService(Configuration{
		StateDirectory:        filepath.Join(rootPath, "state"),
		CalendarDatabasePath:  filepath.Join(rootPath, "state", "calendar.sqlite"),
		FlowDatabasePath:      filepath.Join(rootPath, "state", "flow.sqlite"),
		APIBaseURL:            "https://api.intern.kim",
		AdminEmailPath:        writeBenchmarkFile("admin-email", "admin@example.com"),
		ClaimedAdminEmailPath: writeBenchmarkFile("claimed-admin-email", "admin@example.com"),
		FleetIDPath:           writeBenchmarkFile("fleet-id", "device-1"),
		FleetSecretPath:       writeBenchmarkFile("fleet-secret", "secret-1"),
		CalendarSyncDisabled:  true,
	})
	service.HTTPClient = &http.Client{Transport: calendarEventWindowBenchmarkTransport(testContext)}
	seedCalendarEventWindowBenchmarkFixture(testContext, service)
	seedCalendarEventWindowBenchmarkHistory(testContext, service, historyEventCount)
	service.Configuration.MattermostBaseURL = "http://mattermost.local"
	service.Configuration.MattermostAdminPasswordPath = writeBenchmarkFile("mattermost-admin-password", "admin-password")
	return service
}

func calendarEventWindowBenchmarkTransport(testContext testing.TB) roundTripFunc {
	testContext.Helper()
	return roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch {
		case request.Method == http.MethodGet && request.URL.String() == "https://api.intern.kim/api/users?fleet_id=device-1":
			return jsonResponse(http.StatusOK, `{"records":[{"userID":"user-admin","email":"admin@example.com","name":"Admin","role":"admin","status":"active"},{"userID":"user-staff","email":"staff@example.com","name":"Staff","role":"member","status":"active"},{"userID":"user-other","email":"other@example.com","name":"Other","role":"member","status":"active"}]}`, nil), nil
		case request.Method == http.MethodGet && request.URL.Path == "/admin/api/policy":
			return jsonResponse(http.StatusOK, `{"people":[]}`, nil), nil
		case request.Method == http.MethodPost && request.URL.String() == "http://mattermost.local/api/v4/users/login":
			return jsonResponse(http.StatusOK, `{"id":"admin"}`, http.Header{"Token": []string{"admin-token"}}), nil
		case request.Method == http.MethodGet && request.URL.String() == "http://mattermost.local/api/v4/users/email/staff@example.com":
			return jsonResponse(http.StatusOK, `{"id":"user-staff","email":"staff@example.com","username":"staff","nickname":"Staff","last_picture_update":1710000000000}`, nil), nil
		default:
			testContext.Fatalf("unexpected request %s %s", request.Method, request.URL.String())
			return nil, nil
		}
	})
}

func seedCalendarEventWindowBenchmarkFixture(testContext testing.TB, service *Service) {
	testContext.Helper()
	startTime, _ := calendarEventWindowBenchmarkRange()
	seedCalendarEventWindowBenchmarkEvents(testContext, service, "benchmark-event", startTime, calendarEventWindowBenchmarkEventCount)
}

func seedCalendarEventWindowBenchmarkHistory(testContext testing.TB, service *Service, eventCount int) {
	testContext.Helper()
	startTime, _ := calendarEventWindowBenchmarkRange()
	historyStartTime := startTime.Add(-time.Duration(eventCount) * 6 * time.Hour)
	seedCalendarEventWindowBenchmarkEvents(testContext, service, "benchmark-history-event", historyStartTime, eventCount)
}

func seedCalendarEventWindowBenchmarkEvents(testContext testing.TB, service *Service, idPrefix string, startTime time.Time, eventCount int) {
	testContext.Helper()
	participants := []calendarParticipant{
		{PersonID: stableFlowID("staff@example.com"), Name: "Staff", Email: "staff@example.com"},
		{PersonID: stableFlowID("other@example.com"), Name: "Other", Email: "other@example.com"},
	}
	for index := 0; index < eventCount; index++ {
		eventStartTime := startTime.Add(time.Duration(index) * 6 * time.Hour)
		event := calendarEvent{
			ID:                fmt.Sprintf("%s-%04d", idPrefix, index),
			UID:               fmt.Sprintf("%s-%04d@intern.kim", idPrefix, index),
			Title:             fmt.Sprintf("Benchmark event %03d", index),
			Description:       "Calendar event window benchmark",
			Location:          "Benchmark room",
			StartISO:          eventStartTime.Format(time.RFC3339),
			EndISO:            eventStartTime.Add(time.Hour).Format(time.RFC3339),
			TimeZone:          "UTC",
			Color:             "#2563eb",
			Participants:      participants,
			ReminderLeadHours: calendarDefaultReminderLeadHours,
			CreatedByEmail:    "staff@example.com",
			CreatedByName:     "Staff",
			UpdatedByEmail:    "staff@example.com",
			UpdatedByName:     "Staff",
		}
		if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
			testContext.Fatal(errorValue)
		}
	}
}

func calendarEventWindowBenchmarkRange() (time.Time, time.Time) {
	startTime := time.Date(2025, time.July, 1, 0, 0, 0, 0, time.UTC)
	return startTime, startTime.AddDate(0, 1, 0)
}

func calendarEventWindowBenchmarkQueryPlan(testContext testing.TB, service *Service) []string {
	testContext.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer database.Close()
	startTime, endTime := calendarEventWindowBenchmarkRange()
	query, arguments := calendarEventRangeQuery(startTime, endTime)
	rows, errorValue := database.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, arguments...)
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer rows.Close()
	details := []string{}
	for rows.Next() {
		var identifier int
		var parentIdentifier int
		var unused int
		var detail string
		if errorValue := rows.Scan(&identifier, &parentIdentifier, &unused, &detail); errorValue != nil {
			testContext.Fatal(errorValue)
		}
		details = append(details, detail)
	}
	if errorValue := rows.Err(); errorValue != nil {
		testContext.Fatal(errorValue)
	}
	return details
}

func createCalendarEventWindowBenchmarkRangeIndex(testContext testing.TB, service *Service) {
	testContext.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer database.Close()
	statement := "CREATE INDEX " + calendarEventWindowBenchmarkRangeIndexName + " ON calendar_events(end_at, start_at) WHERE deleted_at = ''"
	if _, errorValue := database.ExecContext(context.Background(), statement); errorValue != nil {
		testContext.Fatal(errorValue)
	}
}

func serveCalendarEventWindowBenchmark(testContext testing.TB, handler http.Handler) *httptest.ResponseRecorder {
	testContext.Helper()
	request := calendarEventWindowBenchmarkRequest()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		testContext.Fatalf("calendar event window status = %d body = %s", response.Code, response.Body.String())
	}
	return response
}

func decodeCalendarEventWindowBenchmarkResponse(testContext testing.TB, response *httptest.ResponseRecorder) calendarEventsResponse {
	testContext.Helper()
	var document calendarEventsResponse
	if errorValue := json.NewDecoder(bytes.NewReader(response.Body.Bytes())).Decode(&document); errorValue != nil {
		testContext.Fatal(errorValue)
	}
	return document
}

func calendarEventWindowBenchmarkRequest() *http.Request {
	startTime, endTime := calendarEventWindowBenchmarkRange()
	query := url.Values{}
	query.Set("startISO", startTime.Format(time.RFC3339))
	query.Set("endISO", endTime.Format(time.RFC3339))
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events?"+query.Encode(), nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	return request
}

func resetCalendarActorProfileBenchmarkCache(service *Service) {
	service.calendarActorCacheMutex.Lock()
	defer service.calendarActorCacheMutex.Unlock()
	service.calendarActorCache = map[string]calendarActorProfileCacheEntry{}
}

func verifyCalendarEventWindowBenchmarkResponse(testContext testing.TB, response calendarEventsResponse) {
	testContext.Helper()
	if len(response.Events) != calendarEventWindowBenchmarkEventCount {
		testContext.Fatalf("calendar event count = %d, want %d", len(response.Events), calendarEventWindowBenchmarkEventCount)
	}
	firstEvent := response.Events[0]
	if len(firstEvent.Participants) != 2 || strings.TrimSpace(firstEvent.Participants[0].Image) == "" {
		testContext.Fatalf("calendar participants = %+v", firstEvent.Participants)
	}
	if strings.TrimSpace(firstEvent.CreatedByImage) == "" {
		testContext.Fatalf("calendar actor image = %q", firstEvent.CreatedByImage)
	}
}
