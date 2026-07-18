package admind

import (
	"context"
	"fmt"
)

func (service *Service) prepareCalendarOutboxPushBatches(ctx context.Context, account remoteCalendarAccount, rows []calendarOutboxRow) ([]calendarOutboxRow, bool, error) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, false, errorValue
	}
	defer database.Close()
	persistedTarget, errorValue := readActiveCalendarOutboxTargetURLWithRunner(ctx, database, account.ID)
	if errorValue != nil {
		return nil, false, errorValue
	}
	cycleTarget := canonicalCalendarTargetURL(activeRemoteCalendarTarget(account).CalendarURL)
	if persistedTarget == "" || persistedTarget != cycleTarget {
		return nil, false, nil
	}
	activeBatches := []calendarOutboxRow{}
	staleBatches := []calendarOutboxRow{}
	for _, row := range aggregateCalendarOutboxRows(rows) {
		rowTarget := canonicalCalendarTargetURL(row.TargetCalendarURL)
		if rowTarget == "" {
			continue
		}
		if rowTarget == persistedTarget {
			activeBatches = append(activeBatches, row)
			continue
		}
		staleBatches = append(staleBatches, row)
	}
	if len(staleBatches) == 0 {
		return activeBatches, true, nil
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return nil, false, errorValue
	}
	for _, row := range staleBatches {
		if errorValue := deleteCalendarOutboxBatchWithRunner(ctx, transaction, row); errorValue != nil {
			_ = transaction.Rollback()
			return nil, false, fmt.Errorf("delete retargeted calendar outbox batch %d: %w", row.ID, errorValue)
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return nil, false, errorValue
	}
	return activeBatches, true, nil
}
