package admind

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestStateDatabaseAdoptsLegacyDatabasesOnce(t *testing.T) {
	stateDirectory := t.TempDir()
	legacyFlowPath := filepath.Join(stateDirectory, "flow.sqlite")
	legacyCalendarPath := filepath.Join(stateDirectory, "calendar.sqlite")
	seedLegacyStateDatabase(t, legacyFlowPath, "CREATE TABLE flow_definitions (kind TEXT NOT NULL, value TEXT NOT NULL, position INTEGER NOT NULL, color TEXT NOT NULL DEFAULT '', PRIMARY KEY(kind, value))",
		"INSERT INTO flow_definitions(kind, value, position, color) VALUES ('category', '여명거리', 0, '#db2777')")
	seedLegacyStateDatabase(t, legacyCalendarPath, "CREATE TABLE calendar_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"INSERT INTO calendar_settings(key, value) VALUES ('ics_token', 'legacy-token')")

	service := NewService(Configuration{
		StateDirectory:       stateDirectory,
		FlowDatabasePath:     legacyFlowPath,
		CalendarDatabasePath: legacyCalendarPath,
	})
	definitions, errorValue := service.readFlowDefinitions(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(definitions.Categories) != 1 || definitions.Categories[0] != "여명거리" {
		t.Fatalf("categories = %#v", definitions.Categories)
	}
	if definitions.CategoryColors["여명거리"] != "#db2777" {
		t.Fatalf("categoryColors = %#v", definitions.CategoryColors)
	}

	unifiedPath := service.stateDatabasePath()
	if unifiedPath != filepath.Join(stateDirectory, stateDatabaseFileName) {
		t.Fatalf("unified path = %s", unifiedPath)
	}
	if _, errorValue := os.Stat(unifiedPath); errorValue != nil {
		t.Fatalf("unified database missing: %v", errorValue)
	}
	for _, legacyPath := range []string{legacyFlowPath, legacyCalendarPath} {
		if _, errorValue := os.Stat(legacyPath); !os.IsNotExist(errorValue) {
			t.Fatalf("legacy database still in place: %s", legacyPath)
		}
		if _, errorValue := os.Stat(legacyPath + ".migrated"); errorValue != nil {
			t.Fatalf("legacy database was not kept as a backup: %v", errorValue)
		}
	}

	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var storedToken string
	if errorValue := database.QueryRowContext(context.Background(), "SELECT value FROM calendar_settings WHERE key = 'ics_token'").Scan(&storedToken); errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedToken != "legacy-token" {
		t.Fatalf("calendar setting = %q", storedToken)
	}
}

func seedLegacyStateDatabase(t *testing.T, databasePath string, statements ...string) {
	t.Helper()
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(databasePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	for _, statement := range statements {
		if _, errorValue := database.ExecContext(context.Background(), statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}
