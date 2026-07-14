package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const orgchartPeopleCacheSchemaVersion = 2

func ensureOrgchartPeopleCacheSchema(ctx context.Context, database *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS orgchart_people_cache_states (
			cache_kind TEXT NOT NULL,
			cache_key TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 0,
			is_dirty INTEGER NOT NULL DEFAULT 0 CHECK(is_dirty IN (0, 1)),
			active_mutations INTEGER NOT NULL DEFAULT 0 CHECK(active_mutations >= 0),
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
	stateSchema, errorValue := readSQLiteTableSchema(ctx, database, "orgchart_people_cache_states")
	if errorValue != nil {
		return fmt.Errorf("read orgchart people cache state schema: %w", errorValue)
	}
	if !strings.Contains(strings.ToLower(stateSchema), "active_mutations") {
		if _, errorValue := database.ExecContext(ctx, `ALTER TABLE orgchart_people_cache_states ADD COLUMN active_mutations INTEGER NOT NULL DEFAULT 0 CHECK(active_mutations >= 0)`); errorValue != nil {
			return fmt.Errorf("migrate orgchart people cache state schema: %w", errorValue)
		}
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM orgchart_people_cache_entries WHERE EXISTS (
		SELECT 1 FROM orgchart_people_cache_states
		WHERE orgchart_people_cache_states.cache_kind = orgchart_people_cache_entries.cache_kind
			AND orgchart_people_cache_states.cache_key = orgchart_people_cache_entries.cache_key
			AND (orgchart_people_cache_states.active_mutations != 0 OR orgchart_people_cache_states.is_dirty != 0)
	)`); errorValue != nil {
		return fmt.Errorf("remove interrupted orgchart people cache entries: %w", errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE orgchart_people_cache_states SET active_mutations = 0, is_dirty = 0 WHERE active_mutations != 0 OR is_dirty != 0`); errorValue != nil {
		return fmt.Errorf("recover orgchart people cache mutation state: %w", errorValue)
	}
	return nil
}
