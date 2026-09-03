package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type carriedCRMRow struct {
	Table string
	Body  map[string]any
}

// The plane a CRM carry writes into: it names one company, knows the members it
// is told about, and mints a uuid for every row it takes.
func startPlaneTakingCRM(t *testing.T, memberIDs []string) (*httptest.Server, *[]carriedCRMRow) {
	t.Helper()
	written := []carriedCRMRow{}
	minted := 0
	members := []string{}
	for _, memberID := range memberIDs {
		members = append(members, `{"memberID":"`+memberID+`","email":"`+memberID+`@example.com","status":"active"}`)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/api/agent/session":
			_, _ = writer.Write([]byte(`{"memberID":"member-admin","accessToken":"token","expiresAt":99999999999}`))
		case request.URL.Path == "/api/agent/member":
			_, _ = writer.Write([]byte(`{"members":[` + strings.Join(members, ",") + `]}`))
		case request.Method == http.MethodGet && strings.HasPrefix(request.URL.Path, "/rest/v1/company"):
			_, _ = writer.Write([]byte(`[{"id":"company-1"}]`))
		case request.Method == http.MethodPost && strings.HasPrefix(request.URL.Path, "/rest/v1/"):
			var body map[string]any
			_ = json.NewDecoder(request.Body).Decode(&body)
			table := strings.TrimPrefix(request.URL.Path, "/rest/v1/")
			minted++
			written = append(written, carriedCRMRow{Table: table, Body: body})
			writer.WriteHeader(http.StatusCreated)
			_, _ = writer.Write([]byte(`[{"id":"` + table + `-uuid-` + string(rune('a'+minted-1)) + `"}]`))
		default:
			_, _ = writer.Write([]byte(`[]`))
		}
	}))
	t.Cleanup(server.Close)
	return server, &written
}

