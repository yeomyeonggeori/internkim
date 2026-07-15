package admind

import (
	"context"
	"database/sql"
	"fmt"
)

func ensureFlowSummaryCacheSchema(ctx context.Context, database *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS flow_summary_source_revisions (
			source_kind TEXT NOT NULL,
			source_key TEXT NOT NULL,
			revision INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (source_kind, source_key)
		)`,
		`CREATE TABLE IF NOT EXISTS flow_summary_cache_entries (
			week_code TEXT PRIMARY KEY,
			requested_week_revision INTEGER NOT NULL,
			previous_week_revision INTEGER NOT NULL,
			current_month_revision INTEGER NOT NULL,
			previous_month_revision INTEGER NOT NULL,
			definitions_revision INTEGER NOT NULL,
			member_fingerprint TEXT NOT NULL,
			schema_version INTEGER NOT NULL,
			payload_json TEXT NOT NULL,
			cached_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS flow_summary_cache_entries_cached_at_idx
		ON flow_summary_cache_entries(cached_at)`,
	}
	for _, statement := range statements {
		if _, errorValue := database.ExecContext(ctx, statement); errorValue != nil {
			return fmt.Errorf("create flow summary cache schema: %w", errorValue)
		}
	}
	return nil
}
