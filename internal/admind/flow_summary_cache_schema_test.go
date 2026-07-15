package admind

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureFlowSummaryCacheSchemaCreatesExactTables(t *testing.T) {
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	database, errorValue := service.openSQLiteDatabase(context.Background(), service.Configuration.FlowDatabasePath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if errorValue := ensureFlowSummaryCacheSchema(context.Background(), database); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFlowSummaryCacheTableShape(t, database, "flow_summary_source_revisions", "source_kind,source_key,revision", "source_kind,source_key")
	assertFlowSummaryCacheTableShape(t, database, "flow_summary_cache_entries", "week_code,requested_week_revision,previous_week_revision,current_month_revision,previous_month_revision,definitions_revision,member_fingerprint,schema_version,payload_json,cached_at", "week_code")
}

func TestEnsureFlowSummaryCacheSchemaPreservesLegacyFlowData(t *testing.T) {
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "legacy-flow.sqlite")})
	database, errorValue := service.openSQLiteDatabase(context.Background(), service.Configuration.FlowDatabasePath, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, statement := range []string{
		"CREATE TABLE flow_tasks (id TEXT PRIMARY KEY, content TEXT NOT NULL)",
		"CREATE TABLE flow_definitions (kind TEXT NOT NULL, value TEXT NOT NULL, position INTEGER NOT NULL, PRIMARY KEY(kind, value))",
		"INSERT INTO flow_tasks(id, content) VALUES ('legacy-task', 'keep task')",
		"INSERT INTO flow_definitions(kind, value, position) VALUES ('type', '회의', 0)",
	} {
		if _, errorValue := database.ExecContext(context.Background(), statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if errorValue := ensureFlowSummaryCacheSchema(context.Background(), database); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFlowSummaryCacheTableShape(t, database, "flow_summary_source_revisions", "source_kind,source_key,revision", "source_kind,source_key")
	assertFlowSummaryCacheTableShape(t, database, "flow_summary_cache_entries", "week_code,requested_week_revision,previous_week_revision,current_month_revision,previous_month_revision,definitions_revision,member_fingerprint,schema_version,payload_json,cached_at", "week_code")
	var taskContent string
	if errorValue := database.QueryRowContext(context.Background(), "SELECT content FROM flow_tasks WHERE id = 'legacy-task'").Scan(&taskContent); errorValue != nil {
		t.Fatal(errorValue)
	}
	var definitionValue string
	if errorValue := database.QueryRowContext(context.Background(), "SELECT value FROM flow_definitions WHERE kind = 'type'").Scan(&definitionValue); errorValue != nil {
		t.Fatal(errorValue)
	}
	if taskContent != "keep task" || definitionValue != "회의" {
		t.Fatalf("legacy task = %q, definition = %q", taskContent, definitionValue)
	}
}

func assertFlowSummaryCacheTableShape(t *testing.T, database *sql.DB, tableName string, expectedColumns string, expectedPrimaryKey string) {
	t.Helper()
	rows, errorValue := database.QueryContext(context.Background(), "SELECT name, pk FROM pragma_table_info(?) ORDER BY cid", tableName)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	columns := []string{}
	primaryKeyColumns := map[int]string{}
	for rows.Next() {
		var columnName string
		var primaryKeyPosition int
		if errorValue := rows.Scan(&columnName, &primaryKeyPosition); errorValue != nil {
			t.Fatal(errorValue)
		}
		columns = append(columns, columnName)
		if primaryKeyPosition > 0 {
			primaryKeyColumns[primaryKeyPosition] = columnName
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	primaryKey := make([]string, 0, len(primaryKeyColumns))
	for position := 1; position <= len(primaryKeyColumns); position++ {
		primaryKey = append(primaryKey, primaryKeyColumns[position])
	}
	if strings.Join(columns, ",") != expectedColumns || strings.Join(primaryKey, ",") != expectedPrimaryKey {
		t.Fatalf("table %s columns = %q primary key = %q", tableName, strings.Join(columns, ","), strings.Join(primaryKey, ","))
	}
}
