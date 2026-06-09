package admind

import (
	"context"
	"log"
)

func (service *Service) syncExistingMattermostManagedPosts(ctx context.Context) {
	service.reconcileFlowMattermostProjections(ctx)
	service.reconcileCalendarMattermostProjections(ctx)
}

func (service *Service) reconcileFlowMattermostProjections(ctx context.Context) {
	tasks, errorValue := service.readFlowTasksWithMattermostPosts(ctx)
	if errorValue != nil {
		log.Printf("Flow Mattermost existing notification sync failed: %v", errorValue)
		return
	}
	for _, task := range tasks {
		service.applyFlowMattermostProjection(ctx, task)
	}
	service.drainFlowMattermostProjectionOutbox(ctx)
}

func (service *Service) reconcileCalendarMattermostProjections(ctx context.Context) {
	eventIDs, errorValue := service.readCalendarEventIDsRequiringMattermostProjection(ctx)
	if errorValue != nil {
		log.Printf("calendar Mattermost existing log sync failed: %v", errorValue)
		return
	}
	for _, eventID := range eventIDs {
		if errorValue := service.applyCalendarMattermostProjectionByID(ctx, eventID); errorValue != nil {
			log.Printf("calendar Mattermost projection reconcile failed for %s: %v", eventID, errorValue)
		}
	}
	service.drainCalendarMattermostProjectionOutbox(ctx)
}
