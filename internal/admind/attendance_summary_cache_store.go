package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type attendanceSummaryCacheKind string

const (
	attendanceSummaryCacheKindEvents    attendanceSummaryCacheKind = "events"
	attendanceSummaryCacheKindAbsences  attendanceSummaryCacheKind = "absences"
	attendanceSummaryCacheSchemaVersion                            = 1
	attendanceSummaryCacheLifetime                                 = 90 * 24 * time.Hour
)

type attendanceSummaryCacheEntry struct {
	Generation     string
	SourceRevision int64
	SchemaVersion  int
	Payload        string
	CachedAt       string
}

func (service *Service) attendanceSummaryCacheGeneration() string {
	return service.startedAt.UTC().Format(time.RFC3339Nano)
}

func (service *Service) readAttendanceSummaryCachePayload(
	ctx context.Context,
	kind attendanceSummaryCacheKind,
	month string,
	now time.Time,
) ([]byte, int64, bool, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, 0, false, errorValue
	}
	defer database.Close()
	revision, entry, found, errorValue := readAttendanceSummaryCacheSnapshot(ctx, database, kind, month)
	if errorValue != nil || !found {
		return nil, revision, false, errorValue
	}
	if attendanceSummaryCacheEntryIsValid(entry, revision, service.attendanceSummaryCacheGeneration(), now) {
		return []byte(entry.Payload), revision, true, nil
	}
	if errorValue := deleteAttendanceSummaryCacheEntry(ctx, database, kind, month, revision, entry); errorValue != nil {
		return nil, revision, false, errorValue
	}
	return nil, revision, false, nil
}

func readAttendanceSummaryCacheSnapshot(
	ctx context.Context,
	database *sql.DB,
	kind attendanceSummaryCacheKind,
	month string,
) (int64, attendanceSummaryCacheEntry, bool, error) {
	var revision int64
	var generation sql.NullString
	var sourceRevision sql.NullInt64
	var schemaVersion sql.NullInt64
	var payload sql.NullString
	var cachedAt sql.NullString
	errorValue := database.QueryRowContext(ctx, `
	WITH cache_key(cache_kind, month) AS (VALUES (?, ?))
	SELECT COALESCE(revisions.revision, 0), entries.generation, entries.source_revision, entries.schema_version, entries.payload_json, entries.cached_at
	FROM cache_key
	LEFT JOIN attendance_summary_cache_revisions AS revisions USING (cache_kind, month)
	LEFT JOIN attendance_summary_cache_entries AS entries USING (cache_kind, month)`, kind, month).Scan(
		&revision,
		&generation,
		&sourceRevision,
		&schemaVersion,
		&payload,
		&cachedAt,
	)
	if errorValue != nil {
		return 0, attendanceSummaryCacheEntry{}, false, fmt.Errorf("read attendance summary cache snapshot: %w", errorValue)
	}
	if !sourceRevision.Valid {
		return revision, attendanceSummaryCacheEntry{}, false, nil
	}
	entry := attendanceSummaryCacheEntry{
		Generation:     generation.String,
		SourceRevision: sourceRevision.Int64,
		SchemaVersion:  int(schemaVersion.Int64),
		Payload:        payload.String,
		CachedAt:       cachedAt.String,
	}
	return revision, entry, true, nil
}

func attendanceSummaryCacheEntryIsValid(entry attendanceSummaryCacheEntry, revision int64, generation string, now time.Time) bool {
	cachedAt, errorValue := time.Parse(time.RFC3339Nano, entry.CachedAt)
	if errorValue != nil {
		return false
	}
	return entry.Generation == generation &&
		entry.SourceRevision == revision &&
		entry.SchemaVersion == attendanceSummaryCacheSchemaVersion &&
		!cachedAt.Before(now.Add(-attendanceSummaryCacheLifetime))
}

