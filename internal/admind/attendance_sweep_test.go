package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func newAttendanceSweepTestService(t *testing.T, planeURL string) *Service {
	t.Helper()
	stateDirectory := t.TempDir()
	configuration := Configuration{StateDirectory: stateDirectory}
	if planeURL != "" {
		configuration.CentralPlaneAppURL = planeURL
		configuration.CentralPlaneProjectURL = planeURL
		configuration.CentralPlanePublishableKey = "publishable"
		configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")
	}
	return NewService(configuration)
}

func seedRetiredAttendanceTables(t *testing.T, databasePath string, clockCount int, pinnedLedgerRows int) {
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
		"CREATE TABLE attendance_events (id TEXT PRIMARY KEY, email TEXT NOT NULL, kind TEXT NOT NULL, occurred_at TEXT NOT NULL, location_name TEXT NOT NULL, canceled_at TEXT NOT NULL)",
		"CREATE TABLE attendance_event_overrides (id TEXT PRIMARY KEY, event_id TEXT NOT NULL, override_occurred_at TEXT NOT NULL)",
		"CREATE TABLE attendance_leave_requests (id TEXT PRIMARY KEY, employee_email TEXT NOT NULL, leave_type_id TEXT NOT NULL, balance_mode TEXT NOT NULL, status TEXT NOT NULL, start_date TEXT NOT NULL, end_date TEXT NOT NULL, total_deduction_milli_days INTEGER NOT NULL, reason TEXT NOT NULL)",
		"CREATE TABLE attendance_leave_request_events (id TEXT PRIMARY KEY)",
		"CREATE TABLE attendance_summary_cache_entries (id TEXT PRIMARY KEY)",
		"CREATE TABLE attendance_leave_operations (id TEXT PRIMARY KEY)",
		"CREATE TABLE attendance_leave_ledger_entries (id TEXT PRIMARY KEY)",
	} {
		if _, errorValue := database.ExecContext(context.Background(), statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := database.ExecContext(context.Background(),
		"INSERT INTO attendance_summary_cache_entries (id) VALUES ('cached')"); errorValue != nil {
		t.Fatal(errorValue)
	}
	for index := 0; index < clockCount; index++ {
		if _, errorValue := database.ExecContext(context.Background(),
			"INSERT INTO attendance_events (id, email, kind, occurred_at, location_name, canceled_at) VALUES (?, 'member@example.com', 'clock_in', '2026-08-03T00:00:00Z', '사무실', '')",
			"clock-"+string(rune('a'+index))); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	for index := 0; index < pinnedLedgerRows; index++ {
		if _, errorValue := database.ExecContext(context.Background(),
			"INSERT INTO attendance_leave_ledger_entries (id) VALUES (?)", "ledger-"+string(rune('a'+index))); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func seedDeviceLeavePolicy(t *testing.T, service *Service) {
	t.Helper()
	document := `{"leavePolicy":{"leaveTypes":[{"id":"annual","paid":true},{"id":"unpaid","paid":false}]}}`
	path := filepath.Join(service.Configuration.StateDirectory, "attendance-settings.json")
	if errorValue := os.WriteFile(path, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func seedDeviceLeave(t *testing.T, databasePath string, id string, leaveKind string, status string) {
	t.Helper()
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(databasePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(context.Background(),
		"INSERT INTO attendance_leave_requests (id, employee_email, leave_type_id, balance_mode, status, start_date, end_date, total_deduction_milli_days, reason) VALUES (?, 'member@example.com', ?, 'annual', ?, '2026-08-03', '2026-08-04', 2000, '개인 일정')",
		id, leaveKind, status); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func countAttendanceTablesNamed(t *testing.T, database *sql.DB, tableName string) int {
	t.Helper()
	var count int
	if errorValue := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}

func TestAttendanceSweepDropsWhatTheCompanyNowHolds(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(companyShareRecordHandler))
	defer plane.Close()
	service := newAttendanceSweepTestService(t, plane.URL)
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 0, 0)

	service.sweepTheAttendanceTheCompanyNowHolds(context.Background())

	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, tableName := range []string{"attendance_events", "attendance_leave_requests", "attendance_summary_cache_entries"} {
		if countAttendanceTablesNamed(t, database, tableName) != 0 {
			t.Fatalf("%s survived the sweep", tableName)
		}
	}
}

func TestAttendanceSweepKeepsClocksTheRecordHasNotTaken(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(companyShareRecordHandler))
	defer plane.Close()
	service := newAttendanceSweepTestService(t, plane.URL)
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 2, 0)

	service.sweepTheAttendanceTheCompanyNowHolds(context.Background())

	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countAttendanceTablesNamed(t, database, "attendance_events") != 1 {
		t.Fatal("clocks the record never took were dropped anyway")
	}
	if countAttendanceTablesNamed(t, database, "attendance_summary_cache_entries") != 0 {
		t.Fatal("a cache of a store that is going should not pin the store")
	}
}

// A device that names no company covers nothing, so an unasked record must not
// read as covering everything. Getting this backwards drops a store the record
// has never seen.
func TestAttendanceSweepKeepsEverythingWhenNoCompanyIsNamed(t *testing.T) {
	service := newAttendanceSweepTestService(t, "")
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 2, 0)

	service.sweepTheAttendanceTheCompanyNowHolds(context.Background())

	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countAttendanceTablesNamed(t, database, "attendance_events") != 1 {
		t.Fatal("a device that names no company let go of attendance nobody else holds")
	}
}

func TestAttendanceSweepKeepsAStoreTheRecordHasNoPlaceFor(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(companyShareRecordHandler))
	defer plane.Close()
	service := newAttendanceSweepTestService(t, plane.URL)
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 0, 1)

	service.sweepTheAttendanceTheCompanyNowHolds(context.Background())

	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countAttendanceTablesNamed(t, database, "attendance_events") != 1 {
		t.Fatal("a ledger with nowhere to go should keep the whole store, not part of it")
	}
}

func TestAttendanceCarryWritesEveryClockTheRecordDoesNotHold(t *testing.T) {
	carried := 0
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.URL.Path == "/api/agent/session" {
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":99999999999}`))
			return
		}
		if request.Method == http.MethodPost {
			carried++
			writer.WriteHeader(http.StatusCreated)
			return
		}
		writer.Write([]byte(`[]`))
	}))
	defer plane.Close()
	service := newAttendanceSweepTestService(t, plane.URL)
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 2, 0)

	report, errorValue := service.carryAttendanceIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Clocks != 2 || len(report.Refused) != 0 || carried != 2 {
		t.Fatalf("carry report = %#v, writes = %d", report, carried)
	}
}

