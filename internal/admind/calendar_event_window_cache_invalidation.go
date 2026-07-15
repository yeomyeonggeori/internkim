package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type calendarEventWindowMutationRange struct {
	StartISO string
	EndISO   string
}

func invalidateCalendarEventWindowCache(ctx context.Context, transaction *sql.Tx, events ...calendarEvent) error {
	ranges := map[calendarEventWindowMutationRange]struct{}{}
	for _, event := range events {
		mutationRange, errorValue := calendarEventWindowMutationRangeFor(event)
		if errorValue != nil {
			return errorValue
		}
		ranges[mutationRange] = struct{}{}
	}
	for mutationRange := range ranges {
		if _, errorValue := transaction.ExecContext(ctx, `
			DELETE FROM calendar_event_window_cache_entries
			WHERE end_at > ? AND start_at < ?`, mutationRange.StartISO, mutationRange.EndISO); errorValue != nil {
			return fmt.Errorf("invalidate overlapping calendar event window cache entries: %w", errorValue)
		}
	}
	result, errorValue := transaction.ExecContext(ctx, "UPDATE calendar_event_window_source_state SET revision = revision + 1 WHERE id = 1")
	if errorValue != nil {
		return fmt.Errorf("increment calendar event window source revision: %w", errorValue)
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return fmt.Errorf("read calendar event window source revision update result: %w", errorValue)
	}
	if rowsAffected != 1 {
		return fmt.Errorf("increment calendar event window source revision: updated %d rows", rowsAffected)
	}
	return nil
}

func readCalendarEventWindowMutationEvent(ctx context.Context, transaction *sql.Tx, eventID string) (calendarEvent, bool, error) {
	var event calendarEvent
	event.ID = strings.TrimSpace(eventID)
	errorValue := transaction.QueryRowContext(ctx, `
		SELECT start_at, end_at
		FROM calendar_events
		WHERE id = ? AND deleted_at = ''`, event.ID).Scan(&event.StartISO, &event.EndISO)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return calendarEvent{}, false, nil
	}
	if errorValue != nil {
		return calendarEvent{}, false, fmt.Errorf("read previous calendar event range for cache invalidation: %w", errorValue)
	}
	return event, true, nil
}

func calendarEventWindowMutationRangeFor(event calendarEvent) (calendarEventWindowMutationRange, error) {
	startTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(event.StartISO))
	if errorValue != nil {
		return calendarEventWindowMutationRange{}, fmt.Errorf("parse calendar event %s start time for cache invalidation: %w", event.ID, errorValue)
	}
	endTime, errorValue := time.Parse(time.RFC3339, strings.TrimSpace(event.EndISO))
	if errorValue != nil {
		return calendarEventWindowMutationRange{}, fmt.Errorf("parse calendar event %s end time for cache invalidation: %w", event.ID, errorValue)
	}
	if !startTime.Before(endTime) {
		return calendarEventWindowMutationRange{}, fmt.Errorf("calendar event %s cache invalidation range is empty", event.ID)
	}
	return calendarEventWindowMutationRange{
		StartISO: formatCalendarEventWindowCacheTimestamp(startTime),
		EndISO:   formatCalendarEventWindowCacheTimestamp(endTime),
	}, nil
}
