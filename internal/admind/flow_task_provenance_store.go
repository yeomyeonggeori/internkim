package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type flowTaskExternalReference struct {
	TaskID                string   `json:"taskID"`
	SourceSystem          string   `json:"sourceSystem"`
	SourceRecordID        string   `json:"sourceRecordID"`
	SourceLocator         string   `json:"sourceLocator,omitempty"`
	SourcePayloadHash     string   `json:"sourcePayloadHash,omitempty"`
	DedupeKey             string   `json:"dedupeKey,omitempty"`
	ImportBatchID         string   `json:"importBatchID,omitempty"`
	OriginalCreatedAt     string   `json:"originalCreatedAt,omitempty"`
	OriginalUpdatedAt     string   `json:"originalUpdatedAt,omitempty"`
	OriginalOwnerName     string   `json:"originalOwnerName,omitempty"`
	OriginalStatus        string   `json:"originalStatus,omitempty"`
	OriginalBusiness      string   `json:"originalBusiness,omitempty"`
	OriginalCategory      string   `json:"originalCategory,omitempty"`
	OriginalType          string   `json:"originalType,omitempty"`
	SourceMetadata        string   `json:"sourceMetadata,omitempty"`
	NormalizationWarnings []string `json:"normalizationWarnings,omitempty"`
	CreatedAt             string   `json:"createdAt,omitempty"`
}

var errFlowTaskExternalReferenceIncomplete = errors.New("flow task external reference requires a task id, source system, and source record id")

type flowTaskProvenanceQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func writeFlowTaskExternalReference(ctx context.Context, executor sqlContextExecutor, reference flowTaskExternalReference) error {
	reference = trimmedFlowTaskExternalReference(reference)
	if reference.TaskID == "" || reference.SourceSystem == "" || reference.SourceRecordID == "" {
		return errFlowTaskExternalReferenceIncomplete
	}
	createdAt := reference.CreatedAt
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339)
	}
	_, errorValue := executor.ExecContext(ctx, `
INSERT INTO flow_task_external_refs (
		task_id, source_system, source_record_id, source_locator, source_payload_hash, dedupe_key, import_batch_id,
		original_created_at, original_updated_at, original_owner_name, original_status,
		original_business, original_category, original_type, source_metadata, normalization_warnings, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		reference.TaskID,
		reference.SourceSystem,
		reference.SourceRecordID,
		reference.SourceLocator,
		reference.SourcePayloadHash,
		reference.DedupeKey,
		reference.ImportBatchID,
		reference.OriginalCreatedAt,
		reference.OriginalUpdatedAt,
		reference.OriginalOwnerName,
		reference.OriginalStatus,
		reference.OriginalBusiness,
		reference.OriginalCategory,
		reference.OriginalType,
		reference.SourceMetadata,
		encodeFlowTaskWarnings(reference.NormalizationWarnings),
		createdAt,
	)
	return errorValue
}

func readFlowTaskExternalReferenceBySource(ctx context.Context, queryer flowTaskProvenanceQueryer, sourceSystem string, sourceRecordID string) (flowTaskExternalReference, bool, error) {
	row := queryer.QueryRowContext(ctx, flowTaskExternalReferenceSelectStatement+" WHERE source_system = ? AND source_record_id = ?", strings.TrimSpace(sourceSystem), strings.TrimSpace(sourceRecordID))
	return scanFlowTaskExternalReference(row)
}

func readFlowTaskExternalReferencesByBatch(ctx context.Context, queryer flowTaskProvenanceQueryer, importBatchID string) ([]flowTaskExternalReference, error) {
	rows, errorValue := queryer.QueryContext(ctx, flowTaskExternalReferenceSelectStatement+" WHERE import_batch_id = ? ORDER BY created_at, task_id", strings.TrimSpace(importBatchID))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	references := []flowTaskExternalReference{}
	for rows.Next() {
		reference, _, errorValue := scanFlowTaskExternalReference(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		references = append(references, reference)
	}
	return references, rows.Err()
}

const flowTaskExternalReferenceSelectStatement = `
SELECT task_id, source_system, source_record_id, source_locator, source_payload_hash, dedupe_key, import_batch_id,
	original_created_at, original_updated_at, original_owner_name, original_status,
	original_business, original_category, original_type, source_metadata, normalization_warnings, created_at
FROM flow_task_external_refs`

type sqlRowScanner interface {
	Scan(destinations ...any) error
}

func scanFlowTaskExternalReference(scanner sqlRowScanner) (flowTaskExternalReference, bool, error) {
	reference := flowTaskExternalReference{}
	warnings := ""
	errorValue := scanner.Scan(
		&reference.TaskID,
		&reference.SourceSystem,
		&reference.SourceRecordID,
		&reference.SourceLocator,
		&reference.SourcePayloadHash,
		&reference.DedupeKey,
		&reference.ImportBatchID,
		&reference.OriginalCreatedAt,
		&reference.OriginalUpdatedAt,
		&reference.OriginalOwnerName,
		&reference.OriginalStatus,
		&reference.OriginalBusiness,
		&reference.OriginalCategory,
		&reference.OriginalType,
		&reference.SourceMetadata,
		&warnings,
		&reference.CreatedAt,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return flowTaskExternalReference{}, false, nil
	}
	if errorValue != nil {
		return flowTaskExternalReference{}, false, errorValue
	}
	reference.NormalizationWarnings = decodeFlowTaskWarnings(warnings)
	return reference, true, nil
}

func encodeFlowTaskWarnings(warnings []string) string {
	trimmed := make([]string, 0, len(warnings))
	for _, warning := range warnings {
		if cleaned := strings.TrimSpace(warning); cleaned != "" {
			trimmed = append(trimmed, cleaned)
		}
	}
	return strings.Join(trimmed, "\n")
}

func decodeFlowTaskWarnings(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, "\n")
}

func trimmedFlowTaskExternalReference(reference flowTaskExternalReference) flowTaskExternalReference {
	reference.TaskID = strings.TrimSpace(reference.TaskID)
	reference.SourceSystem = strings.TrimSpace(reference.SourceSystem)
	reference.SourceRecordID = strings.TrimSpace(reference.SourceRecordID)
	reference.SourceLocator = strings.TrimSpace(reference.SourceLocator)
	reference.SourcePayloadHash = strings.TrimSpace(reference.SourcePayloadHash)
	reference.DedupeKey = strings.TrimSpace(reference.DedupeKey)
	reference.ImportBatchID = strings.TrimSpace(reference.ImportBatchID)
	reference.CreatedAt = strings.TrimSpace(reference.CreatedAt)
	return reference
}
