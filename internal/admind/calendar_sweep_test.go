package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCalendarSweepDropsWhatTheCompanyNowHolds(t *testing.T) {
	service := newCalendarTestService(t)
	seedRetiredCalendarTables(t, service.stateDatabasePath(), 0)

	service.sweepTheCalendarTheCompanyNowHolds(context.Background())

	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, tableName := range []string{"calendar_events", "calendar_delete_intents", "calendar_settings"} {
		if countCalendarTablesNamed(t, database, tableName) != 0 {
			t.Fatalf("%s survived the sweep", tableName)
		}
	}
	if countCalendarTablesNamed(t, database, "calendar_holidays") != 0 {
		t.Fatal("the company answers holidays now, so this device keeps no cache of them")
	}
}

func TestCalendarSweepKeepsAStoreTheRecordHasNotTaken(t *testing.T) {
	service := newCalendarTestService(t)
	seedRetiredCalendarTables(t, service.stateDatabasePath(), 2)

	service.sweepTheCalendarTheCompanyNowHolds(context.Background())

	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, tableName := range []string{"calendar_events", "calendar_event_participants", "calendar_settings"} {
		if countCalendarTablesNamed(t, database, tableName) != 1 {
			t.Fatalf("%s was dropped while the record did not hold its events", tableName)
		}
	}
	var keptEvents int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_events").Scan(&keptEvents); errorValue != nil {
		t.Fatal(errorValue)
	}
	if keptEvents != 2 {
		t.Fatalf("kept events = %d", keptEvents)
	}
}

func TestOrphanedSyncTablesGoWhateverTheRecordHolds(t *testing.T) {
	service := newCalendarTestService(t)
	seedRetiredCalendarTables(t, service.stateDatabasePath(), 2)

	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCalendarTablesNamed(t, database, "calendar_remote_accounts") != 0 {
		t.Fatal("a sync table nothing has written since #1347 was kept")
	}
	if countCalendarTablesNamed(t, database, "calendar_events") != 1 {
		t.Fatal("the device's own calendar went with the sync tables")
	}
}

func seedRetiredCalendarTables(t *testing.T, databasePath string, eventCount int) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(databasePath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(databasePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, statement := range []string{
		"CREATE TABLE calendar_events (id TEXT PRIMARY KEY, title TEXT NOT NULL, start_at TEXT NOT NULL, end_at TEXT NOT NULL, is_all_day INTEGER NOT NULL, created_by_email TEXT NOT NULL, deleted_at TEXT NOT NULL)",
		"CREATE TABLE calendar_event_participants (event_id TEXT NOT NULL, person_id TEXT NOT NULL, name TEXT NOT NULL, email TEXT NOT NULL, sort_order INTEGER NOT NULL)",
		"CREATE TABLE calendar_delete_intents (event_id TEXT PRIMARY KEY)",
		"CREATE TABLE calendar_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"CREATE TABLE calendar_remote_accounts (id TEXT PRIMARY KEY)",
	} {
		if _, errorValue := database.ExecContext(context.Background(), statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	for index := 0; index < eventCount; index += 1 {
		if _, errorValue := database.ExecContext(context.Background(),
			"INSERT INTO calendar_events(id, title, start_at, end_at, is_all_day, created_by_email, deleted_at) VALUES (?, ?, ?, ?, 0, ?, '')",
			fmt.Sprintf("never-carried-%d", index), "제품 회고", "2026-08-20T01:00:00Z", "2026-08-20T02:00:00Z", "admin@example.com"); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func countCalendarTablesNamed(t *testing.T, database *sql.DB, tableName string) int {
	t.Helper()
	var found int
	errorValue := database.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&found)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return found
}

func TestCarryingTheCalendarIntoTheRecordLetsTheSweepDropIt(t *testing.T) {
	savedTitles := []string{}
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/api/agent/session") {
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"member-token","expiresAt":4102444800}`))
			return
		}
		if strings.HasPrefix(request.URL.Path, "/rest/v1/rpc/") {
			var arguments map[string]any
			json.NewDecoder(request.Body).Decode(&arguments)
			title, _ := arguments["target_title"].(string)
			savedTitles = append(savedTitles, title)
			writer.Write([]byte(`"record-` + fmt.Sprint(len(savedTitles)) + `"`))
			return
		}
		writer.Write([]byte(`[]`))
	}))
	defer plane.Close()

	service := newCalendarTestService(t)
	seedRetiredCalendarTables(t, service.stateDatabasePath(), 2)
	attachCompanyForCalendarCarryTest(t, service, plane.URL)

	uncovered, errorValue := service.calendarEventsTheRecordDoesNotHold(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if uncovered != 2 {
		t.Fatalf("uncovered before the carry = %d", uncovered)
	}

	coverage, errorValue := service.calendarCoverageOfTheRecord(calendarCarryRequestForTest())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if coverage.Carried != 2 || len(coverage.Refused) != 0 {
		t.Fatalf("carried = %d, refused = %v", coverage.Carried, coverage.Refused)
	}
	if len(savedTitles) != 2 {
		t.Fatalf("the record was asked to keep %v", savedTitles)
	}

	uncovered, errorValue = service.calendarEventsTheRecordDoesNotHold(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if uncovered != 0 {
		t.Fatalf("uncovered after the carry = %d", uncovered)
	}

	service.sweepTheCalendarTheCompanyNowHolds(context.Background())

	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, tableName := range []string{"calendar_events", "calendar_event_participants", "calendar_carried_events"} {
		if countCalendarTablesNamed(t, database, tableName) != 0 {
			t.Fatalf("%s survived a calendar the record has taken", tableName)
		}
	}
}

func TestARefusedEventIsReportedAndKeptAsItHappened(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if strings.HasSuffix(request.URL.Path, "/api/agent/session") {
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"member-token","expiresAt":4102444800}`))
			return
		}
		if strings.HasPrefix(request.URL.Path, "/rest/v1/rpc/") {
			http.Error(writer, `{"message":"an event may not end before it starts"}`, http.StatusBadRequest)
			return
		}
		writer.Write([]byte(`[]`))
	}))
	defer plane.Close()

	service := newCalendarTestService(t)
	seedRetiredCalendarTables(t, service.stateDatabasePath(), 2)
	attachCompanyForCalendarCarryTest(t, service, plane.URL)

	coverage, errorValue := service.calendarCoverageOfTheRecord(calendarCarryRequestForTest())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if coverage.Carried != 0 || len(coverage.Refused) != 2 {
		t.Fatalf("carried = %d, refused = %v", coverage.Carried, coverage.Refused)
	}

	service.sweepTheCalendarTheCompanyNowHolds(context.Background())

	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCalendarTablesNamed(t, database, "calendar_events") != 1 {
		t.Fatal("the events the record refused were dropped instead of kept")
	}
}

func attachCompanyForCalendarCarryTest(t *testing.T, service *Service, planeURL string) {
	t.Helper()
	forgetCentralPlaneForTest(service)
	service.Configuration.CentralPlaneAppURL = planeURL
	service.Configuration.CentralPlaneProjectURL = planeURL
	service.Configuration.CentralPlanePublishableKey = "publishable"
	service.Configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")
}

func calendarCarryRequestForTest() *http.Request {
	request := httptest.NewRequest(http.MethodGet, "/agent/api/calendar-record-coverage?carry=true", nil)
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	return request
}
