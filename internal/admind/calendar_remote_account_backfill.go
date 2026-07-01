package admind

import (
	"context"
)

type calendarBackfillEvent struct {
	ID         string
	UID        string
	RemoteHref string
}

func (service *Service) enqueueCalendarBackfillOutbox(ctx context.Context, account remoteCalendarAccount) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	shouldSignalSync, errorValue := enqueueCalendarBackfillOutboxWithRunner(ctx, transaction, account)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	if shouldSignalSync {
		service.signalCalendarSyncWakeUp()
	}
	return nil
}

func enqueueCalendarBackfillOutboxWithRunner(ctx context.Context, queryRunner calendarSQLRunner, account remoteCalendarAccount) (bool, error) {
	if !remoteCalendarAccountCanWrite(account) {
		return false, nil
	}
	target := selectedRemoteCalendarTarget(account)
	if target.CalendarURL == "" {
		return false, nil
	}
	events, errorValue := readCalendarBackfillEvents(ctx, queryRunner)
	if errorValue != nil {
		return false, errorValue
	}
	shouldSignalSync := false
	for _, event := range events {
		if remoteCalendarHrefBelongsToTarget(event.RemoteHref, target) {
			continue
		}
		hasPendingPut, errorValue := hasPendingCalendarPutForTargetWithRunner(ctx, queryRunner, account.ID, event.UID, target)
		if errorValue != nil {
			return false, errorValue
		}
		if hasPendingPut {
			continue
		}
		if errorValue := enqueueCalendarOutboxWithRunner(ctx, queryRunner, calendarOutboxRow{
			AccountID:     account.ID,
			EventID:       event.ID,
			EventUID:      event.UID,
			Operation:     calendarOutboxOperationPut,
			ChangedFields: calendarAllUserEditableFields(),
		}); errorValue != nil {
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