func TestAttendanceCarryReportsWhatTheRecordRefused(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.URL.Path == "/api/agent/session" {
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":99999999999}`))
			return
		}
		if request.Method == http.MethodPost {
			http.Error(writer, "no", http.StatusForbidden)
			return
		}
		writer.Write([]byte(`[]`))
	}))
	defer plane.Close()
	service := newAttendanceSweepTestService(t, plane.URL)
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 1, 0)

	report, errorValue := service.carryAttendanceIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Clocks != 0 || len(report.Refused) != 1 {
		t.Fatalf("a refused row must be named, not reshaped: %#v", report)
	}
}

// The trap #1375 named for the calendar, in attendance's shape: a carried row
// is still in the local table, so a sweep that only counts rows would keep the
// store forever and the carry would never finish anything.
func TestAttendanceSweepDropsTheStoreOnceTheCarryHasFinished(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.URL.Path == "/api/agent/session" {
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":99999999999}`))
			return
		}
		if request.Method == http.MethodPost {
			writer.WriteHeader(http.StatusCreated)
			return
		}
		writer.Write([]byte(`[]`))
	}))
	defer plane.Close()
	service := newAttendanceSweepTestService(t, plane.URL)
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 2, 0)

	if _, errorValue := service.carryAttendanceIntoTheRecord(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.sweepTheAttendanceTheCompanyNowHolds(context.Background())

	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, tableName := range []string{"attendance_events", "attendance_carried_rows"} {
		if countAttendanceTablesNamed(t, database, tableName) != 0 {
			t.Fatalf("%s survived a finished carry", tableName)
		}
	}
}

