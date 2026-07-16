package admind

import (
	"context"
	"database/sql"
	"time"
)

type calendarBackfillEvent struct {
	ID         string
	UID        string
	RemoteHref string
}

func enqueueCalendarBackfillOutboxWithRunner(ctx context.Context, transaction *sql.Tx, candidateClock *calendarConflictCandidateClock, account remoteCalendarAccount, candidate time.Time) (bool, error) {
	if !remoteCalendarAccountCanWrite(account) {
		return false, nil
	}
	target := selectedRemoteCalendarTarget(account)
	if target.CalendarURL == "" {
		return false, nil
	}
	events, errorValue := readCalendarBackfillEvents(ctx, transaction)
	if errorValue != nil {
		return false, errorValue
	}
	shouldSignalSync := false
	for _, event := range events {
		if remoteCalendarHrefBelongsToTarget(event.RemoteHref, target) {
			continue
		}
		hasPendingPut, errorValue := hasPendingCalendarPutForTargetWithRunner(ctx, transaction, account.ID, event.UID, target)
		if errorValue != nil {
			return false, errorValue
		}
		if hasPendingPut {
			continue
		}
		logicalTime, errorValue := allocateCalendarConflictTimeWithCandidateClock(ctx, transaction, candidateClock, event.UID, candidate)
		if errorValue != nil {
			return false, errorValue
		}
		if errorValue := enqueueCalendarOutboxWithRunner(ctx, transaction, calendarOutboxRow{
			AccountID:     account.ID,
			EventID:       event.ID,
			EventUID:      event.UID,
			Operation:     calendarOutboxOperationPut,
			ChangedFields: calendarAllUserEditableFields(),
		}, logicalTime.Format(time.RFC3339Nano)); errorValue != nil {
			return false, errorValue
		}
		shouldSignalSync = true
	}
	return shouldSignalSync, nil
}

func readCalendarBackfillEvents(ctx context.Context, queryRunner calendarSQLRunner) ([]calendarBackfillEvent, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT id, uid, remote_href
FROM calendar_events
WHERE deleted_at = ''
ORDER BY start_at, title`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	events := []calendarBackfillEvent{}
	for rows.Next() {
		var event calendarBackfillEvent
		if errorValue := rows.Scan(&event.ID, &event.UID, &event.RemoteHref); errorValue != nil {
			return nil, errorValue
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
