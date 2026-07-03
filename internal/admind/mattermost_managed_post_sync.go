package admind

import (
	"context"
	"log"
	"time"
)

const mattermostManagedPostReconcilePhaseTimeout = 30 * time.Second

// Each phase gets its own timeout budget carved from ctx so a slow flow
// reconcile cannot starve the calendar reconcile that follows it.
func (service *Service) syncExistingMattermostManagedPosts(ctx context.Context) {
	service.reconcileFlowMattermostProjectionsWithTimeout(ctx)
	service.reconcileCalendarMattermostProjectionsWithTimeout(ctx)
}

func (service *Service) reconcileFlowMattermostProjectionsWithTimeout(ctx context.Context) {
	syncContext, cancel := context.WithTimeout(ctx, mattermostManagedPostReconcilePhaseTimeout)
	defer cancel()
	service.reconcileFlowMattermostProjections(syncContext)
}

func (service *Service) reconcileCalendarMattermostProjectionsWithTimeout(ctx context.Context) {
	syncContext, cancel := context.WithTimeout(ctx, mattermostManagedPostReconcilePhaseTimeout)
	defer cancel()
	service.reconcileCalendarMattermostProjections(syncContext)
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
