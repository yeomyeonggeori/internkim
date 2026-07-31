package admind

import (
	"context"
	"database/sql"
)

func ensureFlowTaskProvenanceSchema(ctx context.Context, database *sql.DB) error {
	if errorValue := ensureFlowColumn(ctx, database, "flow_tasks", "is_historical", "INTEGER NOT NULL DEFAULT 0"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowColumn(ctx, database, "flow_tasks", "archived_at", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowTaskExternalRefTable(ctx, database); errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowTaskImportBatchTable(ctx, database); errorValue != nil {
		return errorValue
	}
	return ensureFlowTaskProvenanceIndexes(ctx, database)
}

func ensureFlowTaskExternalRefTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_task_external_refs (
	task_id TEXT PRIMARY KEY,
	source_system TEXT NOT NULL,
	source_record_id TEXT NOT NULL,
	source_locator TEXT NOT NULL DEFAULT '',
	source_payload_hash TEXT NOT NULL DEFAULT '',
	dedupe_key TEXT NOT NULL DEFAULT '',
	import_batch_id TEXT NOT NULL DEFAULT '',
	original_created_at TEXT NOT NULL DEFAULT '',
	original_updated_at TEXT NOT NULL DEFAULT '',
	original_owner_name TEXT NOT NULL DEFAULT '',
	original_status TEXT NOT NULL DEFAULT '',
	original_business TEXT NOT NULL DEFAULT '',
	original_category TEXT NOT NULL DEFAULT '',
	original_type TEXT NOT NULL DEFAULT '',
	source_metadata TEXT NOT NULL DEFAULT '',
	normalization_warnings TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	UNIQUE(source_system, source_record_id)
)`)
	return errorValue
}

func ensureFlowTaskImportBatchTable(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_task_import_batches (
	batch_id TEXT PRIMARY KEY,
	source_system TEXT NOT NULL,
	requester_email TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	is_dry_run INTEGER NOT NULL DEFAULT 0,
	conflict_policy TEXT NOT NULL DEFAULT 'skip',
	received_count INTEGER NOT NULL DEFAULT 0,
	valid_count INTEGER NOT NULL DEFAULT 0,
	created_count INTEGER NOT NULL DEFAULT 0,
	updated_count INTEGER NOT NULL DEFAULT 0,
	skipped_count INTEGER NOT NULL DEFAULT 0,
	conflict_count INTEGER NOT NULL DEFAULT 0,
	failed_count INTEGER NOT NULL DEFAULT 0,
	potential_duplicate_count INTEGER NOT NULL DEFAULT 0,
	checksum TEXT NOT NULL DEFAULT '',
	issues TEXT NOT NULL DEFAULT '',
	started_at TEXT NOT NULL,
	finished_at TEXT NOT NULL DEFAULT ''
)`)
	return errorValue
}

func ensureFlowTaskProvenanceIndexes(ctx context.Context, database *sql.DB) error {
	for _, statement := range []string{
		"CREATE INDEX IF NOT EXISTS flow_tasks_is_historical_idx ON flow_tasks(is_historical)",
		"CREATE INDEX IF NOT EXISTS flow_task_external_refs_import_batch_id_idx ON flow_task_external_refs(import_batch_id)",
		"CREATE INDEX IF NOT EXISTS flow_task_external_refs_dedupe_key_idx ON flow_task_external_refs(dedupe_key)",
	} {
		if _, errorValue := database.ExecContext(ctx, statement); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
