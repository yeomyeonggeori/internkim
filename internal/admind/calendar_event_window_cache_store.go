package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

func readCalendarEventWindowSourceRevision(ctx context.Context, database *sql.DB) (int64, error) {
	var revision int64
	if errorValue := database.QueryRowContext(ctx, "SELECT revision FROM calendar_event_window_source_state WHERE id = 1").Scan(&revision); errorValue != nil {
		return 0, fmt.Errorf("read calendar event window source revision: %w", errorValue)
	}
	return revision, nil
}

func readCalendarEventWindowCacheEntry(ctx context.Context, database *sql.DB, cacheRange calendarEventWindowCacheRange, now time.Time) ([]calendarEvent, bool, error) {
	var schemaVersion int
	var payloadJSON []byte
	errorValue := database.QueryRowContext(ctx, `
		SELECT schema_version, payload_json
		FROM calendar_event_window_cache_entries
		WHERE cache_key = ?`, cacheRange.Key).Scan(&schemaVersion, &payloadJSON)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return nil, false, nil
	}
	if errorValue != nil {
		return nil, false, fmt.Errorf("read calendar event window cache entry: %w", errorValue)
	}
	var payload calendarEventWindowCachePayload
	if schemaVersion != calendarEventWindowCacheSchemaVersion || json.Unmarshal(payloadJSON, &payload) != nil || payload.Version != calendarEventWindowCacheSchemaVersion {
		if _, errorValue := database.ExecContext(ctx, "DELETE FROM calendar_event_window_cache_entries WHERE cache_key = ?", cacheRange.Key); errorValue != nil {
			return nil, false, fmt.Errorf("delete invalid calendar event window cache entry: %w", errorValue)
		}
		return nil, false, nil
	}
	if _, errorValue := database.ExecContext(ctx, "UPDATE calendar_event_window_cache_entries SET last_used_at = ? WHERE cache_key = ?", now.UTC().Format(time.RFC3339Nano), cacheRange.Key); errorValue != nil {
		return nil, false, fmt.Errorf("touch calendar event window cache entry: %w", errorValue)
	}
	return payload.Events, true, nil
}

func writeCalendarEventWindowCacheEntryIfCurrent(ctx context.Context, database *sql.DB, cacheRange calendarEventWindowCacheRange, expectedRevision int64, events []calendarEvent, now time.Time) (bool, error) {
	payloadJSON, errorValue := json.Marshal(calendarEventWindowCachePayload{Version: calendarEventWindowCacheSchemaVersion, Events: events})
	if errorValue != nil {
		return false, fmt.Errorf("encode calendar event window cache payload: %w", errorValue)
	}
	timestamp := now.UTC().Format(time.RFC3339Nano)
	result, errorValue := database.ExecContext(ctx, `
		INSERT INTO calendar_event_window_cache_entries (
			cache_key, start_at, end_at, source_revision, schema_version, payload_json, cached_at, last_used_at
		)
		SELECT ?, ?, ?, ?, ?, ?, ?, ?
		WHERE (SELECT revision FROM calendar_event_window_source_state WHERE id = 1) = ?
		ON CONFLICT(cache_key) DO UPDATE SET
			start_at = excluded.start_at,
			end_at = excluded.end_at,
			source_revision = excluded.source_revision,
			schema_version = excluded.schema_version,
			payload_json = excluded.payload_json,
			cached_at = excluded.cached_at,
			last_used_at = excluded.last_used_at`,
		cacheRange.Key,
		cacheRange.StartISO,
		cacheRange.EndISO,
		expectedRevision,
		calendarEventWindowCacheSchemaVersion,
		payloadJSON,
		timestamp,
		timestamp,
		expectedRevision,
	)
	if errorValue != nil {
		return false, fmt.Errorf("write calendar event window cache entry: %w", errorValue)
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return false, fmt.Errorf("read calendar event window cache write result: %w", errorValue)
	}
	if rowsAffected == 0 {
		return false, nil
	}
	if errorValue := cleanupCalendarEventWindowCacheEntries(ctx, database); errorValue != nil {
		return false, errorValue
	}
	return true, nil
}

func cleanupCalendarEventWindowCacheEntries(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
		DELETE FROM calendar_event_window_cache_entries
		WHERE cache_key IN (
			SELECT cache_key
			FROM calendar_event_window_cache_entries
			ORDER BY last_used_at DESC, cache_key DESC
			LIMIT -1 OFFSET ?
		)`, calendarEventWindowCacheMaximumEntries)
	if errorValue != nil {
		return fmt.Errorf("clean calendar event window cache entries: %w", errorValue)
	}
	return nil
}
