package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (service *Service) updateCalendarEventMattermostPostID(ctx context.Context, eventID string, postID string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return fmt.Errorf("begin calendar Mattermost projection update: %w", errorValue)
	}
	defer transaction.Rollback()
	event, isDeleted, found, errorValue := readCalendarMattermostProjectionMutationEvent(ctx, transaction, eventID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return transaction.Commit()
	}
	trimmedPostID := strings.TrimSpace(postID)
	postCreatedAt := ""
	if trimmedPostID != "" {
		postCreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	if _, errorValue := transaction.ExecContext(ctx, "UPDATE calendar_events SET mattermost_post_id = ?, mattermost_post_created_at = ?, updated_at = ? WHERE id = ?", trimmedPostID, postCreatedAt, updatedAt, event.ID); errorValue != nil {
		return fmt.Errorf("update calendar Mattermost projection: %w", errorValue)
	}
	if !isDeleted {
		if errorValue := service.invalidateCalendarEventWindowCache(ctx, transaction, event); errorValue != nil {
			return errorValue
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit calendar Mattermost projection update: %w", errorValue)
	}
	return nil
}

func readCalendarMattermostProjectionMutationEvent(ctx context.Context, transaction *sql.Tx, eventID string) (calendarEvent, bool, bool, error) {
	var event calendarEvent
	var deletedAt string
	event.ID = strings.TrimSpace(eventID)
	errorValue := transaction.QueryRowContext(ctx, `
		SELECT start_at, end_at, deleted_at
		FROM calendar_events
		WHERE id = ?`, event.ID).Scan(&event.StartISO, &event.EndISO, &deletedAt)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return calendarEvent{}, false, false, nil
	}
	if errorValue != nil {
		return calendarEvent{}, false, false, fmt.Errorf("read calendar Mattermost projection mutation event: %w", errorValue)
	}
	return event, strings.TrimSpace(deletedAt) != "", true, nil
}
