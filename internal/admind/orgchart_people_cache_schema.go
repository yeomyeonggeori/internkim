package admind

import (
	"context"
	"database/sql"
	"fmt"
)

const orgchartPeopleCacheSchemaVersion = 1

func ensureOrgchartPeopleCacheSchema(ctx context.Context, database *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS orgchart_people_cache_states (
			cache_kind TEXT NOT NULL,
			cache_key TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 0,
			is_dirty INTEGER NOT NULL DEFAULT 0 CHECK(is_dirty IN (0, 1)),
			PRIMARY KEY(cache_kind, cache_key)
		)`,
		`CREATE TABLE IF NOT EXISTS orgchart_people_cache_entries (
			cache_kind TEXT NOT NULL,
			cache_key TEXT NOT NULL,
			source_revision TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			payload_json BLOB NOT NULL,
			cached_at TEXT NOT NULL,
			PRIMARY KEY(cache_kind, cache_key)
		)`,
		`CREATE INDEX IF NOT EXISTS orgchart_people_cache_entries_cached_at_idx
		ON orgchart_people_cache_entries(cached_at)`,
	}
	for _, statement := range statements {
		if _, errorValue := database.ExecContext(ctx, statement); errorValue != nil {
			return fmt.Errorf("ensure orgchart people cache schema: %w", errorValue)
		}
	}
	return nil
}
