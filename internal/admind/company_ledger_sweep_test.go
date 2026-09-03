package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type carriedLedgerRow struct {
	Table string
	Body  map[string]any
	Token string
}

// The plane a carry writes into: it names one company, hands every address it
// knows a token of its own so a test can tell who signed a row, and refuses an
// address nobody works under.
func startPlaneHoldingTheCompany(t *testing.T, known []string, refuse func(table string) bool) (*httptest.Server, *[]carriedLedgerRow) {
	t.Helper()
	written := []carriedLedgerRow{}
	knownAddresses := map[string]bool{}
	for _, address := range known {
		knownAddresses[address] = true
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		if request.Method == http.MethodPost && request.URL.Path == "/api/agent/session" {
			var asked struct {
				ExternalID string `json:"externalID"`
			}
			_ = json.NewDecoder(request.Body).Decode(&asked)
			if !knownAddresses[asked.ExternalID] {
				http.Error(writer, "no such member", http.StatusNotFound)
				return
			}
			_, _ = writer.Write([]byte(`{"memberID":"member-` + asked.ExternalID + `","accessToken":"token-` + asked.ExternalID + `","expiresAt":99999999999}`))
			return
		}
		if request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/rest/v1/company") {
			_, _ = writer.Write([]byte(`[{"id":"company-1"}]`))
			return
		}
		if request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/rest/v1/") {
			table := strings.TrimPrefix(request.URL.Path, "/rest/v1/")
			if refuse != nil && refuse(table) {
				http.Error(writer, "no", http.StatusForbidden)
				return
			}
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			written = append(written, carriedLedgerRow{
				Table: table,
				Body:  body,
				Token: strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "),
			})
			writer.WriteHeader(http.StatusCreated)
			return
		}
		_, _ = writer.Write([]byte(`[]`))
	}))
	t.Cleanup(server.Close)
	return server, &written
}

func newCompanyLedgerTestService(t *testing.T, planeURL string) *Service {
	t.Helper()
	configuration := Configuration{
		StateDirectory:        t.TempDir(),
		ClaimedAdminEmailPath: writeTestFile(t, "admin@example.com"),
	}
	if planeURL != "" {
		configuration.CentralPlaneAppURL = planeURL
		configuration.CentralPlaneProjectURL = planeURL
		configuration.CentralPlanePublishableKey = "publishable"
		configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")
	}
	return NewService(configuration)
}

