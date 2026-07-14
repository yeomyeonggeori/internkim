package admind

import (
	"context"
	"database/sql"
	"fmt"
)

func (service *Service) insertAttendanceEventOverride(ctx context.Context, database *sql.DB, override attendanceEventOverride) error {
	return service.withAttendanceEventMutation(ctx, database, func(transaction *sql.Tx) ([]string, error) {
		event, found, errorValue := readAttendanceEventByIDInTransaction(ctx, transaction, override.EventID)
		if errorValue != nil {
			return nil, errorValue
		}
		if !found {
			return nil, fmt.Errorf("attendance event %q was not found", override.EventID)
		}
		dates, errorValue := attendanceEventCacheDatesInTransaction(ctx, transaction, event)
		if errorValue != nil {
			return nil, errorValue
		}
		_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO attendance_event_overrides (
	id, event_id, edited_by, edited_at, reason,
	original_occurred_at, original_local_date, original_local_time, original_location_id, original_location_name,
	override_occurred_at, override_local_date, override_local_time, override_location_id, override_location_name
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			override.ID,
			override.EventID,
			override.EditedBy,
			override.EditedAt,
			override.Reason,
			override.OriginalOccurredAt,
			override.OriginalLocalDate,
			override.OriginalLocalTime,
			override.OriginalLocationID,
			override.OriginalLocationName,
			override.OverrideOccurredAt,
			override.OverrideLocalDate,
			override.OverrideLocalTime,
			override.OverrideLocationID,
			override.OverrideLocationName,
		)
		if errorValue != nil {
			return nil, fmt.Errorf("insert attendance event override: %w", errorValue)
		}
		dates = append(dates, override.OriginalLocalDate, override.OverrideLocalDate)
		return dates, nil
	})
}
