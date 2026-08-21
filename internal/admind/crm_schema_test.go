package admind

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCRMSchemaCreatesApprovedTablesAndSeeds(t *testing.T) {
	service := Service{Configuration: Configuration{StateDirectory: t.TempDir()}}
	database, errorValue := service.openCRMDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	rows, errorValue := database.Query(`
SELECT name
FROM sqlite_master
WHERE type = 'table'
	AND name NOT LIKE 'sqlite_%'
ORDER BY name`)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	tables := []string{}
	for rows.Next() {
		var tableName string
		if errorValue := rows.Scan(&tableName); errorValue != nil {
			t.Fatal(errorValue)
		}
		tables = append(tables, tableName)
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedTables := []string{
		"account",
		"activity",
		"contact",
		"lost_reason",
		"opportunity",
		"opportunity_contact",
		"pipeline",
		"pipeline_stage",
		"resource_link",
	}
	sort.Strings(expectedTables)
	if len(tables) != len(expectedTables) {
		t.Fatalf("CRM tables = %v, want %v", tables, expectedTables)
	}
	for index := range expectedTables {
		if tables[index] != expectedTables[index] {
			t.Fatalf("CRM tables = %v, want %v", tables, expectedTables)
		}
	}

	assertCRMRowCount(t, database, "pipeline", 6)
	assertCRMRowCount(t, database, "pipeline_stage", 36)
	assertCRMRowCount(t, database, "lost_reason", 7)

	var foreignKeys int
	if errorValue := database.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); errorValue != nil {
		t.Fatal(errorValue)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	if service.crmDatabasePath() != filepath.Join(service.Configuration.StateDirectory, "crm.sqlite") {
		t.Fatalf("CRM database path = %q", service.crmDatabasePath())
	}
}

func TestCRMSchemaRejectsInvalidCoreRows(t *testing.T) {
	service := Service{Configuration: Configuration{StateDirectory: t.TempDir()}}
	database, errorValue := service.openCRMDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	validTime := "2026-08-02T00:00:00Z"
	_, errorValue = database.Exec(`
INSERT INTO contact (
	id, name, email, phone, is_primary, owner_person_id,
	created_at, created_by_person_id, updated_at, updated_by_person_id
) VALUES (?, ?, NULL, NULL, 0, ?, ?, ?, ?, ?)`,
		"contact-missing-identity", "연락처", "person-owner", validTime, "person-owner", validTime, "person-owner")
	if errorValue == nil {
		t.Fatal("contact without email and phone should fail")
	}

	_, errorValue = database.Exec(`
INSERT INTO account (
	id, name, status, tags, importance, owner_person_id,
	created_at, created_by_person_id, updated_at, updated_by_person_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"account-invalid-time", "관계처", "active", "[]", "medium", "person-owner",
		"2026-08-02 00:00:00", "person-owner", validTime, "person-owner")
	if errorValue == nil {
		t.Fatal("account with non-RFC3339 UTC time should fail")
	}

	_, errorValue = database.Exec(`
INSERT INTO account (
	id, name, status, types, tags, importance, owner_person_id,
	created_at, created_by_person_id, updated_at, updated_by_person_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"account-invalid-type", "관계처", "active", `["not-approved"]`, "[]", "medium", "person-owner",
		validTime, "person-owner", validTime, "person-owner")
	if errorValue == nil {
		t.Fatal("account with invalid type should fail")
	}

	_, errorValue = database.Exec(`
INSERT INTO opportunity (
	id, account_id, name, pipeline, stage, stage_position, stage_changed_at,
	owner_person_id, currency_code, importance,
	created_at, created_by_person_id, updated_at, updated_by_person_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"opportunity-invalid-stage", "account-missing", "진행 건", "sales", "not-a-stage", 1024.0, validTime,
		"person-owner", "KRW", "medium", validTime, "person-owner", validTime, "person-owner")
	if errorValue == nil {
		t.Fatal("opportunity with invalid account or pipeline stage should fail")
	}
}

func TestCRMSchemaInitializationRollsBackOnFailure(t *testing.T) {
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(filepath.Join(t.TempDir(), "crm.sqlite")))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	configureSQLiteDatabase(database)
	if errorValue := configureSQLiteConnection(context.Background(), database); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec("CREATE TABLE pipeline (pipeline TEXT PRIMARY KEY)"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := ensureCRMSchema(context.Background(), database); errorValue == nil {
		t.Fatal("schema initialization should fail for an incompatible existing table")
	}

	var accountTableCount int
	if errorValue := database.QueryRow(`
SELECT COUNT(*)
FROM sqlite_master
WHERE type = 'table' AND name = 'account'`).Scan(&accountTableCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if accountTableCount != 0 {
		t.Fatalf("account table count = %d, want rollback to 0", accountTableCount)
	}
}

func TestCRMSchemaMigratesLegacyPipelineStagesToUniformSet(t *testing.T) {
	ctx := context.Background()
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(filepath.Join(t.TempDir(), "crm.sqlite")))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	configureSQLiteDatabase(database)
	if errorValue := configureSQLiteConnection(ctx, database); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, statement := range crmSchemaStatements {
		if _, errorValue := database.Exec(statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	validTime := "2026-08-02T00:00:00Z"
	if _, errorValue := database.Exec(`INSERT INTO pipeline(pipeline, label, direction, is_active) VALUES(?, ?, ?, 1)`,
		"sales", "영업", "inbound"); errorValue != nil {
		t.Fatal(errorValue)
	}
	legacyStages := []struct {
		stage    string
		position int
		outcome  string
	}{
		{stage: "lead", position: 1, outcome: "open"},
		{stage: "qualified", position: 2, outcome: "open"},
		{stage: "proposal", position: 3, outcome: "open"},
		{stage: "negotiation", position: 4, outcome: "open"},
		{stage: "won", position: 5, outcome: "won"},
		{stage: "lost", position: 6, outcome: "lost"},
		{stage: "on_hold", position: 7, outcome: "on_hold"},
	}
	for _, legacyStage := range legacyStages {
		if _, errorValue := database.Exec(`
INSERT INTO pipeline_stage(pipeline, stage, position, outcome)
VALUES(?, ?, ?, ?)`, "sales", legacyStage.stage, legacyStage.position, legacyStage.outcome); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := database.Exec(`
INSERT INTO account (
	id, name, status, tags, importance, owner_person_id,
	created_at, created_by_person_id, updated_at, updated_by_person_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"account-legacy", "레거시 관계처", "active", "[]", "medium", "person-owner",
		validTime, "person-owner", validTime, "person-owner"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(`
INSERT INTO opportunity (
	id, account_id, name, pipeline, stage, stage_position, stage_changed_at,
	owner_person_id, currency_code, importance,
	created_at, created_by_person_id, updated_at, updated_by_person_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"opportunity-legacy", "account-legacy", "레거시 진행 건", "sales", "proposal", 1024.0, validTime,
		"person-owner", "KRW", "medium", validTime, "person-owner", validTime, "person-owner"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := ensureCRMSchema(ctx, database); errorValue != nil {
		t.Fatal(errorValue)
	}

	var migratedStage string
	if errorValue := database.QueryRow("SELECT stage FROM opportunity WHERE id = ?", "opportunity-legacy").Scan(&migratedStage); errorValue != nil {
		t.Fatal(errorValue)
	}
	if migratedStage != "in_progress" {
		t.Fatalf("migrated opportunity stage = %q, want in_progress", migratedStage)
	}

	var legacyStageCount int
	if errorValue := database.QueryRow(`
SELECT COUNT(*) FROM pipeline_stage
WHERE pipeline = ? AND stage NOT IN ('waiting', 'in_progress', 'review', 'done', 'on_hold', 'lost')`, "sales").Scan(&legacyStageCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if legacyStageCount != 0 {
		t.Fatalf("legacy pipeline_stage rows remaining = %d, want 0", legacyStageCount)
	}

	var uniformStageCount int
	if errorValue := database.QueryRow("SELECT COUNT(*) FROM pipeline_stage WHERE pipeline = ?", "sales").Scan(&uniformStageCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if uniformStageCount != 6 {
		t.Fatalf("sales pipeline_stage row count = %d, want 6", uniformStageCount)
	}

	var stageChangeActivityCount int
	if errorValue := database.QueryRow("SELECT COUNT(*) FROM activity WHERE kind = 'stage_change'").Scan(&stageChangeActivityCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if stageChangeActivityCount != 0 {
		t.Fatalf("stage_change activities created by migration = %d, want 0", stageChangeActivityCount)
	}
}

const legacyCRMOpportunityTableWithLostReasonForeignKey = `CREATE TABLE opportunity (
	id TEXT PRIMARY KEY CHECK(trim(id) <> ''),
	account_id TEXT,
	business TEXT,
	name TEXT NOT NULL CHECK(trim(name) <> ''),
	pipeline TEXT NOT NULL,
	stage TEXT NOT NULL,
	stage_position REAL NOT NULL,
	stage_changed_at TEXT NOT NULL CHECK(stage_changed_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
	owner_person_id TEXT NOT NULL CHECK(trim(owner_person_id) <> ''),
	owner_circle_id TEXT,
	amount_minor INTEGER CHECK(amount_minor IS NULL OR amount_minor >= 0),
	currency_code TEXT NOT NULL CHECK(currency_code IN ('KRW', 'USD', 'JPY', 'EUR')),
	base_amount_minor INTEGER CHECK(base_amount_minor IS NULL OR base_amount_minor >= 0),
	base_currency_code TEXT CHECK(base_currency_code IS NULL OR base_currency_code IN ('KRW', 'USD', 'JPY', 'EUR')),
	importance TEXT NOT NULL CHECK(importance IN ('high', 'medium', 'low')),
	due_at TEXT CHECK(due_at IS NULL OR due_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
	due_time_zone TEXT,
	lost_reason TEXT,
	description TEXT,
	created_at TEXT NOT NULL CHECK(created_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
	created_by_person_id TEXT NOT NULL CHECK(trim(created_by_person_id) <> ''),
	updated_at TEXT NOT NULL CHECK(updated_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
	updated_by_person_id TEXT NOT NULL CHECK(trim(updated_by_person_id) <> ''),
	archived_at TEXT CHECK(archived_at IS NULL OR archived_at GLOB '[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]T[0-9][0-9]:[0-9][0-9]:[0-9][0-9]Z'),
	archived_by_person_id TEXT,
	CHECK((due_at IS NULL) = (due_time_zone IS NULL)),
	CHECK((base_amount_minor IS NULL) = (base_currency_code IS NULL)),
	CHECK((archived_at IS NULL) = (archived_by_person_id IS NULL)),
	FOREIGN KEY (account_id) REFERENCES account(id) ON DELETE RESTRICT,
	FOREIGN KEY (pipeline, stage) REFERENCES pipeline_stage(pipeline, stage) ON DELETE RESTRICT,
	FOREIGN KEY (lost_reason) REFERENCES lost_reason(reason) ON DELETE RESTRICT
)`

func TestCRMSchemaMigratesOpportunityAwayFromLostReasonForeignKey(t *testing.T) {
	ctx := context.Background()
	stateDirectory := t.TempDir()
	databasePath := filepath.Join(stateDirectory, "crm.sqlite")
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(databasePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	configureSQLiteDatabase(database)
	if errorValue := configureSQLiteConnection(ctx, database); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, statement := range crmSchemaStatements {
		if _, errorValue := database.Exec(statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := database.Exec("DROP TABLE opportunity"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(legacyCRMOpportunityTableWithLostReasonForeignKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, statement := range crmSchemaStatements {
		if _, errorValue := database.Exec(statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	validTime := "2026-08-02T00:00:00Z"
	if _, errorValue := database.Exec(`INSERT INTO pipeline(pipeline, label, direction, is_active) VALUES(?, ?, ?, 1)`,
		"sales", "영업", "inbound"); errorValue != nil {
		t.Fatal(errorValue)
	}
	legacyStages := []struct {
		stage    string
		position int
		outcome  string
	}{
		{stage: "waiting", position: 1, outcome: "open"},
		{stage: "in_progress", position: 2, outcome: "open"},
		{stage: "review", position: 3, outcome: "open"},
		{stage: "done", position: 4, outcome: "won"},
		{stage: "on_hold", position: 5, outcome: "on_hold"},
		{stage: "lost", position: 6, outcome: "lost"},
	}
	for _, stage := range legacyStages {
		if _, errorValue := database.Exec(`
INSERT INTO pipeline_stage(pipeline, stage, position, outcome)
VALUES(?, ?, ?, ?)`, "sales", stage.stage, stage.position, stage.outcome); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := database.Exec(`
INSERT INTO account (
	id, name, status, tags, importance, owner_person_id,
	created_at, created_by_person_id, updated_at, updated_by_person_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"account-fk-legacy", "FK 레거시 관계처", "active", "[]", "medium", "person-owner",
		validTime, "person-owner", validTime, "person-owner"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.Exec(`
INSERT INTO opportunity (
	id, account_id, name, pipeline, stage, stage_position, stage_changed_at,
	owner_person_id, currency_code, importance,
	created_at, created_by_person_id, updated_at, updated_by_person_id
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"opportunity-fk-legacy", "account-fk-legacy", "FK 레거시 진행 건", "sales", "waiting", 1024.0, validTime,
		"person-owner", "KRW", "medium", validTime, "person-owner", validTime, "person-owner"); errorValue != nil {
		t.Fatal(errorValue)
	}

	var preMigrationCreateStatement string
	if errorValue := database.QueryRow(`
SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'opportunity'`).Scan(&preMigrationCreateStatement); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(preMigrationCreateStatement, "REFERENCES lost_reason(reason)") {
		t.Fatal("test setup did not produce a legacy opportunity table with the lost_reason foreign key")
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	service := &Service{Configuration: Configuration{StateDirectory: stateDirectory}}

	rejectedTransitionError := service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID: "opportunity-fk-legacy",
		Stage:         "lost",
		StagePosition: 2048,
		OccurredAt:    "2026-08-02T05:00:00Z",
		ActorPersonID: "person-owner",
		LostReason:    "   ",
	})
	if rejectedTransitionError == nil {
		t.Fatal("lost transition without a reason should fail after migration")
	}
	afterRejection, _, errorValue := service.readCRMOpportunity(ctx, "opportunity-fk-legacy", false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if afterRejection.Stage != "waiting" {
		t.Fatalf("stage after rejected lost transition = %q, want waiting", afterRejection.Stage)
	}

	freeTextReason := "고객 예산 축소로 계약 보류"
	if errorValue := service.transitionCRMOpportunityStage(ctx, crmOpportunityStageTransition{
		OpportunityID: "opportunity-fk-legacy",
		Stage:         "lost",
		StagePosition: 2048,
		OccurredAt:    "2026-08-02T05:05:00Z",
		ActorPersonID: "person-owner",
		LostReason:    "  " + freeTextReason + "  ",
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	afterFreeTextTransition, _, errorValue := service.readCRMOpportunity(ctx, "opportunity-fk-legacy", false)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if afterFreeTextTransition.Stage != "lost" || afterFreeTextTransition.LostReason != freeTextReason {
		t.Fatalf("opportunity after free-text lost transition = %#v, want lost reason %q", afterFreeTextTransition, freeTextReason)
	}

	migratedDatabase, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer migratedDatabase.Close()

	var postMigrationCreateStatement string
	if errorValue := migratedDatabase.QueryRow(`
SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'opportunity'`).Scan(&postMigrationCreateStatement); errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(postMigrationCreateStatement, "REFERENCES lost_reason(reason)") {
		t.Fatalf("opportunity table definition after migration still has the lost_reason foreign key: %s", postMigrationCreateStatement)
	}
	if !strings.Contains(postMigrationCreateStatement, "lost_reason TEXT") {
		t.Fatalf("opportunity table definition after migration lost the lost_reason column: %s", postMigrationCreateStatement)
	}

	var indexCount int
	if errorValue := migratedDatabase.QueryRow(`
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'index' AND name = 'opportunity_stage_position'`).Scan(&indexCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if indexCount != 1 {
		t.Fatalf("opportunity_stage_position index count after rebuild = %d, want 1", indexCount)
	}
	expectedTriggers := []string{
		"opportunity_validate_insert",
		"opportunity_validate_update",
		"opportunity_realized_amount_immutable",
		"opportunity_validate_contacts_after_insert",
		"opportunity_validate_contacts_after_update",
	}
	for _, triggerName := range expectedTriggers {
		var triggerCount int
		if errorValue := migratedDatabase.QueryRow(`
SELECT COUNT(*) FROM sqlite_master
WHERE type = 'trigger' AND name = ?`, triggerName).Scan(&triggerCount); errorValue != nil {
			t.Fatal(errorValue)
		}
		if triggerCount != 1 {
			t.Fatalf("trigger %s count after rebuild = %d, want 1", triggerName, triggerCount)
		}
	}

	violationRows, errorValue := migratedDatabase.Query("PRAGMA foreign_key_check")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	foreignKeyViolationCount := 0
	for violationRows.Next() {
		foreignKeyViolationCount++
	}
	if errorValue := violationRows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := violationRows.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if foreignKeyViolationCount != 0 {
		t.Fatalf("foreign_key_check violations after rebuild = %d, want 0", foreignKeyViolationCount)
	}

	var foreignKeysSetting int
	if errorValue := migratedDatabase.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeysSetting); errorValue != nil {
		t.Fatal(errorValue)
	}
	if foreignKeysSetting != 1 {
		t.Fatalf("foreign_keys pragma after rebuild = %d, want 1 (re-enabled)", foreignKeysSetting)
	}

	secondService := &Service{Configuration: Configuration{StateDirectory: stateDirectory}}
	if _, _, errorValue := secondService.readCRMOpportunity(ctx, "opportunity-fk-legacy", false); errorValue != nil {
		t.Fatalf("re-opening the already-migrated CRM database should succeed idempotently: %v", errorValue)
	}
}

func assertCRMRowCount(t *testing.T, database *sql.DB, tableName string, expected int) {
	t.Helper()
	var count int
	var errorValue error
	switch tableName {
	case "pipeline":
		errorValue = database.QueryRow("SELECT COUNT(*) FROM pipeline").Scan(&count)
	case "pipeline_stage":
		errorValue = database.QueryRow("SELECT COUNT(*) FROM pipeline_stage").Scan(&count)
	case "lost_reason":
		errorValue = database.QueryRow("SELECT COUNT(*) FROM lost_reason").Scan(&count)
	default:
		t.Fatalf("unsupported CRM count table %q", tableName)
	}
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if count != expected {
		t.Fatalf("%s row count = %d, want %d", tableName, count, expected)
	}
}
