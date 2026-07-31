package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	flowTaskImportBatchStatusRunning    = "running"
	flowTaskImportBatchStatusCompleted  = "completed"
	flowTaskImportBatchStatusFailed     = "failed"
	flowTaskImportBatchStatusRolledBack = "rolled_back"
)

type flowTaskImportBatch struct {
	BatchID                 string `json:"batchID"`
	SourceSystem            string `json:"sourceSystem"`
	RequesterEmail          string `json:"requesterEmail,omitempty"`
	Status                  string `json:"status"`
	IsDryRun                bool   `json:"isDryRun"`
	ConflictPolicy          string `json:"conflictPolicy"`
	ReceivedCount           int    `json:"receivedCount"`
	ValidCount              int    `json:"validCount"`
	CreatedCount            int    `json:"createdCount"`
	UpdatedCount            int    `json:"updatedCount"`
	SkippedCount            int    `json:"skippedCount"`
	ConflictCount           int    `json:"conflictCount"`
	FailedCount             int    `json:"failedCount"`
	PotentialDuplicateCount int    `json:"potentialDuplicateCount"`
	Checksum                string `json:"checksum,omitempty"`
	Issues                  string `json:"issues,omitempty"`
	StartedAt               string `json:"startedAt"`
	FinishedAt              string `json:"finishedAt,omitempty"`
}

var errFlowTaskImportBatchIncomplete = errors.New("flow task import batch requires a batch id, source system, and status")

func writeFlowTaskImportBatch(ctx context.Context, executor sqlContextExecutor, batch flowTaskImportBatch) error {
	batch = trimmedFlowTaskImportBatch(batch)
	if batch.BatchID == "" || batch.SourceSystem == "" || batch.Status == "" {
		return errFlowTaskImportBatchIncomplete
	}
	if batch.StartedAt == "" {
		batch.StartedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if batch.ConflictPolicy == "" {
		batch.ConflictPolicy = "skip"
	}
	_, errorValue := executor.ExecContext(ctx, `
INSERT INTO flow_task_import_batches (
		batch_id, source_system, requester_email, status, is_dry_run, conflict_policy,
		received_count, valid_count, created_count, updated_count, skipped_count,
		conflict_count, failed_count, potential_duplicate_count, checksum, issues, started_at, finished_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(batch_id) DO UPDATE SET
	status = excluded.status,
	received_count = excluded.received_count,
	valid_count = excluded.valid_count,
	created_count = excluded.created_count,
	updated_count = excluded.updated_count,
	skipped_count = excluded.skipped_count,
	conflict_count = excluded.conflict_count,
	failed_count = excluded.failed_count,
	potential_duplicate_count = excluded.potential_duplicate_count,
	checksum = excluded.checksum,
	issues = excluded.issues,
	finished_at = excluded.finished_at`,
		batch.BatchID,
		batch.SourceSystem,
		batch.RequesterEmail,
		batch.Status,
		boolToInteger(batch.IsDryRun),
		batch.ConflictPolicy,
		batch.ReceivedCount,
		batch.ValidCount,
		batch.CreatedCount,
		batch.UpdatedCount,
		batch.SkippedCount,
		batch.ConflictCount,
		batch.FailedCount,
		batch.PotentialDuplicateCount,
		batch.Checksum,
		batch.Issues,
		batch.StartedAt,
		batch.FinishedAt,
	)
	return errorValue
}

func readFlowTaskImportBatch(ctx context.Context, queryer flowTaskProvenanceQueryer, batchID string) (flowTaskImportBatch, bool, error) {
	row := queryer.QueryRowContext(ctx, `
SELECT batch_id, source_system, requester_email, status, is_dry_run, conflict_policy,
	received_count, valid_count, created_count, updated_count, skipped_count,
	conflict_count, failed_count, potential_duplicate_count, checksum, issues, started_at, finished_at
FROM flow_task_import_batches WHERE batch_id = ?`, strings.TrimSpace(batchID))
	batch := flowTaskImportBatch{}
	isDryRun := 0
	errorValue := row.Scan(
		&batch.BatchID,
		&batch.SourceSystem,
		&batch.RequesterEmail,
		&batch.Status,
		&isDryRun,
		&batch.ConflictPolicy,
		&batch.ReceivedCount,
		&batch.ValidCount,
		&batch.CreatedCount,
		&batch.UpdatedCount,
		&batch.SkippedCount,
		&batch.ConflictCount,
		&batch.FailedCount,
		&batch.PotentialDuplicateCount,
		&batch.Checksum,
		&batch.Issues,
		&batch.StartedAt,
		&batch.FinishedAt,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return flowTaskImportBatch{}, false, nil
	}
	if errorValue != nil {
		return flowTaskImportBatch{}, false, errorValue
	}
	batch.IsDryRun = isDryRun != 0
	return batch, true, nil
}

func boolToInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}

func trimmedFlowTaskImportBatch(batch flowTaskImportBatch) flowTaskImportBatch {
	batch.BatchID = strings.TrimSpace(batch.BatchID)
	batch.SourceSystem = strings.TrimSpace(batch.SourceSystem)
	batch.RequesterEmail = strings.TrimSpace(batch.RequesterEmail)
	batch.Status = strings.TrimSpace(batch.Status)
	batch.ConflictPolicy = strings.TrimSpace(batch.ConflictPolicy)
	batch.StartedAt = strings.TrimSpace(batch.StartedAt)
	batch.FinishedAt = strings.TrimSpace(batch.FinishedAt)
	return batch
}
