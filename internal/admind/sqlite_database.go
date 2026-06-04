package admind

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

type adminDatabaseSchemas struct {
	mutex sync.Mutex
	ready map[string]bool
}

func newAdminDatabaseSchemas() *adminDatabaseSchemas {
	return &adminDatabaseSchemas{ready: map[string]bool{}}
}

func (service *Service) openSQLiteDatabase(ctx context.Context, databasePath string, ensureSchema func(context.Context, *sql.DB) error) (*sql.DB, error) {
	if errorValue := os.MkdirAll(filepath.Dir(databasePath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(databasePath))
	if errorValue != nil {
		return nil, errorValue
	}
	configureSQLiteDatabase(database)
	if errorValue := configureSQLiteConnection(ctx, database); errorValue != nil {
		_ = database.Close()
		return nil, errorValue
	}
	if errorValue := service.ensureSQLiteSchema(ctx, databasePath, database, ensureSchema); errorValue != nil {
		_ = database.Close()
		return nil, errorValue
	}
	return database, nil
}

func sqliteDatabaseDSN(databasePath string) string {
	separator := "?"
	if strings.Contains(databasePath, "?") {
		separator = "&"
	}
	return databasePath + separator + "_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(on)"
}

func configureSQLiteDatabase(database *sql.DB) {
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
}

func configureSQLiteConnection(ctx context.Context, database *sql.DB) error {
	if _, errorValue := database.ExecContext(ctx, "PRAGMA journal_mode=WAL"); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, "PRAGMA busy_timeout=5000"); errorValue != nil {
		return errorValue
	}
	_, errorValue := database.ExecContext(ctx, "PRAGMA foreign_keys=ON")
	return errorValue
}

func (service *Service) ensureSQLiteSchema(ctx context.Context, databasePath string, database *sql.DB, ensureSchema func(context.Context, *sql.DB) error) error {
	if ensureSchema == nil {
		return nil
	}
	schemas := service.databaseSchemaState()
	schemas.mutex.Lock()
	defer schemas.mutex.Unlock()
	if schemas.ready[databasePath] {
		return nil
	}
	if errorValue := ensureSchema(ctx, database); errorValue != nil {
		return errorValue
	}
	schemas.ready[databasePath] = true
	return nil
}

func (service *Service) databaseSchemaState() *adminDatabaseSchemas {
	if service.databaseSchemas != nil {
		return service.databaseSchemas
	}
	service.databaseSchemas = newAdminDatabaseSchemas()
	return service.databaseSchemas
}
