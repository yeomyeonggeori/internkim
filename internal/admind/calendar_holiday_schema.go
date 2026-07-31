package admind

import (
	"context"
	"database/sql"
)

func ensureCalendarHolidaySchema(ctx context.Context, database *sql.DB) error {
	if _, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_company_holidays (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	holiday_date TEXT NOT NULL,
	recurs_annually INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS calendar_company_holidays_date_idx
ON calendar_company_holidays(holiday_date)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_holidays (
	source TEXT NOT NULL,
	source_key TEXT NOT NULL,
	external_id TEXT NOT NULL,
	country_code TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	holiday_date TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	PRIMARY KEY(source, source_key, external_id)
)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS calendar_holidays_source_date_idx
ON calendar_holidays(source, holiday_date)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE INDEX IF NOT EXISTS calendar_holidays_country_date_idx
ON calendar_holidays(country_code, holiday_date)`); errorValue != nil {
		return errorValue
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_holiday_countries (
	country_code TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	fetched_at TEXT NOT NULL
)`); errorValue != nil {
		return errorValue
	}
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS calendar_holiday_sources (
	provider TEXT NOT NULL,
	source_key TEXT NOT NULL,
	last_synced_at TEXT NOT NULL DEFAULT '',
	last_error TEXT NOT NULL DEFAULT '',
	PRIMARY KEY(provider, source_key)
)`)
	return errorValue
}