func openSeededLedgerStore(t *testing.T, databasePath string) *sql.DB {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(databasePath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(databasePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return database
}

func seedRetiredCompanyLedger(t *testing.T, databasePath string) {
	t.Helper()
	database := openSeededLedgerStore(t, databasePath)
	defer database.Close()
	for _, statement := range []string{
		"CREATE TABLE company_metrics (metric TEXT NOT NULL, year INTEGER NOT NULL, quarter INTEGER NOT NULL DEFAULT 0, month INTEGER NOT NULL DEFAULT 0, value REAL NOT NULL, currency TEXT NOT NULL DEFAULT '', value_usd REAL, unit TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL, PRIMARY KEY (metric, year, quarter, month))",
		"CREATE TABLE company_records (id TEXT PRIMARY KEY, category TEXT NOT NULL, record_date TEXT NOT NULL DEFAULT '', title TEXT NOT NULL, detail TEXT NOT NULL DEFAULT '', attributes TEXT NOT NULL DEFAULT '{}', updated_at TEXT NOT NULL)",
		"CREATE TABLE company_documents (id TEXT PRIMARY KEY, document_number TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT 'issued', document_type TEXT NOT NULL, title TEXT NOT NULL, counterpart TEXT NOT NULL DEFAULT '', language TEXT NOT NULL DEFAULT '', file_path TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '', summary_embedding TEXT NOT NULL DEFAULT '', requester_email TEXT NOT NULL DEFAULT '', issued_at TEXT NOT NULL, updated_at TEXT NOT NULL)",
		`INSERT INTO company_metrics (metric, year, quarter, month, value, currency, value_usd, unit, note, updated_at)
		 VALUES ('annualRevenue', 2025, 0, 0, 1200000000, 'KRW', 870000, '', '재무제표 기준', '2026-01-01T00:00:00Z')`,
		`INSERT INTO company_records (id, category, record_date, title, detail, attributes, updated_at)
		 VALUES ('record-1', 'funding', '2025-12-01', '시드 투자 유치', '제품 검증 자금', '{"round":"Seed"}', '2026-01-01T00:00:00Z')`,
		`INSERT INTO company_documents (id, document_number, kind, document_type, title, counterpart, language, file_path, summary, summary_embedding, requester_email, issued_at, updated_at)
		 VALUES ('document-1', 'Q-2025-001', 'issued', 'quote', 'ABC물산 견적서', 'ABC물산', 'ko', 'shared/q.md', '용역 견적 요약', '', 'member@example.com', '2025-12-02T00:00:00Z', '2025-12-02T00:00:00Z')`,
	} {
		if _, errorValue := database.ExecContext(context.Background(), statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func countCompanyLedgerTablesNamed(t *testing.T, database *sql.DB, tableName string) int {
	t.Helper()
	var count int
	if errorValue := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}

func TestCompanyLedgerCarryWritesEveryRowAsWhoseItIs(t *testing.T) {
	plane, written := startPlaneHoldingTheCompany(t, []string{"admin@example.com", "member@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	seedRetiredCompanyLedger(t, service.stateDatabasePath())

	report, errorValue := service.carryTheCompanyLedgerIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Metrics != 1 || report.Records != 1 || report.Documents != 1 || len(report.Refused) != 0 {
		t.Fatalf("carry report = %#v", report)
	}

	signerOf := map[string]string{}
	for _, row := range *written {
		signerOf[row.Table] = row.Token
		if row.Body["company_id"] != "company-1" {
			t.Fatalf("%s was carried without naming its company: %#v", row.Table, row.Body)
		}
	}
	if signerOf["company_metric"] != "token-admin@example.com" || signerOf["company_record"] != "token-admin@example.com" {
		t.Fatalf("what a company states about itself is an administrator's to record: %#v", signerOf)
	}
	if signerOf["company_document"] != "token-member@example.com" {
		t.Fatalf("a document is written as the person who asked for it: %#v", signerOf)
	}
}

func TestCompanyLedgerCarryKeepsWhatTheDeviceWroteRatherThanMintingItAgain(t *testing.T) {
	plane, written := startPlaneHoldingTheCompany(t, []string{"admin@example.com", "member@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	seedRetiredCompanyLedger(t, service.stateDatabasePath())

	if _, errorValue := service.carryTheCompanyLedgerIntoTheRecord(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}

	for _, row := range *written {
		switch row.Table {
		case "company_document":
			if row.Body["document_number"] != "Q-2025-001" {
				t.Fatalf("the number this document was issued under was not kept: %#v", row.Body)
			}
			if row.Body["issued_at"] != "2025-12-02T00:00:00Z" {
				t.Fatalf("the day this document was issued was not kept: %#v", row.Body)
			}
			if row.Body["requester_id"] != "member-member@example.com" {
				t.Fatalf("the document does not name who asked for it: %#v", row.Body)
			}
		case "company_metric":
			if row.Body["value_usd"] != float64(870000) || row.Body["currency_code"] != "KRW" {
				t.Fatalf("the amount and its USD equivalent were not kept together: %#v", row.Body)
			}
		case "company_record":
			attributes, isObject := row.Body["attributes"].(map[string]any)
			if !isObject || attributes["round"] != "Seed" {
				t.Fatalf("the structured details were not kept: %#v", row.Body)
			}
		}
	}
}

func TestCompanyLedgerCarryRefusesADocumentWhoseRequesterTheCompanyDoesNotKnow(t *testing.T) {
	plane, written := startPlaneHoldingTheCompany(t, []string{"admin@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	seedRetiredCompanyLedger(t, service.stateDatabasePath())

	report, errorValue := service.carryTheCompanyLedgerIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Documents != 0 || len(report.Refused) != 1 {
		t.Fatalf("a document nobody here asked for must be named, not written: %#v", report)
	}
	if !strings.Contains(report.Refused[0], "member@example.com") {
		t.Fatalf("the refusal does not name the address: %q", report.Refused[0])
	}
	for _, row := range *written {
		if row.Table == "company_document" {
			t.Fatalf("a refused document was written anyway: %#v", row.Body)
		}
	}
}

func TestCompanyLedgerCarryRefusesARecordDatedByMonthAlone(t *testing.T) {
	plane, _ := startPlaneHoldingTheCompany(t, []string{"admin@example.com", "member@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	seedRetiredCompanyLedger(t, service.stateDatabasePath())
	database := openSeededLedgerStore(t, service.stateDatabasePath())
	if _, errorValue := database.Exec(
		"UPDATE company_records SET record_date = '2025-12' WHERE id = 'record-1'"); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	report, errorValue := service.carryTheCompanyLedgerIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Records != 0 || len(report.Refused) != 1 {
		t.Fatalf("a half date is refused rather than guessed into a day: %#v", report)
	}
	if !strings.Contains(report.Refused[0], "2025-12") {
		t.Fatalf("the refusal does not name the date: %q", report.Refused[0])
	}
}

func TestCompanyLedgerCarryRefusesAnAmountWithoutItsUSDEquivalent(t *testing.T) {
	plane, _ := startPlaneHoldingTheCompany(t, []string{"admin@example.com", "member@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	seedRetiredCompanyLedger(t, service.stateDatabasePath())
	database := openSeededLedgerStore(t, service.stateDatabasePath())
	if _, errorValue := database.Exec(
		"UPDATE company_metrics SET value_usd = NULL WHERE metric = 'annualRevenue'"); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	report, errorValue := service.carryTheCompanyLedgerIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Metrics != 0 || len(report.Refused) != 1 {
		t.Fatalf("an amount the record cannot compare is refused rather than carried: %#v", report)
	}
	if !strings.Contains(report.Refused[0], "KRW") {
		t.Fatalf("the refusal does not name the currency: %q", report.Refused[0])
	}
}

func TestCompanyLedgerCarryNamesWhatTheRecordRefused(t *testing.T) {
	plane, _ := startPlaneHoldingTheCompany(t, []string{"admin@example.com", "member@example.com"},
		func(table string) bool { return table == "company_metric" })
	service := newCompanyLedgerTestService(t, plane.URL)
	seedRetiredCompanyLedger(t, service.stateDatabasePath())

	report, errorValue := service.carryTheCompanyLedgerIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Metrics != 0 || report.Records != 1 || len(report.Refused) != 1 {
		t.Fatalf("a refused row must be named while the rest goes: %#v", report)
	}
	if !strings.Contains(report.Refused[0], "annualRevenue") {
		t.Fatalf("the refusal does not name the metric: %q", report.Refused[0])
	}
}

// A carried row is still in the local table, so a sweep that only counted rows
// would keep the store forever and the carry would never finish anything.
func TestCompanyLedgerSweepDropsTheStoreOnceTheCarryHasFinished(t *testing.T) {
	plane, _ := startPlaneHoldingTheCompany(t, []string{"admin@example.com", "member@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	seedRetiredCompanyLedger(t, service.stateDatabasePath())

	if _, errorValue := service.carryTheCompanyLedgerIntoTheRecord(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.sweepTheCompanyLedgerTheRecordNowHolds(context.Background())

	database, errorValue := service.openCompanyLedgerDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, tableName := range carriedCompanyLedgerTables {
		if countCompanyLedgerTablesNamed(t, database, tableName) != 0 {
			t.Fatalf("%s survived a finished carry", tableName)
		}
	}
}

func TestCompanyLedgerSweepKeepsRowsTheRecordHasNotTaken(t *testing.T) {
	plane, _ := startPlaneHoldingTheCompany(t, []string{"admin@example.com", "member@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	seedRetiredCompanyLedger(t, service.stateDatabasePath())

	service.sweepTheCompanyLedgerTheRecordNowHolds(context.Background())

	database, errorValue := service.openCompanyLedgerDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCompanyLedgerTablesNamed(t, database, "company_documents") != 1 {
		t.Fatal("a store the record has not taken was dropped")
	}
}

func TestCompanyLedgerSweepKeepsEverythingWhenNoCompanyIsNamed(t *testing.T) {
	service := newCompanyLedgerTestService(t, "")
	seedRetiredCompanyLedger(t, service.stateDatabasePath())

	service.sweepTheCompanyLedgerTheRecordNowHolds(context.Background())

	database, errorValue := service.openCompanyLedgerDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, tableName := range []string{"company_metrics", "company_records", "company_documents"} {
		if countCompanyLedgerTablesNamed(t, database, tableName) != 1 {
			t.Fatalf("%s was dropped by a device the record has never heard of", tableName)
		}
	}
}

func TestCompanyLedgerSweepKeepsTheStoreWhenItCannotBeRead(t *testing.T) {
	plane, _ := startPlaneHoldingTheCompany(t, []string{"admin@example.com", "member@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	seedRetiredCompanyLedger(t, service.stateDatabasePath())
	database := openSeededLedgerStore(t, service.stateDatabasePath())
	if _, errorValue := database.Exec("DROP TABLE company_records"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec("ALTER TABLE company_documents RENAME COLUMN issued_at TO issued"); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	service.sweepTheCompanyLedgerTheRecordNowHolds(context.Background())

	kept, errorValue := service.openCompanyLedgerDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer kept.Close()
	if countCompanyLedgerTablesNamed(t, kept, "company_documents") != 1 {
		t.Fatal("a store that could not be read was dropped anyway")
	}
}

// The oldest stores predate the currency a metric carries. Their rows still
// have to be readable, or the carry cannot get them out.
func TestCompanyLedgerReadsAStoreThatPredatesTheCurrencyColumn(t *testing.T) {
	plane, written := startPlaneHoldingTheCompany(t, []string{"admin@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	database := openSeededLedgerStore(t, service.stateDatabasePath())
	if _, errorValue := database.Exec(`
CREATE TABLE company_metrics (
	metric TEXT NOT NULL,
	year INTEGER NOT NULL,
	quarter INTEGER NOT NULL DEFAULT 0,
	month INTEGER NOT NULL DEFAULT 0,
	value REAL NOT NULL,
	unit TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL,
	PRIMARY KEY (metric, year, quarter, month)
);
INSERT INTO company_metrics (metric, year, value, unit, updated_at)
VALUES ('sites', 2025, 63, '곳', '2026-01-01T00:00:00Z')`); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	report, errorValue := service.carryTheCompanyLedgerIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Metrics != 1 || len(report.Refused) != 0 {
		t.Fatalf("a metric written before the currency column could not be carried: %#v", report)
	}
	if len(*written) != 1 || (*written)[0].Body["unit"] != "곳" {
		t.Fatalf("the metric was not carried as it was written: %#v", *written)
	}
	if _, carriesCurrency := (*written)[0].Body["currency_code"]; carriesCurrency {
		t.Fatalf("a metric that names no currency was given one: %#v", (*written)[0].Body)
	}
}

func TestCompanyLedgerCarryRefusesWhenNoAdministratorIsClaimed(t *testing.T) {
	plane, _ := startPlaneHoldingTheCompany(t, []string{"admin@example.com"}, nil)
	service := newCompanyLedgerTestService(t, plane.URL)
	service.Configuration.ClaimedAdminEmailPath = writeTestFile(t, "")
	seedRetiredCompanyLedger(t, service.stateDatabasePath())

	_, errorValue := service.carryTheCompanyLedgerIntoTheRecord(context.Background())
	if errorValue == nil || !strings.Contains(errorValue.Error(), "administrator") {
		t.Fatalf("carry error = %v", errorValue)
	}
}
