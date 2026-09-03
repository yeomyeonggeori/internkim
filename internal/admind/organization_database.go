package admind

import (
	"context"
	"database/sql"
	"log/slog"
)

var derivedOrganizationTables = []string{
	"organization_people_cache_entries",
	"organization_people_cache_states",
	"organization_metadata",
	"organization_profiles_legacy",
}

var carriedOrganizationTables = []string{
	"organization_profiles",
	"organization_groups",
	"organization_carried_rows",
}

func (service *Service) openOrganizationDatabase(ctx context.Context) (*sql.DB, error) {
	return service.openStateDatabase(ctx, "organization", ensureOrganizationSchema, sqliteDatabaseOptions{})
}

func ensureOrganizationSchema(context.Context, *sql.DB) error {
	return nil
}

func (service *Service) startOrganizationSweep(ctx context.Context) {
	go service.sweepTheOrganizationTheCompanyNowHolds(ctx)
}

func dropOrganizationTables(ctx context.Context, database *sql.DB, tableNames []string) error {
	for _, tableName := range tableNames {
		rowCount, held := countRowsInOrganizationTable(ctx, database, tableName)
		if !held {
			continue
		}
		if _, errorValue := database.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName); errorValue != nil {
			return errorValue
		}
		slog.InfoContext(ctx, "dropped an organization table the company now holds",
			"table", tableName, "rows", rowCount)
	}
	return nil
}

func countRowsInOrganizationTable(ctx context.Context, database *sql.DB, tableName string) (int, bool) {
	var rowCount int
	errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableName).Scan(&rowCount)
	if errorValue != nil {
		return 0, false
	}
	return rowCount, true
}
