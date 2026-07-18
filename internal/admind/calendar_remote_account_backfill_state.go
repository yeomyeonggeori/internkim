package admind

import (
	"context"
	"strings"
	"time"
)

func readCalendarBackfillState(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string) ([]calendarBackfillEvent, error) {
	events, errorValue := readCalendarBackfillEventRows(ctx, queryRunner)
	if errorValue != nil {
		return nil, errorValue
	}
	fieldClocks, errorValue := readCalendarEventFieldClocks(ctx, queryRunner)
	if errorValue != nil {
		return nil, errorValue
	}
	acknowledgements, errorValue := readCalendarBackfillAcknowledgements(ctx, queryRunner, accountID, calendarURL)
	if errorValue != nil {
		return nil, errorValue
	}
	pendingActions, errorValue := readCalendarBackfillPendingActions(ctx, queryRunner, accountID, calendarURL)
	if errorValue != nil {
		return nil, errorValue
	}
	for index := range events {
		events[index].FieldChangedAt = fieldClocks[events[index].UID]
		events[index].AcknowledgedAt = acknowledgements[events[index].UID]
		events[index].PendingActions = pendingActions[events[index].UID]
	}
	return events, nil
}

func readCalendarBackfillEventRows(ctx context.Context, queryRunner calendarSQLRunner) ([]calendarBackfillEvent, error) {
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
	return events, rows.Err()
}

func readCalendarBackfillAcknowledgements(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string) (map[string]map[string]time.Time, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT event_uid, field, acknowledged_at
FROM calendar_target_field_acknowledgements
WHERE account_id = ? AND calendar_url = ?`, strings.TrimSpace(accountID), canonicalCalendarTargetURL(calendarURL))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := map[string]map[string]time.Time{}
	for rows.Next() {
		var eventUID string
		var field string
		var acknowledgedAt string
		if errorValue := rows.Scan(&eventUID, &field, &acknowledgedAt); errorValue != nil {
			return nil, errorValue
		}
		if result[eventUID] == nil {
			result[eventUID] = map[string]time.Time{}
		}
		result[eventUID][field] = parseCalendarConflictTime(acknowledgedAt)
	}
	return result, rows.Err()
}

func readCalendarBackfillPendingActions(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string) (map[string]map[string]bool, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT event_uid, operation
FROM calendar_outbox
WHERE account_id = ? AND target_calendar_url = ? AND status IN (?, ?)`,
		strings.TrimSpace(accountID),
		normalizeCalendarOutboxTargetURL(calendarURL),
		calendarOutboxStatusPending,
		calendarOutboxStatusBlocked,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := map[string]map[string]bool{}
	for rows.Next() {
		var eventUID string
		var operation string
		if errorValue := rows.Scan(&eventUID, &operation); errorValue != nil {
			return nil, errorValue
		}
		if result[eventUID] == nil {
			result[eventUID] = map[string]bool{}
		}
		result[eventUID][operation] = true
	}
	return result, rows.Err()
}
