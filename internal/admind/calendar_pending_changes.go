package admind

import (
	"context"
	"strconv"
	"strings"
	"time"
)

type retainedCalendarOutboxRow struct {
	ID     int64
	Fields []string
	Status string
}

type pendingCalendarLocalChange struct {
	EventUID       string
	ChangedFields  []string
	FieldChangedAt map[string]time.Time
}

func normalizeCalendarOutboxPutChangedFields(fields []string) []string {
	if len(fields) == 0 {
		return calendarAllUserEditableFields()
	}
	return fields
}

func (service *Service) listPendingCalendarLocalChanges(ctx context.Context, accountID string, targetCalendarURL string) (map[string]pendingCalendarLocalChange, error) {
	rows, errorValue := service.listCalendarOutbox(ctx, accountID, true)
	if errorValue != nil {
		return nil, errorValue
	}
	result := map[string]pendingCalendarLocalChange{}
	for _, row := range rows {
		eventUID := strings.TrimSpace(row.EventUID)
		if row.Operation != calendarOutboxOperationPut || eventUID == "" || normalizeCalendarOutboxTargetURL(row.TargetCalendarURL) != normalizeCalendarOutboxTargetURL(targetCalendarURL) {
			continue
		}
		change := result[eventUID]
		change.EventUID = eventUID
		if change.FieldChangedAt == nil {
			change.FieldChangedAt = map[string]time.Time{}
		}
		change.ChangedFields = mergeCalendarFieldLists(change.ChangedFields, row.ChangedFields)
		changedAt := parseCalendarConflictTime(row.CreatedAt)
		for _, field := range row.ChangedFields {
			if changedAt.After(change.FieldChangedAt[field]) {
				change.FieldChangedAt[field] = changedAt
			}
		}
		result[eventUID] = change
	}
	return result, nil
}

