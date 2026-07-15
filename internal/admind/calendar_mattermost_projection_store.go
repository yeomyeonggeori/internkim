package admind

import (
	"context"
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
	event, found, errorValue := readCalendarEventWindowMutationEvent(ctx, transaction, eventID)
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
	if errorValue := invalidateCalendarEventWindowCache(ctx, transaction, event); errorValue != nil {
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit calendar Mattermost projection update: %w", errorValue)
	}
	return nil
}