func deleteAttendanceSummaryCacheEntry(
	ctx context.Context,
	database *sql.DB,
	kind attendanceSummaryCacheKind,
	month string,
	revision int64,
	entry attendanceSummaryCacheEntry,
) error {
	_, errorValue := database.ExecContext(ctx, `
	DELETE FROM attendance_summary_cache_entries
	WHERE cache_kind = ? AND month = ?
		AND generation = ? AND source_revision = ? AND schema_version = ? AND payload_json = ? AND cached_at = ?
		AND COALESCE((
			SELECT revision FROM attendance_summary_cache_revisions
			WHERE cache_kind = ? AND month = ?
		), 0) = ?`,
		kind,
		month,
		entry.Generation,
		entry.SourceRevision,
		entry.SchemaVersion,
		entry.Payload,
		entry.CachedAt,
		kind,
		month,
		revision,
	)
	if errorValue != nil {
		return fmt.Errorf("delete invalid attendance summary cache entry: %w", errorValue)
	}
	return nil
}

func (service *Service) deleteAttendanceSummaryCachePayloadIfUnchanged(
	ctx context.Context,
	kind attendanceSummaryCacheKind,
	month string,
	revision int64,
	payload []byte,
) error {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
	DELETE FROM attendance_summary_cache_entries
	WHERE cache_kind = ? AND month = ?
		AND generation = ? AND source_revision = ? AND payload_json = ?`,
		kind,
		month,
		service.attendanceSummaryCacheGeneration(),
		revision,
		string(payload),
	)
	if errorValue != nil {
		return fmt.Errorf("delete corrupt attendance summary cache payload: %w", errorValue)
	}
	return nil
}

func (service *Service) writeAttendanceSummaryCachePayloadIfCurrent(
	ctx context.Context,
	kind attendanceSummaryCacheKind,
	month string,
	expectedRevision int64,
	payload []byte,
	now time.Time,
) (bool, error) {
	if !json.Valid(payload) {
		return false, fmt.Errorf("write attendance summary cache: payload is not valid JSON")
	}
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	result, errorValue := database.ExecContext(ctx, `
	INSERT INTO attendance_summary_cache_entries (
		cache_kind, month, generation, source_revision, schema_version, payload_json, cached_at
	)
	SELECT ?, ?, ?, ?, ?, ?, ?
	WHERE COALESCE((
		SELECT revision FROM attendance_summary_cache_revisions
		WHERE cache_kind = ? AND month = ?
	), 0) = ?
	ON CONFLICT(cache_kind, month) DO UPDATE SET
		generation = excluded.generation,
	source_revision = excluded.source_revision,
	schema_version = excluded.schema_version,
	payload_json = excluded.payload_json,
	cached_at = excluded.cached_at`,
		kind,
		month,
		service.attendanceSummaryCacheGeneration(),
		expectedRevision,
		attendanceSummaryCacheSchemaVersion,
		string(payload),
		now.UTC().Format(time.RFC3339Nano),
		kind,
		month,
		expectedRevision,
	)
	if errorValue != nil {
		return false, fmt.Errorf("write attendance summary cache entry: %w", errorValue)
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return false, fmt.Errorf("read attendance summary cache write result: %w", errorValue)
	}
	if rowsAffected == 0 {
		return false, nil
	}
	return true, nil
}

func invalidateAttendanceSummaryCacheMonths(
	ctx context.Context,
	transaction *sql.Tx,
	kind attendanceSummaryCacheKind,
	months []string,
) error {
	monthSet := map[string]struct{}{}
	for _, month := range months {
		if _, errorValue := time.Parse("2006-01", month); errorValue != nil {
			return fmt.Errorf("invalidate attendance summary cache month %q: %w", month, errorValue)
		}
		monthSet[month] = struct{}{}
	}
	for month := range monthSet {
		if errorValue := invalidateAttendanceSummaryCacheKey(ctx, transaction, kind, month); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func invalidateAttendanceSummaryCacheKey(
	ctx context.Context,
	transaction *sql.Tx,
	kind attendanceSummaryCacheKind,
	month string,
) error {
	_, errorValue := transaction.ExecContext(ctx, `
	INSERT INTO attendance_summary_cache_revisions (cache_kind, month, revision)
	VALUES (?, ?, 1)
	ON CONFLICT(cache_kind, month) DO UPDATE SET revision = revision + 1`, kind, month)
	if errorValue != nil {
		return fmt.Errorf("increase attendance summary cache revision: %w", errorValue)
	}
	_, errorValue = transaction.ExecContext(ctx, `
	DELETE FROM attendance_summary_cache_entries
	WHERE cache_kind = ? AND month = ?`, kind, month)
	if errorValue != nil {
		return fmt.Errorf("delete attendance summary cache entry: %w", errorValue)
	}
	return nil
}
