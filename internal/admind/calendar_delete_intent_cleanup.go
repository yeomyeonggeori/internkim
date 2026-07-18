package admind

import (
	"context"
	"time"
)

const (
	calendarDeleteIntentRetentionDuration = 30 * 24 * time.Hour
	calendarDeleteIntentCleanupInterval   = 24 * time.Hour
)

func (service *Service) pruneResolvedCalendarDeleteIntents(ctx context.Context, now time.Time) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	cutoff := now.UTC().Add(-calendarDeleteIntentRetentionDuration).Format(time.RFC3339Nano)
	_, errorValue = database.ExecContext(ctx, `
DELETE FROM calendar_delete_intents
WHERE status IN (?, ?, ?) AND resolved_at != '' AND resolved_at < ?`,
		calendarDeleteIntentStatusCanceled,
		calendarDeleteIntentStatusExecuted,
		calendarDeleteIntentStatusConflicted,
		cutoff,
	)
	return errorValue
}
