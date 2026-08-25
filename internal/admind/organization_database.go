package admind

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
)

const organizationProfilesColumnsSQL = `(
	profile_key TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	email TEXT NOT NULL,
	job_title TEXT NOT NULL,
	group_id TEXT NOT NULL,
	phone_number TEXT NOT NULL,
	hire_date TEXT NOT NULL,
	supervisor_id TEXT NOT NULL,
	status TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`

type organizationSchemaExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (service *Service) openOrganizationDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openStateDatabase(ctx, "organization", ensureOrganizationSchema, sqliteDatabaseOptions{})
}

func (service *Service) organizationDatabasePath() string {
	return filepath.Join(service.Configuration.StateDirectory, "organization.sqlite")
}

func ensureOrganizationSchema(ctx context.Context, database *sql.DB) error {
	if errorValue := createOrganizationProfilesTable(ctx, database, "IF NOT EXISTS organization_profiles"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureOrganizationProfileConstraints(ctx, database); errorValue != nil {
		return errorValue
	}
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS organization_groups (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	parent_id TEXT NOT NULL DEFAULT '',
	position INTEGER NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureOrganizationGroupParentColumn(ctx, database); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS organization_metadata (
	key TEXT PRIMARY KEY,
	value TEXT NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	return ensureOrganizationPeopleCacheSchema(ctx, database)
}

func ensureOrganizationGroupParentColumn(ctx context.Context, database *sql.DB) error {
	schema, errorValue := readSQLiteTableSchema(ctx, database, "organization_groups")
	if errorValue != nil {
		return errorValue
	}
	if strings.Contains(strings.ToLower(schema), "parent_id") {
		return nil
	}
	_, errorValue = database.ExecContext(ctx, "ALTER TABLE organization_groups ADD COLUMN parent_id TEXT NOT NULL DEFAULT ''")
	return errorValue
}

func createOrganizationProfilesTable(ctx context.Context, executor organizationSchemaExecutor, tableName string) error {
	_, errorValue := executor.ExecContext(ctx, "CREATE TABLE "+tableName+" "+organizationProfilesColumnsSQL)
	return errorValue
}

func ensureOrganizationProfileConstraints(ctx context.Context, database *sql.DB) error {
	schema, errorValue := readSQLiteTableSchema(ctx, database, "organization_profiles")
	if errorValue != nil {
		return errorValue
	}
	if hasOrganizationProfileConstraints(schema) {
		return nil
	}
	return migrateOrganizationProfilesSchema(ctx, database)
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

func hasOrganizationProfileConstraints(schema string) bool {
	normalizedSchema := strings.ReplaceAll(strings.ToLower(schema), " ", "")
	return strings.Contains(normalizedSchema, "group_idtextnotnull") &&
		strings.Contains(normalizedSchema, "phone_numbertextnotnull") &&
		strings.Contains(normalizedSchema, "hire_datetextnotnull") &&
		strings.Contains(normalizedSchema, "statustextnotnull") &&
		!strings.Contains(normalizedSchema, "group_idstextnotnull") &&
		!strings.Contains(normalizedSchema, "position_level") &&
		!strings.Contains(normalizedSchema, "project_ids") &&
		!strings.Contains(normalizedSchema, "team_role") &&
		!strings.Contains(normalizedSchema, "employment_status") &&
		!strings.Contains(normalizedSchema, "is_organization_visible")
}

func migrateOrganizationProfilesSchema(ctx context.Context, database *sql.DB) error {
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := renameLegacyOrganizationProfiles(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := createOrganizationProfilesTable(ctx, transaction, "organization_profiles"); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := copyLegacyOrganizationProfiles(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, "DROP TABLE organization_profiles_legacy"); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func renameLegacyOrganizationProfiles(ctx context.Context, transaction *sql.Tx) error {
	if _, errorValue := transaction.ExecContext(ctx, "DROP TABLE IF EXISTS organization_profiles_legacy"); errorValue != nil {
		return errorValue
	}
	_, errorValue := transaction.ExecContext(ctx, "ALTER TABLE organization_profiles RENAME TO organization_profiles_legacy")
	return errorValue
}

func copyLegacyOrganizationProfiles(ctx context.Context, transaction *sql.Tx) error {
	legacySchema, errorValue := readSQLiteTransactionTableSchema(ctx, transaction, "organization_profiles_legacy")
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = transaction.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO organization_profiles(
	profile_key,
	user_id,
	email,
	job_title,
	group_id,
	phone_number,
	hire_date,
	supervisor_id,
	status,
	updated_at
)
SELECT
	profile_key,
	trim(COALESCE(user_id, '')),
	lower(trim(COALESCE(email, ''))),
	trim(COALESCE(job_title, '')),
	%s,
	%s,
	%s,
	trim(COALESCE(supervisor_id, '')),
	%s,
	trim(COALESCE(updated_at, ''))
FROM organization_profiles_legacy
WHERE trim(COALESCE(profile_key, '')) != ''`, legacyOrganizationGroupIDExpression(legacySchema), legacyOrganizationColumnExpression(legacySchema, "phone_number"), legacyOrganizationColumnExpression(legacySchema, "hire_date"), legacyOrganizationStatusExpression(legacySchema)))
	return errorValue
}

func legacyOrganizationStatusExpression(legacySchema string) string {
	normalizedSchema := strings.ToLower(legacySchema)
	if strings.Contains(normalizedSchema, "status") && !strings.Contains(normalizedSchema, "employment_status") {
		return "trim(COALESCE(status, ''))"
	}
	if !strings.Contains(normalizedSchema, "employment_status") {
		return "'active'"
	}
	return `CASE WHEN lower(trim(COALESCE(employment_status, ''))) = 'resigned' THEN 'departed' ELSE 'active' END`
}

func legacyOrganizationColumnExpression(legacySchema string, columnName string) string {
	if !strings.Contains(strings.ToLower(legacySchema), columnName) {
		return "''"
	}
	return "trim(COALESCE(" + columnName + ", ''))"
}

func legacyOrganizationGroupIDExpression(legacySchema string) string {
	normalizedSchema := strings.ReplaceAll(strings.ToLower(legacySchema), " ", "")
	if strings.Contains(normalizedSchema, "primary_group_id") {
		return "trim(COALESCE(primary_group_id, ''))"
	}
	if strings.Contains(normalizedSchema, "group_idtext") {
		return "trim(COALESCE(group_id, ''))"
	}
	return "''"
}

func readSQLiteTransactionTableSchema(ctx context.Context, transaction *sql.Tx, tableName string) (string, error) {
	var schema string
	errorValue := transaction.QueryRowContext(ctx, `
SELECT sql
FROM sqlite_master
WHERE type = 'table'
	AND name = ?`, tableName).Scan(&schema)
	if errorValue == sql.ErrNoRows {
		return "", nil
	}
	return schema, errorValue
}
