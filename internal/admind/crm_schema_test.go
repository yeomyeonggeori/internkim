package admind

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
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
	assertCRMRowCount(t, database, "pipeline_stage", 44)
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
