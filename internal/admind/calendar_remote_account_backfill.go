package admind

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

type calendarBackfillEvent struct {
	ID             string
	UID            string
	RawICS         string
	RemoteETag     string
	RemoteHref     string
	RemoteSource   string
	UpdatedAt      string
	DeletedAt      string
	FieldChangedAt map[string]time.Time
	AcknowledgedAt map[string]time.Time
}

type calendarBackfillFieldGroup struct {
	ChangedAt string
	Fields    []string
}

func enqueueCalendarBackfillOutboxWithRunner(ctx context.Context, transaction *sql.Tx, account remoteCalendarAccount) (bool, error) {
	if !remoteCalendarAccountCanWrite(account) {
		return false, nil
	}
	target := selectedRemoteCalendarTarget(account)
	if target.CalendarURL == "" {
		return false, nil
	}
	events, errorValue := readCalendarBackfillEvents(ctx, transaction, account.ID, target.CalendarURL)
	if errorValue != nil {
		return false, errorValue
	}
	shouldSignalSync := false
	for _, event := range events {
		belongsToTarget := remoteCalendarHrefBelongsToTarget(event.RemoteHref, target)
		if event.DeletedAt != "" {
			enqueued, errorValue := enqueueCalendarDeletionBackfill(ctx, transaction, account, target, event, belongsToTarget)
			if errorValue != nil {
				return false, errorValue
			}
			shouldSignalSync = shouldSignalSync || enqueued
			continue
		}
		changedFields := calendarBackfillChangedFields(event, belongsToTarget)
		if len(changedFields) == 0 {
			continue
		}
		hasPendingPut, errorValue := hasPendingCalendarPutForTargetWithRunner(ctx, transaction, account.ID, event.UID, target)
		if errorValue != nil {
			return false, errorValue
		}
		if hasPendingPut {
			continue
		}
		remoteHref := ""
		remoteETag := ""
		if belongsToTarget {
			remoteHref = event.RemoteHref
			remoteETag = event.RemoteETag
		}
		for _, group := range calendarBackfillFieldGroups(event, changedFields) {
			if errorValue := enqueueCalendarOutboxWithRunner(ctx, transaction, calendarOutboxRow{
				AccountID:     account.ID,
				EventID:       event.ID,
				EventUID:      event.UID,
				Operation:     calendarOutboxOperationPut,
				IfMatchETag:   remoteETag,
				RemoteHref:    remoteHref,
				ChangedFields: group.Fields,
			}, group.ChangedAt); errorValue != nil {
				return false, errorValue
			}
		}
		shouldSignalSync = true
	}
	return shouldSignalSync, nil
}

func enqueueCalendarDeletionBackfill(ctx context.Context, transaction *sql.Tx, account remoteCalendarAccount, target remoteCalendarTarget, event calendarBackfillEvent, belongsToTarget bool) (bool, error) {
	deletedAt := event.FieldChangedAt[calendarEventDeletionClockField]
	if deletedAt.IsZero() || !deletedAt.After(event.AcknowledgedAt[calendarEventDeletionClockField]) {
		return false, nil
	}
	if !belongsToTarget && !calendarBackfillTargetHasEventEvidence(event) {
		return false, nil
	}
	hasPendingDelete, errorValue := hasPendingCalendarDeleteForTargetWithRunner(ctx, transaction, account.ID, event.UID, target)
	if errorValue != nil || hasPendingDelete {
		return false, errorValue
	}
	row := calendarOutboxRow{
		AccountID: account.ID,
		EventID:   event.ID,
		EventUID:  event.UID,
		Operation: calendarOutboxOperationDelete,
	}
	if belongsToTarget {
		row.IfMatchETag = event.RemoteETag
		row.RemoteHref = event.RemoteHref
	}
	if errorValue := enqueueCalendarOutboxWithRunner(ctx, transaction, row, deletedAt.Format(time.RFC3339Nano)); errorValue != nil {
		return false, errorValue
	}
	return true, nil
}

func hasPendingCalendarDeleteForTargetWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, eventUID string, target remoteCalendarTarget) (bool, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT id
FROM calendar_outbox
WHERE account_id = ? AND target_calendar_url = ? AND event_uid = ? AND operation = ? AND status IN (?, ?)
LIMIT 1`, accountID, normalizeCalendarOutboxTargetURL(target.CalendarURL), eventUID, calendarOutboxOperationDelete, calendarOutboxStatusPending, calendarOutboxStatusBlocked)
	if errorValue != nil {
		return false, errorValue
	}
	defer rows.Close()
	return rows.Next(), rows.Err()
}

func readCalendarBackfillEvents(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string) ([]calendarBackfillEvent, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
	SELECT event.id, event.uid, event.raw_ics, event.remote_etag, event.remote_href, event.remote_source, event.updated_at, event.deleted_at
	FROM calendar_events event
WHERE event.deleted_at = '' OR EXISTS (
	SELECT 1 FROM calendar_event_field_clocks clock
	WHERE clock.event_uid = event.uid AND clock.field = ?
)
ORDER BY start_at, title`, calendarEventDeletionClockField)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	events := []calendarBackfillEvent{}
	for rows.Next() {
		var event calendarBackfillEvent
		if errorValue := rows.Scan(&event.ID, &event.UID, &event.RawICS, &event.RemoteETag, &event.RemoteHref, &event.RemoteSource, &event.UpdatedAt, &event.DeletedAt); errorValue != nil {
			return nil, errorValue
		}
		events = append(events, event)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	fieldClocks, errorValue := readCalendarEventFieldClocks(ctx, queryRunner)
	if errorValue != nil {
		return nil, errorValue
	}
	for index := range events {
		events[index].FieldChangedAt = fieldClocks[events[index].UID]
		events[index].AcknowledgedAt, errorValue = readCalendarTargetFieldAcknowledgements(ctx, queryRunner, accountID, calendarURL, events[index].UID)
		if errorValue != nil {
			return nil, errorValue
		}
	}
	return events, nil
}

func calendarBackfillChangedFields(event calendarBackfillEvent, belongsToTarget bool) []string {
	fields := calendarAllUserEditableFields()
	hasTargetEvidence := calendarBackfillTargetHasEventEvidence(event)
	if !belongsToTarget && !hasTargetEvidence {
		return fields
	}
	if event.RemoteSource == remoteCalendarProviderGoogle && !calendarBackfillEventHasFieldClock(event, fields) && !hasTargetEvidence {
		return nil
	}
	result := []string{}
	for _, field := range fields {
		if calendarBackfillFieldChangedAt(event, field).After(event.AcknowledgedAt[field]) {
			result = append(result, field)
		}
	}
	return result
}

func calendarBackfillEventHasFieldClock(event calendarBackfillEvent, fields []string) bool {
	for _, field := range fields {
		if !event.FieldChangedAt[field].IsZero() {
			return true
		}
	}
	return false
}

func calendarBackfillFieldGroups(event calendarBackfillEvent, fields []string) []calendarBackfillFieldGroup {
	fieldsByChangedAt := map[string][]string{}
	for _, field := range fields {
		changedAt := calendarBackfillFieldChangedAt(event, field)
		fieldsByChangedAt[changedAt.UTC().Format(time.RFC3339Nano)] = append(fieldsByChangedAt[changedAt.UTC().Format(time.RFC3339Nano)], field)
	}
	changedTimes := make([]string, 0, len(fieldsByChangedAt))
	for changedAt := range fieldsByChangedAt {
		changedTimes = append(changedTimes, changedAt)
	}
	sort.Strings(changedTimes)
	groups := make([]calendarBackfillFieldGroup, 0, len(changedTimes))
	for _, changedAt := range changedTimes {
		groups = append(groups, calendarBackfillFieldGroup{ChangedAt: changedAt, Fields: fieldsByChangedAt[changedAt]})
	}
	return groups
}

func calendarBackfillFieldChangedAt(event calendarBackfillEvent, field string) time.Time {
	changedAt := event.FieldChangedAt[field]
	if !changedAt.IsZero() {
		return changedAt
	}
	return calendarExistingEventFieldClockBaseline(event.RawICS, event.RemoteHref, event.UpdatedAt)
}

func calendarBackfillTargetHasEventEvidence(event calendarBackfillEvent) bool {
	for _, field := range calendarAllUserEditableFields() {
		if !event.AcknowledgedAt[field].IsZero() {
			return true
		}
	}
	return false
}