func TestAttendanceCarryLandsADecidedLeaveWithItsStatus(t *testing.T) {
	written := []map[string]any{}
	signers := []string{}
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.URL.Path == "/api/agent/session" {
			var asked struct {
				ExternalID string `json:"externalID"`
			}
			json.NewDecoder(request.Body).Decode(&asked)
			signers = append(signers, asked.ExternalID)
			writer.Write([]byte(`{"memberID":"member-of-` + asked.ExternalID + `","accessToken":"token","expiresAt":99999999999}`))
			return
		}
		if request.Method == http.MethodPost {
			var row map[string]any
			json.NewDecoder(request.Body).Decode(&row)
			written = append(written, row)
			writer.WriteHeader(http.StatusCreated)
			return
		}
		writer.Write([]byte(`[]`))
	}))
	defer plane.Close()
	service := newAttendanceSweepTestService(t, plane.URL)
	service.Configuration.ClaimedAdminEmailPath = writeTestFile(t, "admin@example.com")
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 0, 0)
	seedDeviceLeavePolicy(t, service)
	seedDeviceLeave(t, service.stateDatabasePath(), "leave-approved", "annual", "approved")

	report, errorValue := service.carryAttendanceIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Leaves != 1 || len(report.Refused) != 0 {
		t.Fatalf("a decided leave must land, not be refused: %#v", report)
	}
	if len(written) != 1 || written[0]["status"] != "approved" {
		t.Fatalf("the record took the leave without its status: %#v", written)
	}
	if written[0]["member_id"] != "member-of-member@example.com" {
		t.Fatalf("the leave lost whose it was: %#v", written[0])
	}
	if written[0]["is_paid"] != true || written[0]["is_deducted"] != true {
		t.Fatalf("the leave lost what its kind meant: %#v", written[0])
	}
	if !slices.Contains(signers, "admin@example.com") {
		t.Fatalf("a decided leave is an administrator's to record, signers = %v", signers)
	}
}

func TestAttendanceSweepKeepsTheStoreWhenItCannotBeRead(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(companyShareRecordHandler))
	defer plane.Close()
	service := newAttendanceSweepTestService(t, plane.URL)
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 1, 0)
	seedDeviceLeavePolicy(t, service)

	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(service.stateDatabasePath()))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(),
		"ALTER TABLE attendance_events RENAME COLUMN occurred_at TO when_it_happened"); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	service.sweepTheAttendanceTheCompanyNowHolds(context.Background())

	swept, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer swept.Close()
	if countAttendanceTablesNamed(t, swept, "attendance_events") != 1 {
		t.Fatal("a store that could not be read was let go of anyway")
	}
}

func TestAttendanceCarryRefusesALeaveWhoseDatesCannotBeRead(t *testing.T) {
	plane := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.URL.Path == "/api/agent/session" {
			writer.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":99999999999}`))
			return
		}
		if request.Method == http.MethodPost {
			t.Error("a leave with an unreadable date must never reach the record")
			writer.WriteHeader(http.StatusCreated)
			return
		}
		writer.Write([]byte(`[]`))
	}))
	defer plane.Close()
	service := newAttendanceSweepTestService(t, plane.URL)
	service.Configuration.ClaimedAdminEmailPath = writeTestFile(t, "admin@example.com")
	seedRetiredAttendanceTables(t, service.stateDatabasePath(), 0, 0)
	seedDeviceLeavePolicy(t, service)
	seedDeviceLeave(t, service.stateDatabasePath(), "leave-broken", "annual", "approved")

	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(service.stateDatabasePath()))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(),
		"UPDATE attendance_leave_requests SET start_date = 'someday' WHERE id = 'leave-broken'"); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	report, errorValue := service.carryAttendanceIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Leaves != 0 || len(report.Refused) != 1 {
		t.Fatalf("an unreadable date must be reported, not carried: %#v", report)
	}
}
