package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const flowSummarySnapshotSQL = `
SELECT
	COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0),
	COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0),
	COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0),
	COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0),
	COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0),
	entries.requested_week_revision,
	entries.previous_week_revision,
	entries.current_month_revision,
	entries.previous_month_revision,
	entries.definitions_revision,
	entries.member_fingerprint,
	entries.schema_version,
	entries.payload_json
FROM (SELECT 1) AS cache_lookup
LEFT JOIN flow_summary_cache_entries AS entries ON entries.week_code = ?`

const flowSummaryConditionalWriteSQL = `
INSERT INTO flow_summary_cache_entries (
	week_code,
	requested_week_revision,
	previous_week_revision,
	current_month_revision,
	previous_month_revision,
	definitions_revision,
	member_fingerprint,
	schema_version,
	payload_json,
	cached_at
)
SELECT ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
WHERE COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0) = ?
	AND COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0) = ?
	AND COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0) = ?
	AND COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0) = ?
	AND COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0) = ?
ON CONFLICT(week_code) DO UPDATE SET
	requested_week_revision = excluded.requested_week_revision,
	previous_week_revision = excluded.previous_week_revision,
	current_month_revision = excluded.current_month_revision,
	previous_month_revision = excluded.previous_month_revision,
	definitions_revision = excluded.definitions_revision,
	member_fingerprint = excluded.member_fingerprint,
	schema_version = excluded.schema_version,
	payload_json = excluded.payload_json,
	cached_at = excluded.cached_at`

func (service *Service) readFlowSummaryCacheSnapshot(ctx context.Context, weekCode string, keys flowSummaryDependencyKeys, memberFingerprint string) (flowSummaryCacheEntry, flowSummaryDependencySnapshot, bool, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return flowSummaryCacheEntry{}, flowSummaryDependencySnapshot{}, false, errorValue
	}
	defer database.Close()
	var current flowSummaryDependencySnapshot
	var requestedWeekRevision sql.NullInt64
	var previousWeekRevision sql.NullInt64
	var currentMonthRevision sql.NullInt64
	var previousMonthRevision sql.NullInt64
	var definitionsRevision sql.NullInt64
	var storedMemberFingerprint sql.NullString
	var schemaVersion sql.NullInt64
	var payload sql.NullString
	errorValue = database.QueryRowContext(
		ctx,
		flowSummarySnapshotSQL,
		keys.RequestedWeek.Kind, keys.RequestedWeek.Key,
		keys.PreviousWeek.Kind, keys.PreviousWeek.Key,
		keys.CurrentMonth.Kind, keys.CurrentMonth.Key,
		keys.PreviousMonth.Kind, keys.PreviousMonth.Key,
		keys.Definitions.Kind, keys.Definitions.Key,
		weekCode,
	).Scan(
		&current.RequestedWeekRevision,
		&current.PreviousWeekRevision,
		&current.CurrentMonthRevision,
		&current.PreviousMonthRevision,
		&current.DefinitionsRevision,
		&requestedWeekRevision,
		&previousWeekRevision,
		&currentMonthRevision,
		&previousMonthRevision,
		&definitionsRevision,
		&storedMemberFingerprint,
		&schemaVersion,
		&payload,
	)
	if errorValue != nil {
		return flowSummaryCacheEntry{}, flowSummaryDependencySnapshot{}, false, fmt.Errorf("read flow summary cache snapshot: %w", errorValue)
	}
	current.MemberFingerprint = memberFingerprint
	if !requestedWeekRevision.Valid {
		return flowSummaryCacheEntry{}, current, false, nil
	}
	entry := flowSummaryCacheEntry{
		WeekCode: weekCode,
		Dependencies: flowSummaryDependencySnapshot{
			RequestedWeekRevision: requestedWeekRevision.Int64,
			PreviousWeekRevision:  previousWeekRevision.Int64,
			CurrentMonthRevision:  currentMonthRevision.Int64,
			PreviousMonthRevision: previousMonthRevision.Int64,
			DefinitionsRevision:   definitionsRevision.Int64,
			MemberFingerprint:     storedMemberFingerprint.String,
		},
		SchemaVersion: int(schemaVersion.Int64),
		Payload:       payload.String,
	}
	return entry, current, flowSummaryCacheEntryMatches(entry, current), nil
}

func flowSummaryCacheEntryMatches(entry flowSummaryCacheEntry, current flowSummaryDependencySnapshot) bool {
	return entry.Dependencies == current && entry.SchemaVersion == flowSummaryCacheSchemaVersion
}