func newCRMCarryTestService(t *testing.T, planeURL string) *Service {
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

// admind creates no CRM store any more, so a test that wants one holding rows
// writes the columns the carry reads, the way a device that used the CRM has
// them.
var retiredCRMSchema = []string{
	`CREATE TABLE account (id TEXT PRIMARY KEY, name TEXT NOT NULL, status TEXT NOT NULL,
		types TEXT, tags TEXT NOT NULL, importance TEXT NOT NULL, owner_person_id TEXT NOT NULL,
		owner_circle_id TEXT, address TEXT, description TEXT, created_at TEXT NOT NULL,
		created_by_person_id TEXT NOT NULL, updated_at TEXT NOT NULL,
		updated_by_person_id TEXT NOT NULL, archived_at TEXT, archived_by_person_id TEXT)`,
	`CREATE TABLE contact (id TEXT PRIMARY KEY, account_id TEXT, name TEXT NOT NULL, email TEXT,
		phone TEXT, title TEXT, department TEXT, is_primary INTEGER NOT NULL,
		owner_person_id TEXT NOT NULL, owner_circle_id TEXT, description TEXT,
		created_at TEXT NOT NULL, created_by_person_id TEXT NOT NULL, updated_at TEXT NOT NULL,
		updated_by_person_id TEXT NOT NULL, archived_at TEXT, archived_by_person_id TEXT)`,
	`CREATE TABLE opportunity (id TEXT PRIMARY KEY, account_id TEXT, business TEXT,
		name TEXT NOT NULL, pipeline TEXT NOT NULL, stage TEXT NOT NULL,
		stage_position REAL NOT NULL, stage_changed_at TEXT NOT NULL, owner_person_id TEXT NOT NULL,
		owner_circle_id TEXT, amount_minor INTEGER, currency_code TEXT NOT NULL,
		base_amount_minor INTEGER, base_currency_code TEXT, importance TEXT NOT NULL, due_at TEXT,
		due_time_zone TEXT, lost_reason TEXT, description TEXT, created_at TEXT NOT NULL,
		created_by_person_id TEXT NOT NULL, updated_at TEXT NOT NULL,
		updated_by_person_id TEXT NOT NULL, archived_at TEXT, archived_by_person_id TEXT)`,
	`CREATE TABLE opportunity_contact (opportunity_id TEXT NOT NULL, contact_id TEXT NOT NULL,
		is_primary INTEGER NOT NULL, PRIMARY KEY (opportunity_id, contact_id))`,
	`CREATE TABLE activity (id TEXT PRIMARY KEY, account_id TEXT, contact_id TEXT,
		opportunity_id TEXT, business TEXT, kind TEXT NOT NULL, title TEXT NOT NULL,
		occurred_at TEXT NOT NULL, content TEXT, created_at TEXT NOT NULL,
		created_by_person_id TEXT NOT NULL, updated_at TEXT NOT NULL,
		updated_by_person_id TEXT NOT NULL, archived_at TEXT, archived_by_person_id TEXT)`,
	`CREATE TABLE resource_link (id TEXT PRIMARY KEY, entity_type TEXT NOT NULL,
		entity_id TEXT NOT NULL, service TEXT NOT NULL, external_resource_type TEXT NOT NULL,
		external_resource_id TEXT NOT NULL, external_resource_url TEXT, created_at TEXT NOT NULL,
		created_by_person_id TEXT NOT NULL, removed_at TEXT, removed_by_person_id TEXT)`,
}

func seedCRMStore(t *testing.T, service *Service, statements ...string) {
	t.Helper()
	statements = append(append([]string{}, retiredCRMSchema...), statements...)
	database, errorValue := service.openCRMDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(context.Background(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, statement := range statements {
		if _, errorValue := transaction.ExecContext(context.Background(), statement); errorValue != nil {
			transaction.Rollback()
			t.Fatal(errorValue)
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		t.Fatal(errorValue)
	}
}

const seededCRMAccount = `
INSERT INTO account (id, name, status, types, tags, importance, owner_person_id, address, description,
	created_at, created_by_person_id, updated_at, updated_by_person_id)
VALUES ('account-1', 'ABC물산', 'active', '["customer"]', '["seoul"]', 'high', 'member-1', '서울', '주요 고객',
	'2026-01-01T00:00:00Z', 'member-1', '2026-01-02T00:00:00Z', 'member-1')`

const seededCRMContact = `
INSERT INTO contact (id, account_id, name, email, phone, title, department, is_primary,
	owner_person_id, description, created_at, created_by_person_id, updated_at, updated_by_person_id)
VALUES ('contact-1', 'account-1', '이샘플', 'sample@example.com', NULL, '팀장', '구매', 1,
	'member-1', NULL, '2026-01-03T00:00:00Z', 'member-1', '2026-01-03T00:00:00Z', 'member-1')`

const seededCRMOpportunity = `
INSERT INTO opportunity (id, account_id, name, pipeline, stage, stage_position, stage_changed_at,
	owner_person_id, currency_code, importance, created_at, created_by_person_id, updated_at,
	updated_by_person_id)
VALUES ('opportunity-1', 'account-1', '도입 컨설팅', 'sales', 'in_progress', 1024, '2026-01-05T00:00:00Z',
	'member-1', 'KRW', 'high', '2026-01-04T00:00:00Z', 'member-1', '2026-01-05T00:00:00Z', 'member-1')`

func TestCRMCarryWritesEveryRowAndRewritesItsLinks(t *testing.T) {
	plane, written := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount, seededCRMContact, seededCRMOpportunity)

	report, errorValue := service.carryTheCRMIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Organizations != 1 || report.Contacts != 1 || report.Opportunities != 1 || len(report.Refused) != 0 {
		t.Fatalf("carry report = %#v", report)
	}

	byTable := map[string]map[string]any{}
	for _, row := range *written {
		byTable[row.Table] = row.Body
		if row.Body["company_id"] != "company-1" {
			t.Fatalf("%s was carried without naming its company: %#v", row.Table, row.Body)
		}
	}
	if byTable["organization"]["name"] != "ABC물산" || byTable["organization"]["created_at"] != "2026-01-01T00:00:00Z" {
		t.Fatalf("the relationship did not keep what it was written with: %#v", byTable["organization"])
	}
	if byTable["contact"]["organization_id"] != "organization-uuid-a" {
		t.Fatalf("the contact was not linked to the relationship the record minted: %#v", byTable["contact"])
	}
	if byTable["opportunity"]["organization_id"] != "organization-uuid-a" {
		t.Fatalf("the deal's relationship was not rewritten to the record's id: %#v", byTable["opportunity"])
	}
	if _, named := byTable["opportunity"]["contact_id"]; named {
		t.Fatalf("a deal linked to no contact was given one: %#v", byTable["opportunity"])
	}
	if byTable["opportunity"]["stage_id"] != "in_progress" || byTable["opportunity"]["stage_changed_at"] != "2026-01-05T00:00:00Z" {
		t.Fatalf("the deal did not keep the stage it was in and when it moved: %#v", byTable["opportunity"])
	}
}

func TestCRMCarryRemembersWhatItTookSoASecondRunRepeatsNothing(t *testing.T) {
	plane, written := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount)

	if _, errorValue := service.carryTheCRMIntoTheRecord(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	report, errorValue := service.carryTheCRMIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Organizations != 0 || len(*written) != 1 {
		t.Fatalf("a second carry wrote the same relationship again: %#v, %d writes", report, len(*written))
	}
}

func TestCRMCarryRefusesAnOwnerTheCompanyDoesNotKnow(t *testing.T) {
	plane, written := startPlaneTakingCRM(t, []string{"somebody-else"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount)

	report, errorValue := service.carryTheCRMIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Organizations != 0 || len(report.Refused) != 1 {
		t.Fatalf("a row owned by a stranger is named, not written: %#v", report)
	}
	if !strings.Contains(report.Refused[0], "does not know") || len(*written) != 0 {
		t.Fatalf("refusal = %q, writes = %d", report.Refused[0], len(*written))
	}
}

func TestCRMCarryRefusesAStageTheRecordDoesNotKeep(t *testing.T) {
	plane, _ := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount, `
INSERT INTO opportunity (id, account_id, name, pipeline, stage, stage_position, stage_changed_at,
	owner_person_id, currency_code, importance, created_at, created_by_person_id, updated_at,
	updated_by_person_id)
VALUES ('opportunity-9', 'account-1', '값 흥정', 'sales', 'haggling', 1024, '2026-01-05T00:00:00Z',
	'member-1', 'KRW', 'high', '2026-01-04T00:00:00Z', 'member-1', '2026-01-05T00:00:00Z', 'member-1')`)

	report, errorValue := service.carryTheCRMIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Opportunities != 0 || len(report.Refused) != 1 {
		t.Fatalf("a stage the record has no word for is named, not guessed: %#v", report)
	}
	if !strings.Contains(report.Refused[0], "haggling") {
		t.Fatalf("the refusal does not name the stage: %q", report.Refused[0])
	}
}

func TestCRMCarryRefusesADealAgainstNoRelationship(t *testing.T) {
	plane, _ := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMContact2(), `
INSERT INTO opportunity (id, account_id, name, pipeline, stage, stage_position, stage_changed_at,
	owner_person_id, currency_code, importance, created_at, created_by_person_id, updated_at,
	updated_by_person_id)
VALUES ('opportunity-2', NULL, '개인 고객 건', 'sales', 'waiting', 1024, '2026-01-05T00:00:00Z',
	'member-1', 'KRW', 'medium', '2026-01-04T00:00:00Z', 'member-1', '2026-01-05T00:00:00Z', 'member-1')`, `
INSERT INTO opportunity_contact (opportunity_id, contact_id, is_primary)
VALUES ('opportunity-2', 'contact-2', 1)`)

	report, errorValue := service.carryTheCRMIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Opportunities != 0 || len(report.Refused) != 1 {
		t.Fatalf("a contact-only deal has no record home and is named: %#v", report)
	}
	if !strings.Contains(report.Refused[0], "against a contact") {
		t.Fatalf("the refusal does not say why: %q", report.Refused[0])
	}
}

func seededCRMContact2() string {
	return `
INSERT INTO contact (id, account_id, name, email, phone, title, department, is_primary,
	owner_person_id, description, created_at, created_by_person_id, updated_at, updated_by_person_id)
VALUES ('contact-2', NULL, '박예시', 'example@example.com', NULL, NULL, NULL, 0,
	'member-1', NULL, '2026-01-03T00:00:00Z', 'member-1', '2026-01-03T00:00:00Z', 'member-1')`
}

func TestCRMCarryDropsEveryContactLinkBesideTheOneTheDealIsWith(t *testing.T) {
	plane, written := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount, seededCRMContact, seededCRMContact2(),
		`UPDATE contact SET account_id = 'account-1' WHERE id = 'contact-2'`,
		seededCRMOpportunity, `
INSERT INTO opportunity_contact (opportunity_id, contact_id, is_primary) VALUES
	('opportunity-1', 'contact-1', 1), ('opportunity-1', 'contact-2', 0)`)

	report, errorValue := service.carryTheCRMIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Opportunities != 1 || len(report.Dropped) != 1 {
		t.Fatalf("the deal is carried and the extra link is named: %#v", report)
	}
	if !strings.Contains(report.Dropped[0], "contact-2") {
		t.Fatalf("the dropped link does not name the contact: %q", report.Dropped[0])
	}
	for _, row := range *written {
		if row.Table == "opportunity" && row.Body["contact_id"] != "contact-uuid-b" {
			t.Fatalf("the deal kept the wrong contact: %#v", row.Body)
		}
	}
}

func TestCRMCarryWritesAStageChangeTheWayTheRecordWritesOne(t *testing.T) {
	plane, written := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount, seededCRMOpportunity, `
INSERT INTO activity (id, account_id, opportunity_id, kind, title, occurred_at, content, created_at,
	created_by_person_id, updated_at, updated_by_person_id)
VALUES ('activity-1', 'account-1', 'opportunity-1', 'stage_change', 'waiting → in_progress',
	'2026-01-05T00:00:00Z', NULL, '2026-01-05T00:00:00Z', 'member-1', '2026-01-05T00:00:00Z', 'member-1')`)

	report, errorValue := service.carryTheCRMIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Activities != 1 || len(report.Refused) != 0 {
		t.Fatalf("carry report = %#v", report)
	}
	for _, row := range *written {
		if row.Table != "task" {
			continue
		}
		if row.Body["title"] != "도입 컨설팅" || row.Body["note"] != "waiting → in_progress" {
			t.Fatalf("a carried move does not read the way one recorded here does: %#v", row.Body)
		}
		if row.Body["type"] != "stage_change" || row.Body["status"] != "completed" ||
			row.Body["due_at"] != "2026-01-05T00:00:00Z" {
			t.Fatalf("a carried move is not shaped the way the trigger shapes one: %#v", row.Body)
		}
	}
}

