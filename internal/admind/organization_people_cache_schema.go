package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const organizationPeopleCacheSchemaVersion = 2

func ensureOrganizationPeopleCacheSchema(ctx context.Context, database *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS organization_people_cache_states (
			cache_kind TEXT NOT NULL,
			cache_key TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 0,
			is_dirty INTEGER NOT NULL DEFAULT 0 CHECK(is_dirty IN (0, 1)),
			active_mutations INTEGER NOT NULL DEFAULT 0 CHECK(active_mutations >= 0),
			PRIMARY KEY(cache_kind, cache_key)
		)`,
		`CREATE TABLE IF NOT EXISTS organization_people_cache_entries (
			cache_kind TEXT NOT NULL,
			cache_key TEXT NOT NULL,
			source_revision TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			payload_json BLOB NOT NULL,
			cached_at TEXT NOT NULL,
			PRIMARY KEY(cache_kind, cache_key)
		)`,
		`DROP INDEX IF EXISTS organization_people_cache_entries_cached_at_idx`,
	}
	for _, statement := range statements {
		if _, errorValue := database.ExecContext(ctx, statement); errorValue != nil {
			return fmt.Errorf("ensure organization people cache schema: %w", errorValue)
		}
	}
	stateSchema, errorValue := readSQLiteTableSchema(ctx, database, "organization_people_cache_states")
	if errorValue != nil {
		return fmt.Errorf("read organization people cache state schema: %w", errorValue)
	}
	if !strings.Contains(strings.ToLower(stateSchema), "active_mutations") {
		if _, errorValue := database.ExecContext(ctx, `ALTER TABLE organization_people_cache_states ADD COLUMN active_mutations INTEGER NOT NULL DEFAULT 0 CHECK(active_mutations >= 0)`); errorValue != nil {
			return fmt.Errorf("migrate organization people cache state schema: %w", errorValue)
		}
	}
	if _, errorValue := database.ExecContext(ctx, `DELETE FROM organization_people_cache_entries WHERE EXISTS (
		SELECT 1 FROM organization_people_cache_states
		WHERE organization_people_cache_states.cache_kind = organization_people_cache_entries.cache_kind
			AND organization_people_cache_states.cache_key = organization_people_cache_entries.cache_key
			AND (organization_people_cache_states.active_mutations != 0 OR organization_people_cache_states.is_dirty != 0)
	)`); errorValue != nil {
		return fmt.Errorf("remove interrupted organization people cache entries: %w", errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE organization_people_cache_states SET active_mutations = 0, is_dirty = 0 WHERE active_mutations != 0 OR is_dirty != 0`); errorValue != nil {
		return fmt.Errorf("recover organization people cache mutation state: %w", errorValue)
	}
	return nil
}