func (service *Service) readPendingCalendarLocalChange(ctx context.Context, accountID string, targetCalendarURL string, eventUID string) (pendingCalendarLocalChange, bool, error) {
	trimmedEventUID := strings.TrimSpace(eventUID)
	if trimmedEventUID == "" {
		return pendingCalendarLocalChange{}, false, nil
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return pendingCalendarLocalChange{}, false, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT changed_fields, created_at
FROM calendar_outbox
WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND operation = ? AND `+calendarOutboxActiveStatusPredicate+`
ORDER BY id`,
		strings.TrimSpace(accountID),
		normalizeCalendarOutboxTargetURL(targetCalendarURL),
		trimmedEventUID,
		calendarOutboxOperationPut,
		calendarOutboxStatusPending,
		calendarOutboxStatusBlocked,
	)
	if errorValue != nil {
		return pendingCalendarLocalChange{}, false, errorValue
	}
	defer rows.Close()
	change := pendingCalendarLocalChange{EventUID: trimmedEventUID, FieldChangedAt: map[string]time.Time{}}
	found := false
	for rows.Next() {
		var changedFieldsRaw string
		var createdAtRaw string
		if errorValue := rows.Scan(&changedFieldsRaw, &createdAtRaw); errorValue != nil {
			return pendingCalendarLocalChange{}, false, errorValue
		}
		changedFields := normalizeCalendarOutboxPutChangedFields(decodeChangedFields(changedFieldsRaw))
		change.ChangedFields = mergeCalendarFieldLists(change.ChangedFields, changedFields)
		changedAt := parseCalendarConflictTime(createdAtRaw)
		for _, field := range changedFields {
			if changedAt.After(change.FieldChangedAt[field]) {
				change.FieldChangedAt[field] = changedAt
			}
		}
		found = true
	}
	return change, found, rows.Err()
}

func mergeCalendarFieldLists(left []string, right []string) []string {
	if len(left) == 0 {
		return append([]string(nil), right...)
	}
	result := append([]string(nil), left...)
	seen := map[string]struct{}{}
	for _, field := range result {
		seen[field] = struct{}{}
	}
	for _, field := range right {
		if _, found := seen[field]; found {
			continue
		}
		result = append(result, field)
		seen[field] = struct{}{}
	}
	return result
}

func aggregateCalendarOutboxRows(rows []calendarOutboxRow) []calendarOutboxRow {
	result := []calendarOutboxRow{}
	indexesByEvent := map[string]int{}
	for _, row := range rows {
		key := calendarOutboxAggregationKey(row)
		index, found := indexesByEvent[key]
		if !found {
			indexesByEvent[key] = len(result)
			result = append(result, initializeCalendarOutboxBatch(row))
			continue
		}
		result[index] = mergeCalendarOutboxBatch(result[index], row)
	}
	return result
}

func calendarOutboxAggregationKey(row calendarOutboxRow) string {
	eventUID := strings.TrimSpace(row.EventUID)
	if eventUID == "" {
		return "row:" + strconv.FormatInt(row.ID, 10)
	}
	return strings.TrimSpace(row.AccountID) + "\x00" + normalizeCalendarOutboxTargetURL(row.TargetCalendarURL) + "\x00" + eventUID
}

func initializeCalendarOutboxBatch(row calendarOutboxRow) calendarOutboxRow {
	row.SourceRowIDs = []int64{row.ID}
	row.FieldChangedAt = calendarOutboxFieldChangedAt(row)
	return row
}

func mergeCalendarOutboxBatch(batch calendarOutboxRow, row calendarOutboxRow) calendarOutboxRow {
	sourceRowIDs := append(batch.SourceRowIDs, row.ID)
	if calendarOutboxRowHappenedAfter(row, batch) && row.Operation != batch.Operation {
		next := initializeCalendarOutboxBatch(row)
		next.SourceRowIDs = sourceRowIDs
		return next
	}
	if row.Operation == calendarOutboxOperationPut && batch.Operation == calendarOutboxOperationPut {
		batch.ChangedFields = mergeCalendarFieldLists(batch.ChangedFields, row.ChangedFields)
		for field, changedAt := range calendarOutboxFieldChangedAt(row) {
			if changedAt.After(batch.FieldChangedAt[field]) {
				batch.FieldChangedAt[field] = changedAt
			}
		}
	}
	if calendarOutboxRowHappenedAfter(row, batch) {
		batch.ID = row.ID
		batch.EventID = row.EventID
		batch.CreatedAt = row.CreatedAt
		batch.IfMatchETag = row.IfMatchETag
		batch.RemoteHref = row.RemoteHref
		batch.AttemptCount = row.AttemptCount
		batch.LastError = row.LastError
		batch.LastAttemptedAt = row.LastAttemptedAt
		batch.Status = row.Status
		batch.FailedAt = row.FailedAt
	}
	batch.SourceRowIDs = sourceRowIDs
	return batch
}

func calendarOutboxFieldChangedAt(row calendarOutboxRow) map[string]time.Time {
	result := map[string]time.Time{}
	changedAt := parseCalendarConflictTime(row.CreatedAt)
	for _, field := range row.ChangedFields {
		result[field] = changedAt
	}
	return result
}

func calendarOutboxRowHappenedAfter(candidate calendarOutboxRow, current calendarOutboxRow) bool {
	return candidate.ID > current.ID
}

func retainPendingCalendarOutboxFieldsWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, targetCalendarURL string, eventUID string, retainedFields []string, remoteHref string, remoteETag string) error {
	normalizedTarget := normalizeCalendarOutboxTargetURL(targetCalendarURL)
	if _, errorValue := queryRunner.ExecContext(ctx, `DELETE FROM calendar_outbox WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND operation <> ? AND `+calendarOutboxActiveStatusPredicate, strings.TrimSpace(accountID), normalizedTarget, strings.TrimSpace(eventUID), calendarOutboxOperationPut, calendarOutboxStatusPending, calendarOutboxStatusBlocked); errorValue != nil {
		return errorValue
	}
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT id, changed_fields, status
FROM calendar_outbox
WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND operation = ? AND `+calendarOutboxActiveStatusPredicate+`
ORDER BY id`, strings.TrimSpace(accountID), normalizedTarget, strings.TrimSpace(eventUID), calendarOutboxOperationPut, calendarOutboxStatusPending, calendarOutboxStatusBlocked)
	if errorValue != nil {
		return errorValue
	}
	retainedRows := []retainedCalendarOutboxRow{}
	for rows.Next() {
		var rowID int64
		var changedFieldsRaw string
		var status string
		if errorValue := rows.Scan(&rowID, &changedFieldsRaw, &status); errorValue != nil {
			rows.Close()
			return errorValue
		}
		changedFields := normalizeCalendarOutboxPutChangedFields(decodeChangedFields(changedFieldsRaw))
		retainedRows = append(retainedRows, retainedCalendarOutboxRow{ID: rowID, Fields: intersectCalendarFields(changedFields, retainedFields), Status: status})
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	for _, row := range retainedRows {
		if len(row.Fields) == 0 {
			if _, errorValue := queryRunner.ExecContext(ctx, `DELETE FROM calendar_outbox WHERE id = ?`, row.ID); errorValue != nil {
				return errorValue
			}
			continue
		}
		if row.Status == calendarOutboxStatusBlocked {
			if _, errorValue := queryRunner.ExecContext(ctx, `
UPDATE calendar_outbox
SET changed_fields = ?, if_match_etag = ?, remote_href = ?
WHERE id = ?`, encodeChangedFields(row.Fields), strings.TrimSpace(remoteETag), strings.TrimSpace(remoteHref), row.ID); errorValue != nil {
				return errorValue
			}
			continue
		}
		if _, errorValue := queryRunner.ExecContext(ctx, `
UPDATE calendar_outbox
SET changed_fields = ?, if_match_etag = ?, remote_href = ?, status = ?, failed_at = '', attempt_count = 0, last_error = ''
WHERE id = ?`, encodeChangedFields(row.Fields), strings.TrimSpace(remoteETag), strings.TrimSpace(remoteHref), calendarOutboxStatusPending, row.ID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
