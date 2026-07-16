package admind

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

type calendarEventWindowCacheAvailability struct {
	initialize sync.Once
	enabled    atomic.Bool
}

func (service *Service) initializeCalendarEventWindowCache(ctx context.Context, database *sql.DB) {
	availability := &service.calendarWindowCache
	availability.initialize.Do(func() {
		if errorValue := ensureCalendarEventWindowCacheSchema(ctx, database); errorValue != nil {
			slog.Warn("calendar event window cache initialization failed", "error", errorValue.Error())
			return
		}
		availability.enabled.Store(true)
	})
}

func (service *Service) isCalendarEventWindowCacheEnabled() bool {
	return service.calendarWindowCache.enabled.Load()
}

func ensureCalendarEventWindowCacheSchema(ctx context.Context, database *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS calendar_event_window_source_state (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			revision INTEGER NOT NULL DEFAULT 0
		)`,
		`INSERT INTO calendar_event_window_source_state(id, revision)
		VALUES (1, 0)
		ON CONFLICT(id) DO NOTHING`,
		`CREATE TABLE IF NOT EXISTS calendar_event_window_cache_entries (
			cache_key TEXT PRIMARY KEY,
			start_at TEXT NOT NULL,
			end_at TEXT NOT NULL,
			source_revision INTEGER NOT NULL,
			schema_version INTEGER NOT NULL,
			payload_json BLOB NOT NULL,
			cached_at TEXT NOT NULL,
			last_used_at TEXT NOT NULL,
			CHECK(start_at < end_at)
		)`,
		`CREATE INDEX IF NOT EXISTS calendar_event_window_cache_overlap_idx
		ON calendar_event_window_cache_entries(end_at, start_at)`,
		`CREATE INDEX IF NOT EXISTS calendar_event_window_cache_last_used_idx
		ON calendar_event_window_cache_entries(last_used_at)`,
	}
	for _, statement := range statements {
		if _, errorValue := database.ExecContext(ctx, statement); errorValue != nil {
			return fmt.Errorf("ensure calendar event window cache schema: %w", errorValue)
		}
	}
	if _, errorValue := database.ExecContext(ctx, "DELETE FROM calendar_event_window_cache_entries"); errorValue != nil {
		return fmt.Errorf("reset calendar event window cache entries: %w", errorValue)
	}
	return nil
}
