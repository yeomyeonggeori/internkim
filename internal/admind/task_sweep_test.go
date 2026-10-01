package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

type recordedTaskWrite struct {
	Title   string `json:"target_title"`
	Status  string `json:"target_status"`
	Starts  string `json:"target_starts_at"`
	Ends    string `json:"target_ends_at"`
	Mirrors []struct {
		Source     string `json:"source"`
		ExternalID string `json:"externalID"`
	} `json:"target_mirrors"`
}

type companyHoldingTasks struct {
	server     *httptest.Server
	carried    map[string]string
	written    []recordedTaskWrite
	businesses []string
}

// A company that answers what a carry asks of it: who a member is, which words
// its tasks may carry, whether it already carries a device row, and the write
// itself.
func startCompanyForTaskCarry(t *testing.T, alreadyCarried map[string]string) *companyHoldingTasks {
	t.Helper()
	company := &companyHoldingTasks{carried: alreadyCarried, businesses: []string{"신사업"}}
	if company.carried == nil {
		company.carried = map[string]string{}
	}
	company.server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/api/agent/session":
			responseWriter.Write([]byte(`{"memberID":"member-1","accessToken":"token","expiresAt":4102444800}`))
		case request.URL.Path == "/api/agent/member":
			responseWriter.Write([]byte(`{"member":{"memberID":"member-1"}}`))
		case request.URL.Path == "/api/agent/company":
			responseWriter.Write([]byte(`{"company":{"name":"보기","timezone":"Asia/Seoul"}}`))
		case request.URL.Path == "/api/v1/tools/task_list/invoke":
			responseWriter.Write([]byte(`{"result":{"count":0,"tasks":[],"registeredLabels":` +
				company.answerForVocabulary() + `}}`))
		case request.URL.Path == "/rest/v1/rpc/task_save":
			body, _ := io.ReadAll(request.Body)
			var written recordedTaskWrite
			json.Unmarshal(body, &written)
			company.written = append(company.written, written)
			responseWriter.Write([]byte(`"33333333-3333-4333-8333-333333333333"`))
		case strings.HasPrefix(request.URL.Path, "/rest/v1/task"):
			responseWriter.Write([]byte(company.answerForCarryLookup(request.URL.Query().Get("calendar"))))
		default:
			responseWriter.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(company.server.Close)
	return company
}

func (company *companyHoldingTasks) answerForVocabulary() string {
	labels := make([]map[string]string, 0, len(company.businesses))
	for _, name := range company.businesses {
		labels = append(labels, map[string]string{"name": name})
	}
	businesses, _ := json.Marshal(labels)
	return `{"businesses":` + string(businesses) + `,"types":[],"sizes":[],"statuses":[]}`
}

func (company *companyHoldingTasks) answerForCarryLookup(filter string) string {
	for deviceTaskID, recordID := range company.carried {
		if strings.Contains(filter, deviceTaskID) {
			return `[{"id":"` + recordID + `"}]`
		}
	}
	return `[]`
}

func newTaskSweepTestService(t *testing.T, planeURL string) *Service {
	t.Helper()
	stateDirectory := t.TempDir()
	configuration := Configuration{
		StateDirectory: stateDirectory,
		AdminEmailPath: writeTestFile(t, "member@example.com"),
	}
	if planeURL != "" {
		configuration.CentralPlaneAppURL = planeURL
		configuration.CentralPlaneProjectURL = planeURL
		configuration.CentralPlanePublishableKey = "publishable"
		configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")
	}
	service := NewService(configuration)
	service.policyRecordCache = []adminUserMutation{
		{Email: "member@example.com", Name: "이샘플"},
	}
	return service
}

