package admind

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const stateDatabaseFileName = "internkim.sqlite"

type legacyStateDatabase struct {
	name string
	path string
}

func (service *Service) stateDatabasePath() string {
	if configuredPath := strings.TrimSpace(service.Configuration.DatabasePath); configuredPath != "" {
		return configuredPath
	}
	return filepath.Join(service.Configuration.StateDirectory, stateDatabaseFileName)
}

func resolvedStateDatabasePath(configuration Configuration, defaultConfiguration Configuration) string {
	if configuredPath := strings.TrimSpace(configuration.DatabasePath); configuredPath != "" {
		return configuredPath
	}
	for _, providedPath := range []string{
		configuration.FlowDatabasePath,
		configuration.CalendarDatabasePath,
		configuration.MailDatabasePath,
		configuration.AttendanceDatabasePath,
		configuration.BridgeMapDatabasePath,
		configuration.CompanionJobPath,
	} {
		if strings.TrimSpace(providedPath) == "" {
			continue
		}
		return filepath.Join(filepath.Dir(providedPath), stateDatabaseFileName)
	}
	if strings.TrimSpace(configuration.StateDirectory) != "" && configuration.StateDirectory != defaultConfiguration.StateDirectory {
		return filepath.Join(configuration.StateDirectory, stateDatabaseFileName)
	}
	return filepath.Join(filepath.Dir(defaultConfiguration.FlowDatabasePath), stateDatabaseFileName)
}

func (service *Service) legacyStateDatabaseCandidates() []legacyStateDatabase {
	return []legacyStateDatabase{
		{name: "flow", path: service.Configuration.FlowDatabasePath},
		{name: "calendar", path: service.Configuration.CalendarDatabasePath},
		{name: "mail", path: service.Configuration.MailDatabasePath},
		{name: "attendance", path: service.Configuration.AttendanceDatabasePath},
		{name: "bridge-map", path: service.Configuration.BridgeMapDatabasePath},
		{name: "company", path: filepath.Join(service.Configuration.StateDirectory, "company.sqlite")},
		{name: "organization", path: filepath.Join(service.Configuration.StateDirectory, "organization.sqlite")},
	}
}

func (service *Service) openStateDatabase(ctx context.Context, schemaName string, ensureSchema func(context.Context, *sql.DB) error, options sqliteDatabaseOptions) (*sql.DB, error) {
	databasePath := service.stateDatabasePath()
	service.migrateLegacyStateDatabasesOnce(ctx, databasePath)
	return service.openSQLiteDatabaseWithSchemaName(ctx, databasePath, databasePath+"|"+schemaName, ensureSchema, options)
}

func (service *Service) migrateLegacyStateDatabasesOnce(ctx context.Context, databasePath string) {
	service.legacyDatabaseMigration.Do(func() {
		for _, legacy := range service.legacyStateDatabaseCandidates() {
			if errorValue := service.migrateLegacyStateDatabase(ctx, databasePath, legacy); errorValue != nil {
				log.Printf("state database migration failed: database=%s error=%v", legacy.name, errorValue)
			}
		}
	})
}

func (service *Service) migrateLegacyStateDatabase(ctx context.Context, databasePath string, legacy legacyStateDatabase) error {
	legacyPath := strings.TrimSpace(legacy.path)
	if legacyPath == "" || legacyPath == databasePath {
		return nil
	}
	if _, errorValue := os.Stat(legacyPath); errorValue != nil {
		return nil
	}
	database, errorValue := service.openSQLiteDatabaseWithOptions(ctx, databasePath, nil, sqliteDatabaseOptions{})
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(ctx, "ATTACH DATABASE ? AS legacy", legacyPath); errorValue != nil {
		return errorValue
	}
	defer database.ExecContext(ctx, "DETACH DATABASE legacy")
	copiedTables, errorValue := copyLegacyStateTables(ctx, database)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "DETACH DATABASE legacy"); errorValue != nil {
		return errorValue
	}
	if errorValue := archiveLegacyStateDatabase(legacyPath); errorValue != nil {
		return errorValue
	}
	log.Printf("state database migrated: database=%s tables=%d target=%s", legacy.name, copiedTables, databasePath)
	return nil
}

func copyLegacyStateTables(ctx context.Context, database *sql.DB) (int, error) {
	rows, errorValue := database.QueryContext(ctx, "SELECT name, sql FROM legacy.sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'")
	if errorValue != nil {
		return 0, errorValue
	}
	defer rows.Close()
	type legacyTable struct {
		name       string
		definition string
	}
	tables := []legacyTable{}
	for rows.Next() {
		var table legacyTable
		var definition sql.NullString
		if errorValue := rows.Scan(&table.name, &definition); errorValue != nil {
			return 0, errorValue
		}
		table.definition = definition.String
		tables = append(tables, table)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return 0, errorValue
	}
	copiedTables := 0
	for _, table := range tables {
		created, errorValue := ensureLegacyStateTable(ctx, database, table.name, table.definition)
		if errorValue != nil {
			return copiedTables, errorValue
		}
		if !created {
			continue
		}
		if _, errorValue := database.ExecContext(ctx, fmt.Sprintf("INSERT INTO main.%q SELECT * FROM legacy.%q", table.name, table.name)); errorValue != nil {
			return copiedTables, errorValue
		}
		copiedTables++
	}
	return copiedTables, nil
}

func ensureLegacyStateTable(ctx context.Context, database *sql.DB, tableName string, definition string) (bool, error) {
	var existingRowCount int
	errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM main.sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&existingRowCount)
	if errorValue != nil {
		return false, errorValue
	}
	if existingRowCount == 0 {
		if strings.TrimSpace(definition) == "" {
			return false, nil
		}
		if _, errorValue := database.ExecContext(ctx, definition); errorValue != nil {
			return false, errorValue
		}
		return true, nil
	}
	var storedRowCount int
	if errorValue := database.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM main.%q", tableName)).Scan(&storedRowCount); errorValue != nil {
		return false, errorValue
	}
	return storedRowCount == 0, nil
}

func archiveLegacyStateDatabase(legacyPath string) error {
	for _, suffix := range []string{"", "-wal", "-shm"} {
		sourcePath := legacyPath + suffix
		if _, errorValue := os.Stat(sourcePath); errorValue != nil {
			continue
		}
		if errorValue := os.Rename(sourcePath, sourcePath+".migrated"); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
