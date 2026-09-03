package admind

import (
	"context"
	"database/sql"
	"log/slog"
)

// The sync tables #1347 stopped creating. Nothing has written one since, so
// there is nothing in them to carry anywhere.
var orphanedCalendarSyncTables = []string{
	"calendar_remote_accounts",
	"calendar_sync_state",
	"calendar_outbox",
	"calendar_remote_event_sync_state",
	"calendar_push_observation_fences",
	"calendar_conflicts",
	"calendar_target_field_acknowledgements",
	"calendar_event_field_clocks",
}

// The device's own calendar. These hold what a device recorded before the
// company did, so they are dropped only once the record holds all of it.
var retiredCalendarStoreTables = []string{
	"calendar_company_holidays",
	"calendar_holidays",
	"calendar_holiday_countries",
	"calendar_holiday_sources",
	"calendar_events",
	"calendar_event_participants",
	"calendar_event_notifications",
	"calendar_event_logical_clocks",
	"calendar_event_mutation_origins",
	"calendar_event_window_cache_entries",
	"calendar_event_window_source_state",
	"calendar_channel_outbox",
	"calendar_delete_intents",
	"calendar_properties",
	"calendar_settings",
	"calendar_carried_events",
}

func (service *Service) openCalendarDatabase(ctx context.Context) (*sql.DB, error) {
	options := sqliteDatabaseOptions{transactionLock: "immediate"}
	return service.openStateDatabase(ctx, "calendar", ensureCalendarSchema, options)
}

func ensureCalendarSchema(ctx context.Context, database *sql.DB) error {
	return dropCalendarTables(ctx, database, orphanedCalendarSyncTables)
}

func (service *Service) startCalendarSweep(ctx context.Context) {
	go service.sweepTheCalendarTheCompanyNowHolds(ctx)
}

func (service *Service) sweepTheCalendarTheCompanyNowHolds(ctx context.Context) {
	uncovered, errorValue := service.calendarEventsTheRecordDoesNotHold(ctx)
	if errorValue != nil {
		slog.WarnContext(ctx, "the calendar this device still holds could not be counted, so none of it was let go",
			"error", errorValue)
		return
	}
	if uncovered > 0 {
		slog.WarnContext(ctx, "this device holds calendar events the record does not, so its calendar tables stay",
			"uncovered", uncovered, "recovery_action", calendarCarryRecoveryAction)
		return
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		slog.WarnContext(ctx, "the calendar database could not be opened to let go of what the company holds",
			"error", errorValue)
		return
	}
	defer database.Close()
	if errorValue := dropCalendarTables(ctx, database, retiredCalendarStoreTables); errorValue != nil {
		slog.WarnContext(ctx, "a calendar table the company now holds could not be dropped", "error", errorValue)
	}
}

func dropCalendarTables(ctx context.Context, database *sql.DB, tableNames []string) error {
	for _, tableName := range tableNames {
		rowCount, held := countRowsInCalendarTable(ctx, database, tableName)
		if !held {
			continue
		}
		if _, errorValue := database.ExecContext(ctx, "DROP TABLE IF EXISTS "+tableName); errorValue != nil {
			return errorValue
		}
		slog.InfoContext(ctx, "dropped a calendar table the company now holds",
			"table", tableName, "rows", rowCount)
	}
	return nil
}

func countRowsInCalendarTable(ctx context.Context, database *sql.DB, tableName string) (int, bool) {
	var rowCount int
	errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+tableName).Scan(&rowCount)
	if errorValue != nil {
		return 0, false
	}
	return rowCount, true
}
