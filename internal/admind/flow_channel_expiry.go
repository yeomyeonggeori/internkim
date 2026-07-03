package admind

import (
	"context"
	"log"
	"strings"
	"time"
)

func (service *Service) sweepExpiredFlowMattermostNotifications(ctx context.Context) {
	taskIDs, errorValue := service.readExpiredFlowMattermostPostTaskIDs(ctx, time.Now().UTC().Add(-mattermostChannelPostRetentionDuration))
	if errorValue != nil {
		log.Printf("flow Mattermost expiry sweep query failed: %v", errorValue)
		return
	}
	if len(taskIDs) == 0 {
		return
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		log.Printf("flow Mattermost expiry sweep admin token failed: %v", errorValue)
		return
	}
	for _, taskID := range taskIDs {
		task, found, errorValue := service.readFlowTaskByID(ctx, taskID)
		if errorValue != nil || !found {
			continue
		}
		if _, errorValue := service.deleteFlowMattermostNotification(ctx, adminToken, task); errorValue != nil {
			log.Printf("flow Mattermost expiry sweep delete failed for %s: %v", taskID, errorValue)
		}
	}
}

func (service *Service) readExpiredFlowMattermostPostTaskIDs(ctx context.Context, cutoff time.Time) ([]string, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id
FROM flow_tasks
WHERE mattermost_post_id != '' AND mattermost_post_created_at != '' AND mattermost_post_created_at < ?`,
		cutoff.Format(time.RFC3339))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	taskIDs := []string{}
	for rows.Next() {
		var taskID string
		if errorValue := rows.Scan(&taskID); errorValue != nil {
			return nil, errorValue
		}
		taskIDs = append(taskIDs, strings.TrimSpace(taskID))
	}
	return taskIDs, rows.Err()
}