func (service *Service) writeFlowSummaryCacheEntryIfCurrent(ctx context.Context, weekCode string, keys flowSummaryDependencyKeys, expected flowSummaryDependencySnapshot, payload []byte, now time.Time) (bool, error) {
	if weekCode == "" || canonicalFlowSummaryWeekCode(weekCode, now) != weekCode {
		return false, fmt.Errorf("write flow summary cache: noncanonical week code %q", weekCode)
	}
	if !json.Valid(payload) {
		return false, fmt.Errorf("write flow summary cache: payload is not valid JSON")
	}
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	result, errorValue := database.ExecContext(
		ctx,
		flowSummaryConditionalWriteSQL,
		weekCode,
		expected.RequestedWeekRevision,
		expected.PreviousWeekRevision,
		expected.CurrentMonthRevision,
		expected.PreviousMonthRevision,
		expected.DefinitionsRevision,
		expected.MemberFingerprint,
		flowSummaryCacheSchemaVersion,
		string(payload),
		now.UTC().Format(time.RFC3339Nano),
		keys.RequestedWeek.Kind, keys.RequestedWeek.Key, expected.RequestedWeekRevision,
		keys.PreviousWeek.Kind, keys.PreviousWeek.Key, expected.PreviousWeekRevision,
		keys.CurrentMonth.Kind, keys.CurrentMonth.Key, expected.CurrentMonthRevision,
		keys.PreviousMonth.Kind, keys.PreviousMonth.Key, expected.PreviousMonthRevision,
		keys.Definitions.Kind, keys.Definitions.Key, expected.DefinitionsRevision,
	)
	if errorValue != nil {
		return false, fmt.Errorf("write flow summary cache entry: %w", errorValue)
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return false, fmt.Errorf("read flow summary cache write result: %w", errorValue)
	}
	if rowsAffected == 0 {
		return false, nil
	}
	return true, nil
}

func (service *Service) deleteFlowSummaryCacheEntryIfUnchanged(ctx context.Context, entry flowSummaryCacheEntry) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
	DELETE FROM flow_summary_cache_entries
	WHERE week_code = ?
		AND requested_week_revision = ?
		AND previous_week_revision = ?
		AND current_month_revision = ?
		AND previous_month_revision = ?
		AND definitions_revision = ?
		AND member_fingerprint = ?
		AND schema_version = ?
		AND payload_json = ?`,
		entry.WeekCode,
		entry.Dependencies.RequestedWeekRevision,
		entry.Dependencies.PreviousWeekRevision,
		entry.Dependencies.CurrentMonthRevision,
		entry.Dependencies.PreviousMonthRevision,
		entry.Dependencies.DefinitionsRevision,
		entry.Dependencies.MemberFingerprint,
		entry.SchemaVersion,
		entry.Payload,
	)
	if errorValue != nil {
		return fmt.Errorf("delete flow summary cache entry: %w", errorValue)
	}
	return nil
}

func (service *Service) cleanupFlowSummaryCacheEntries(ctx context.Context, now time.Time) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return cleanupFlowSummaryCacheEntriesInDatabase(ctx, database, now)
}

func cleanupFlowSummaryCacheEntriesInDatabase(ctx context.Context, database *sql.DB, now time.Time) error {
	cutoff := now.Add(-flowSummaryCacheRetention).UTC().Format(time.RFC3339Nano)
	if _, errorValue := database.ExecContext(ctx, "DELETE FROM flow_summary_cache_entries WHERE cached_at < ?", cutoff); errorValue != nil {
		return fmt.Errorf("clean flow summary cache entries: %w", errorValue)
	}
	return nil
}

func incrementFlowSummarySourceRevisions(ctx context.Context, transaction *sql.Tx, keys []flowSummarySourceKey) error {
	uniqueKeys := map[flowSummarySourceKey]struct{}{}
	for _, key := range keys {
		if strings.TrimSpace(string(key.Kind)) == "" || strings.TrimSpace(key.Key) == "" {
			return fmt.Errorf("increment flow summary source revision: empty source key")
		}
		uniqueKeys[key] = struct{}{}
	}
	orderedKeys := make([]flowSummarySourceKey, 0, len(uniqueKeys))
	for key := range uniqueKeys {
		orderedKeys = append(orderedKeys, key)
	}
	sort.Slice(orderedKeys, func(leftIndex int, rightIndex int) bool {
		if orderedKeys[leftIndex].Kind != orderedKeys[rightIndex].Kind {
			return orderedKeys[leftIndex].Kind < orderedKeys[rightIndex].Kind
		}
		return orderedKeys[leftIndex].Key < orderedKeys[rightIndex].Key
	})
	for _, key := range orderedKeys {
		if _, errorValue := transaction.ExecContext(ctx, `
		INSERT INTO flow_summary_source_revisions(source_kind, source_key, revision)
		VALUES (?, ?, 1)
		ON CONFLICT(source_kind, source_key) DO UPDATE SET revision = revision + 1`, key.Kind, key.Key); errorValue != nil {
			return fmt.Errorf("increment flow summary source revision %s/%s: %w", key.Kind, key.Key, errorValue)
		}
	}
	return nil
}