func seedRetiredTaskTables(t *testing.T, databasePath string, vocabularyEntries int) *sql.DB {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(databasePath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(databasePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, statement := range []string{
		`CREATE TABLE flow_tasks (id TEXT PRIMARY KEY, participant_ids TEXT NOT NULL, business TEXT NOT NULL,
			type TEXT NOT NULL, content TEXT NOT NULL, size TEXT NOT NULL, status TEXT NOT NULL,
			start_date TEXT NOT NULL, end_date TEXT NOT NULL, created_at TEXT NOT NULL)`,
		"CREATE TABLE flow_definitions (kind TEXT NOT NULL, value TEXT NOT NULL, position INTEGER NOT NULL)",
		"CREATE TABLE flow_size_definitions (name TEXT PRIMARY KEY)",
		"CREATE TABLE flow_definition_meta (kind TEXT PRIMARY KEY, initialized INTEGER NOT NULL)",
		"CREATE TABLE flow_summary_cache_entries (week_code TEXT PRIMARY KEY)",
		"CREATE TABLE flow_channel_outbox (task_id TEXT PRIMARY KEY)",
	} {
		if _, errorValue := database.ExecContext(context.Background(), statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := database.ExecContext(context.Background(),
		"INSERT INTO flow_summary_cache_entries (week_code) VALUES ('26W29')"); errorValue != nil {
		t.Fatal(errorValue)
	}
	for index := 0; index < vocabularyEntries; index++ {
		if _, errorValue := database.ExecContext(context.Background(),
			"INSERT INTO flow_definitions (kind, value, position) VALUES ('category', ?, ?)",
			"사업"+string(rune('a'+index)), index); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func seedDeviceTask(t *testing.T, database *sql.DB, id string, status string, startDate string, endDate string) {
	t.Helper()
	if _, errorValue := database.ExecContext(context.Background(),
		`INSERT INTO flow_tasks (id, participant_ids, business, type, content, size, status, start_date, end_date, created_at)
		VALUES (?, '["`+stableTaskID("member@example.com")+`"]', '개발', '회의', '보고서 초안', 'M', ?, ?, ?, '2026-07-01T00:00:00Z')`,
		id, status, startDate, endDate); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func countTaskTablesNamed(t *testing.T, database *sql.DB, tableName string) int {
	t.Helper()
	var count int
	if errorValue := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}

func openTaskStoreForTest(t *testing.T, service *Service) *sql.DB {
	t.Helper()
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestTaskSweepDropsWhatTheCompanyNowHolds(t *testing.T) {
	service := newTaskSweepTestService(t, startCompanyForTaskCarry(t, nil).server.URL)
	seedRetiredTaskTables(t, service.stateDatabasePath(), 0)

	service.sweepTheTasksTheCompanyNowHolds(context.Background())

	database := openTaskStoreForTest(t, service)
	for _, tableName := range []string{"flow_tasks", "flow_summary_cache_entries", "flow_channel_outbox",
		"flow_definition_meta", "flow_definitions", "flow_size_definitions"} {
		if countTaskTablesNamed(t, database, tableName) != 0 {
			t.Fatalf("%s survived the sweep", tableName)
		}
	}
}

func TestTaskSweepKeepsTasksTheRecordHasNotTaken(t *testing.T) {
	service := newTaskSweepTestService(t, startCompanyForTaskCarry(t, nil).server.URL)
	store := seedRetiredTaskTables(t, service.stateDatabasePath(), 0)
	seedDeviceTask(t, store, "task-1", "planned", "2026-07-06", "2026-07-07")

	service.sweepTheTasksTheCompanyNowHolds(context.Background())

	database := openTaskStoreForTest(t, service)
	if countTaskTablesNamed(t, database, "flow_tasks") != 1 {
		t.Fatal("tasks the record never took were dropped anyway")
	}
	if countTaskTablesNamed(t, database, "flow_summary_cache_entries") != 0 {
		t.Fatal("a cache of a store that is going should not pin the store")
	}
}

// A device that names no company covers nothing, so an unasked record must not
// read as covering everything.
func TestTaskSweepKeepsEverythingWhenNoCompanyIsNamed(t *testing.T) {
	service := newTaskSweepTestService(t, "")
	store := seedRetiredTaskTables(t, service.stateDatabasePath(), 3)
	seedDeviceTask(t, store, "task-1", "planned", "2026-07-06", "2026-07-07")

	service.sweepTheTasksTheCompanyNowHolds(context.Background())

	database := openTaskStoreForTest(t, service)
	if countTaskTablesNamed(t, database, "flow_tasks") != 1 {
		t.Fatal("a device that names no company let go of tasks nobody else holds")
	}
	if countTaskTablesNamed(t, database, "flow_definitions") != 1 {
		t.Fatal("a device that names no company let go of the vocabulary nobody else answers")
	}
}

func TestTaskSweepDropsAVocabularyTheCompanyAnswers(t *testing.T) {
	service := newTaskSweepTestService(t, startCompanyForTaskCarry(t, nil).server.URL)
	seedRetiredTaskTables(t, service.stateDatabasePath(), 3)

	service.sweepTheTasksTheCompanyNowHolds(context.Background())

	database := openTaskStoreForTest(t, service)
	for _, tableName := range []string{"flow_definitions", "flow_size_definitions", "flow_tasks"} {
		if countTaskTablesNamed(t, database, tableName) != 0 {
			t.Fatalf("%s survived a company that answers its own vocabulary", tableName)
		}
	}
}

func TestTaskSweepKeepsAVocabularyTheCompanyDoesNotAnswer(t *testing.T) {
	company := startCompanyForTaskCarry(t, nil)
	company.businesses = []string{}
	service := newTaskSweepTestService(t, company.server.URL)
	seedRetiredTaskTables(t, service.stateDatabasePath(), 3)

	service.sweepTheTasksTheCompanyNowHolds(context.Background())

	database := openTaskStoreForTest(t, service)
	if countTaskTablesNamed(t, database, "flow_definitions") != 1 {
		t.Fatal("the only task vocabulary there is was dropped")
	}
	if countTaskTablesNamed(t, database, "flow_tasks") != 1 {
		t.Fatal("a vocabulary nobody else answers must keep the whole store")
	}
}

func TestTaskSweepKeepsAStoreItCannotRead(t *testing.T) {
	service := newTaskSweepTestService(t, startCompanyForTaskCarry(t, nil).server.URL)
	store := seedRetiredTaskTables(t, service.stateDatabasePath(), 0)
	seedDeviceTask(t, store, "task-1", "planned", "2026-07-06", "2026-07-07")
	if _, errorValue := store.ExecContext(context.Background(),
		"ALTER TABLE flow_tasks RENAME COLUMN participant_ids TO who"); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.sweepTheTasksTheCompanyNowHolds(context.Background())

	database := openTaskStoreForTest(t, service)
	if countTaskTablesNamed(t, database, "flow_tasks") != 1 {
		t.Fatal("a store that could not be read was let go of anyway")
	}
}

func TestTaskCarryWritesEveryTaskTheRecordDoesNotHold(t *testing.T) {
	company := startCompanyForTaskCarry(t, nil)
	service := newTaskSweepTestService(t, company.server.URL)
	store := seedRetiredTaskTables(t, service.stateDatabasePath(), 0)
	seedDeviceTask(t, store, "task-1", "planned", "2026-07-06", "2026-07-07")
	seedDeviceTask(t, store, "task-2", "in_progress", "2026-07-08", "")

	report, errorValue := service.carryTasksIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Tasks != 2 || len(report.Refused) != 0 {
		t.Fatalf("report = %+v", report)
	}
	if len(company.written) != 2 || company.written[0].Title != "보고서 초안" {
		t.Fatalf("written = %+v", company.written)
	}
	if company.written[0].Status != "planned" || company.written[0].Starts != "2026-07-06" {
		t.Fatalf("the row did not land as it happened: %+v", company.written[0])
	}
	if len(company.written[0].Mirrors) != 1 || company.written[0].Mirrors[0].ExternalID != "task-1" {
		t.Fatalf("the record was not told where the row came from: %+v", company.written[0])
	}
}

// A carried row is still in the local table, so a sweep that only counted rows
// would keep the store forever and the carry would never finish anything.
func TestTaskSweepDropsTheStoreOnceTheCarryHasFinished(t *testing.T) {
	company := startCompanyForTaskCarry(t, nil)
	service := newTaskSweepTestService(t, company.server.URL)
	store := seedRetiredTaskTables(t, service.stateDatabasePath(), 0)
	seedDeviceTask(t, store, "task-1", "planned", "2026-07-06", "2026-07-07")
	seedDeviceTask(t, store, "task-2", "requested", "2026-07-06", "2026-07-07")

	if _, errorValue := service.carryTasksIntoTheRecord(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.sweepTheTasksTheCompanyNowHolds(context.Background())

	database := openTaskStoreForTest(t, service)
	if countTaskTablesNamed(t, database, "flow_tasks") != 0 {
		t.Fatal("the store stayed after the record took everything in it")
	}
	if countTaskTablesNamed(t, database, "task_carried_rows") != 0 {
		t.Fatal("the memory of a carry outlived what it described")
	}
}

// The record already carries most of what a device holds, from the hand import
// that predates this. Those rows are counted, never written a second time.
func TestTaskCarryWritesNothingForARowTheRecordAlreadyCarries(t *testing.T) {
	company := startCompanyForTaskCarry(t, map[string]string{"task-1": "44444444-4444-4444-8444-444444444444"})
	service := newTaskSweepTestService(t, company.server.URL)
	store := seedRetiredTaskTables(t, service.stateDatabasePath(), 0)
	seedDeviceTask(t, store, "task-1", "planned", "2026-07-06", "2026-07-07")

	report, errorValue := service.carryTasksIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Tasks != 1 || len(company.written) != 0 {
		t.Fatalf("report = %+v written = %+v", report, company.written)
	}
}

func TestTaskCarryRefusesRatherThanLetTheRecordReshapeARow(t *testing.T) {
	company := startCompanyForTaskCarry(t, nil)
	service := newTaskSweepTestService(t, company.server.URL)
	store := seedRetiredTaskTables(t, service.stateDatabasePath(), 0)
	tomorrow := time.Now().AddDate(0, 0, 2).Format(time.DateOnly)
	seedDeviceTask(t, store, "completed-in-the-future", "completed", "2026-07-06", tomorrow)
	seedDeviceTask(t, store, "ends-before-it-starts", "planned", "2026-07-08", "2026-07-06")
	seedDeviceTask(t, store, "not-a-date", "planned", "언젠가", "")

	report, errorValue := service.carryTasksIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Tasks != 0 || len(report.Refused) != 3 {
		t.Fatalf("report = %+v", report)
	}
	if len(company.written) != 0 {
		t.Fatalf("a row the record would have reshaped was written anyway: %+v", company.written)
	}
	for _, expected := range []string{"completed-in-the-future", "ends-before-it-starts", "not-a-date"} {
		if !strings.Contains(strings.Join(report.Refused, "\n"), expected) {
			t.Fatalf("%s was refused without being named: %+v", expected, report.Refused)
		}
	}
}

func TestTaskCarryWritesARequestedRowNamingNobodyWhoAsked(t *testing.T) {
	company := startCompanyForTaskCarry(t, nil)
	service := newTaskSweepTestService(t, company.server.URL)
	store := seedRetiredTaskTables(t, service.stateDatabasePath(), 0)
	seedDeviceTask(t, store, "requested-by-nobody", "requested", "2026-07-06", "2026-07-07")
	seedDeviceTask(t, store, "rejected-by-nobody", "rejected", "2026-07-06", "2026-07-07")
	seedDeviceTask(t, store, "planned-here", "planned", "2026-07-06", "2026-07-07")

	report, errorValue := service.carryTasksIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Tasks != 3 || len(report.Refused) != 0 {
		t.Fatalf("report = %+v", report)
	}
	if report.WithoutRequester != 2 {
		t.Fatalf("the carry has to say how many rows landed naming nobody, got %d", report.WithoutRequester)
	}
	for _, written := range company.written {
		if len(written.Mirrors) != 1 || written.Mirrors[0].Source != centralplane.DeviceMirrorSource {
			t.Fatalf("the record cannot tell a carried row from an ask made here without this: %+v", written)
		}
	}
}

func TestTaskCarryRefusesARowNamingNobodyTheCompanyKnows(t *testing.T) {
	company := startCompanyForTaskCarry(t, nil)
	service := newTaskSweepTestService(t, company.server.URL)
	service.policyRecordCache = nil
	store := seedRetiredTaskTables(t, service.stateDatabasePath(), 0)
	seedDeviceTask(t, store, "task-1", "planned", "2026-07-06", "2026-07-07")

	report, errorValue := service.carryTasksIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Tasks != 0 || len(report.Refused) != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(company.written) != 0 {
		t.Fatalf("a row nobody owns was written anyway: %+v", company.written)
	}
}
