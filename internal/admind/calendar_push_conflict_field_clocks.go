package admind

import (
	"context"
	"database/sql"
	"time"
)

func (service *Service) persistCalendarPushConflictFieldClocksWithTransaction(ctx context.Context, transaction *sql.Tx, row calendarOutboxRow, remoteEvent calendarEvent, remoteWinningFields []string, fallbackChangedAt time.Time) error {
	if len(remoteWinningFields) == 0 {
		return nil
	}
	fieldClocks, errorValue := readCalendarEventFieldClocksForUID(ctx, transaction, row.EventUID)
	if errorValue != nil {
		return errorValue
	}
	remoteWinningFields = calendarFieldsNotChangedAfter(remoteWinningFields, fieldClocks, parseCalendarConflictTime(row.CreatedAt))
	if len(remoteWinningFields) == 0 {
		return nil
	}
	changedAt := parseCalendarConflictTime(remoteEvent.RemoteModifiedAt)
	if changedAt.IsZero() {
		changedAt, errorValue = service.allocateCalendarConflictTime(ctx, transaction, row.EventUID, fallbackChangedAt)
		if errorValue != nil {
			return errorValue
		}
	}
	existingAcknowledgements, errorValue := readCalendarTargetFieldAcknowledgements(ctx, transaction, row.AccountID, row.TargetCalendarURL, row.EventUID)
	if errorValue != nil {
		return errorValue
	}
	acknowledgements, errorValue := persistAdvancedCalendarEventFieldClocks(ctx, transaction, row.EventUID, remoteWinningFields, changedAt, existingAcknowledgements)
	if errorValue != nil {
		return errorValue
	}
	return persistCalendarTargetFieldAcknowledgements(ctx, transaction, row.AccountID, row.TargetCalendarURL, row.EventUID, acknowledgements)
}

func calendarFieldsNotChangedAfter(fields []string, fieldClocks map[string]time.Time, boundary time.Time) []string {
	result := []string{}
	for _, field := range fields {
		if !fieldClocks[field].After(boundary) {
			result = append(result, field)
		}
	}
	return result
}
