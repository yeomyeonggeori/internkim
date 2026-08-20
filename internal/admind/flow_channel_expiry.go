package admind

import (
	"context"
	"log"
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
