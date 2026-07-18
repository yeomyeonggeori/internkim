package admind

import (
	"context"
	"fmt"
	"time"
)

func (service *Service) persistObservedCalendarRemoteEventStateBatch(ctx context.Context, accountID string, calendarURL string, events []calendarEvent, reservedObservedAt time.Time) error {
	if len(events) == 0 {
		return nil
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	for _, event := range events {
		if _, errorValue := service.persistObservedCalendarRemoteEventStateWithTransaction(ctx, transaction, accountID, calendarURL, event, reservedObservedAt); errorValue != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("persist observed calendar event %s: %w", event.UID, errorValue)
		}
	}
	return transaction.Commit()
}