func TestCRMSweepDropsTheStoreOnceTheCarryHasFinished(t *testing.T) {
	plane, _ := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount, seededCRMContact, seededCRMOpportunity)

	if _, errorValue := service.carryTheCRMIntoTheRecord(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.sweepTheCRMTheRecordNowHolds(context.Background())

	database, errorValue := service.openCRMDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, tableName := range carriedCRMTables {
		if countCRMTablesNamed(t, database, tableName) != 0 {
			t.Fatalf("%s survived a finished carry", tableName)
		}
	}
}

func TestCRMSweepKeepsRowsTheRecordHasNotTaken(t *testing.T) {
	plane, _ := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount)

	service.sweepTheCRMTheRecordNowHolds(context.Background())

	database, errorValue := service.openCRMDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCRMTablesNamed(t, database, "account") != 1 {
		t.Fatal("a store the record has not taken was dropped")
	}
}

func TestCRMSweepKeepsEverythingWhenNoCompanyIsNamed(t *testing.T) {
	service := newCRMCarryTestService(t, "")
	seedCRMStore(t, service, seededCRMAccount)

	service.sweepTheCRMTheRecordNowHolds(context.Background())

	database, errorValue := service.openCRMDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCRMTablesNamed(t, database, "account") != 1 {
		t.Fatal("a device the record has never heard of dropped its company's customers")
	}
}

