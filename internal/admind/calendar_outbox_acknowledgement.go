package admind

import (
	"context"
	"time"
)

func (service *Service) acknowledgeCalendarOutboxBatch(ctx context.Context, row calendarOutboxRow) error {
	acknowledgements := calendarOutboxBatchAcknowledgements(row)
	if len(acknowledgements) == 0 {
		return nil
	}
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := persistCalendarTargetFieldAcknowledgements(ctx, transaction, row.AccountID, row.TargetCalendarURL, row.EventUID, acknowledgements); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func calendarOutboxBatchAcknowledgements(row calendarOutboxRow) map[string]time.Time {
	if row.Operation == calendarOutboxOperationDelete {
		return calendarFieldAcknowledgementsAt([]string{calendarEventDeletionClockField}, parseCalendarConflictTime(row.CreatedAt))
	}
	if row.Operation != calendarOutboxOperationPut {
		return nil
	}
	result := map[string]time.Time{}
	fallback := parseCalendarConflictTime(row.CreatedAt)
	for _, field := range row.ChangedFields {
		changedAt := row.FieldChangedAt[field]
		if changedAt.IsZero() {
			changedAt = fallback
		}
		result[field] = changedAt
	}
	return result
}
