package admind

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
)

const orgchartProfilesColumnsSQL = `(
	profile_key TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	email TEXT NOT NULL,
	job_title TEXT NOT NULL,
	position_level INTEGER NOT NULL CHECK(position_level >= 0),
	primary_group_id TEXT NOT NULL,
	group_ids TEXT NOT NULL,
	supervisor_id TEXT NOT NULL,
	project_ids TEXT NOT NULL,
	team_role TEXT NOT NULL,
	employment_status TEXT NOT NULL CHECK(employment_status IN ('active', 'leave', 'resigned')),
	is_orgchart_visible INTEGER NOT NULL CHECK(is_orgchart_visible IN (0, 1)),
	updated_at TEXT NOT NULL
)`

type orgchartSchemaExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (service *Service) openOrgchartDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openSQLiteDatabase(ctx, service.orgchartDatabasePath(), ensureOrgchartSchema)
}

func (service *Service) orgchartDatabasePath() string {
	return filepath.Join(service.Configuration.StateDirectory, "orgchart.sqlite")
}

func ensureOrgchartSchema(ctx context.Context, database *sql.DB) error {
	if errorValue := createOrgchartProfilesTable(ctx, database, "IF NOT EXISTS orgchart_profiles"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureOrgchartProfileConstraints(ctx, database); errorValue != nil {
		return errorValue
	}
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS orgchart_groups (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	position INTEGER NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS orgchart_metadata (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	return ensureOrgchartPeopleCacheSchema(ctx, database)
}

func createOrgchartProfilesTable(ctx context.Context, executor orgchartSchemaExecutor, tableName string) error {
	_, errorValue := executor.ExecContext(ctx, "CREATE TABLE "+tableName+" "+orgchartProfilesColumnsSQL)
	return errorValue
}

func ensureOrgchartProfileConstraints(ctx context.Context, database *sql.DB) error {
	schema, errorValue := readSQLiteTableSchema(ctx, database, "orgchart_profiles")
	if errorValue != nil {
		return errorValue
	}
	if hasOrgchartProfileConstraints(schema) {
		return nil
	}
	return migrateOrgchartProfilesSchema(ctx, database)
}

func readSQLiteTableSchema(ctx context.Context, database *sql.DB, tableName string) (string, error) {
	var schema string
	errorValue := database.QueryRowContext(ctx, `
SELECT sql
FROM sqlite_master
WHERE type = 'table'
	AND name = ?`, tableName).Scan(&schema)
	if errorValue == sql.ErrNoRows {
		return "", nil
	}
	return schema, errorValue
}

func hasOrgchartProfileConstraints(schema string) bool {
	normalizedSchema := strings.ReplaceAll(strings.ToLower(schema), " ", "")
	return strings.Contains(normalizedSchema, "check(position_level>=0)") &&
		strings.Contains(normalizedSchema, "check(employment_statusin('active','leave','resigned'))") &&
		strings.Contains(normalizedSchema, "check(is_orgchart_visiblein(0,1))")
}

func migrateOrgchartProfilesSchema(ctx context.Context, database *sql.DB) error {
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := renameLegacyOrgchartProfiles(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := createOrgchartProfilesTable(ctx, transaction, "orgchart_profiles"); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := copyLegacyOrgchartProfiles(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, "DROP TABLE orgchart_profiles_legacy"); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func renameLegacyOrgchartProfiles(ctx context.Context, transaction *sql.Tx) error {
	if _, errorValue := transaction.ExecContext(ctx, "DROP TABLE IF EXISTS orgchart_profiles_legacy"); errorValue != nil {
		return errorValue
	}
	_, errorValue := transaction.ExecContext(ctx, "ALTER TABLE orgchart_profiles RENAME TO orgchart_profiles_legacy")
	return errorValue
}

func copyLegacyOrgchartProfiles(ctx context.Context, transaction *sql.Tx) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO orgchart_profiles(
	profile_key,
	user_id,
	email,
	job_title,
	position_level,
	primary_group_id,
	group_ids,
	supervisor_id,
	project_ids,
	team_role,
	employment_status,
	is_orgchart_visible,
	updated_at
)
SELECT
	profile_key,
	trim(COALESCE(user_id, '')),
	lower(trim(COALESCE(email, ''))),
	trim(COALESCE(job_title, '')),
	CASE
		WHEN COALESCE(position_level, 0) < 0 THEN 0
		ELSE COALESCE(position_level, 0)
	END,
	trim(COALESCE(primary_group_id, '')),
	COALESCE(group_ids, '[]'),
	trim(COALESCE(supervisor_id, '')),
	COALESCE(project_ids, '[]'),
	trim(COALESCE(team_role, '')),
	CASE
		WHEN lower(trim(COALESCE(employment_status, ''))) IN ('active', 'leave', 'resigned') THEN lower(trim(employment_status))
		ELSE 'active'
	END,
	CASE
		WHEN COALESCE(is_orgchart_visible, 1) = 0 THEN 0
		ELSE 1
	END,
	trim(COALESCE(updated_at, ''))
FROM orgchart_profiles_legacy
WHERE trim(COALESCE(profile_key, '')) != ''`)
	return errorValue
}