func TestCRMSweepKeepsAStoreHoldingRowsTheRecordHasNoPlaceFor(t *testing.T) {
	plane, _ := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount, `
INSERT INTO resource_link (id, entity_type, entity_id, service, external_resource_type,
	external_resource_id, created_at, created_by_person_id)
VALUES ('link-1', 'account', 'account-1', 'files', 'file', 'file-1', '2026-01-01T00:00:00Z', 'member-1')`)

	if _, errorValue := service.carryTheCRMIntoTheRecord(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.sweepTheCRMTheRecordNowHolds(context.Background())

	database, errorValue := service.openCRMDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCRMTablesNamed(t, database, "account") != 1 {
		t.Fatal("a store holding a link the record has no place for was dropped anyway")
	}
}

func TestCRMSweepKeepsTheStoreWhenItCannotBeRead(t *testing.T) {
	plane, _ := startPlaneTakingCRM(t, []string{"member-1"})
	service := newCRMCarryTestService(t, plane.URL)
	seedCRMStore(t, service, seededCRMAccount, "ALTER TABLE account RENAME COLUMN id TO account_key")

	service.sweepTheCRMTheRecordNowHolds(context.Background())

	database, errorValue := service.openCRMDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if countCRMTablesNamed(t, database, "account") != 1 {
		t.Fatal("a store that could not be read was dropped anyway")
	}
}

func countCRMTablesNamed(t *testing.T, database *sql.DB, tableName string) int {
	t.Helper()
	var count int
	if errorValue := database.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	return count
}
