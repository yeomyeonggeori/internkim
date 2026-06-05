package admind

import (
	"context"
	"log"
)

func (service *Service) syncExistingMattermostManagedPosts(ctx context.Context) {
	service.syncExistingFlowMattermostNotifications(ctx)
	service.syncExistingCalendarMattermostLogs(ctx)
}

func (service *Service) syncExistingFlowMattermostNotifications(ctx context.Context) {
	tasks, errorValue := service.readFlowTasksWithMattermostPosts(ctx)
	if errorValue != nil {
		log.Printf("Flow Mattermost existing notification sync failed: %v", errorValue)
		return
	}
	for _, task := range tasks {
		service.syncFlowMattermostNotification(ctx, task)
	}
}

func (service *Service) syncExistingCalendarMattermostLogs(ctx context.Context) {
	events, errorValue := service.readCalendarEventsWithMattermostPosts(ctx)
	if errorValue != nil {
		log.Printf("calendar Mattermost existing log sync failed: %v", errorValue)
		return
	}
	for _, event := range events {
		service.syncCalendarMattermostLog(ctx, event)
	}
}
