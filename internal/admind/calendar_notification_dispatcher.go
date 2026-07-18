package admind

import (
	"context"
	"log/slog"
	"time"
)

func dispatchCalendarNotificationReconciliations(ctx context.Context, liveInput <-chan calendarNotificationReconciliationJob, repairInput <-chan calendarNotificationReconciliationJob, jobs chan<- calendarNotificationReconciliationJob) {
	defer close(jobs)
	consecutiveLiveJobs := 0
	for {
		job, found := nextCalendarNotificationReconciliation(ctx, liveInput, repairInput, consecutiveLiveJobs)
		if !found {
			return
		}
		select {
		case <-ctx.Done():
			return
		case jobs <- job:
		}
		if job.live {
			consecutiveLiveJobs++
		} else {
			consecutiveLiveJobs = 0
		}
	}
}

func nextCalendarNotificationReconciliation(ctx context.Context, liveInput <-chan calendarNotificationReconciliationJob, repairInput <-chan calendarNotificationReconciliationJob, consecutiveLiveJobs int) (calendarNotificationReconciliationJob, bool) {
	if consecutiveLiveJobs >= calendarNotificationLiveBurstLimit {
		select {
		case job := <-repairInput:
			return job, true
		default:
		}
	} else {
		select {
		case job := <-liveInput:
			return job, true
		default:
		}
	}
	select {
	case <-ctx.Done():
		return calendarNotificationReconciliationJob{}, false
	case job := <-liveInput:
		return job, true
	case job := <-repairInput:
		return job, true
	}
}

func (service *Service) queueCalendarNotificationReconciliation(ctx context.Context, liveInput chan<- calendarNotificationReconciliationJob, repairInput chan<- calendarNotificationReconciliationJob, job calendarNotificationReconciliationJob) {
	input := repairInput
	if job.live {
		input = liveInput
	}
	select {
	case <-ctx.Done():
	case input <- job:
		return
	default:
	}
	service.calendarNotificationMutex.Lock()
	if service.calendarNotificationStates[job.eventID] == job.state && job.state.generation == job.generation {
		job.state.queued = false
		job.state.queuedLive = false
		job.state.pending = true
		job.state.livePending = job.state.livePending || job.live
	}
	service.calendarNotificationMutex.Unlock()
}

func (service *Service) runCalendarNotificationReconciliationWorker(ctx context.Context, jobs <-chan calendarNotificationReconciliationJob) {
	for job := range jobs {
		service.processCalendarNotificationReconciliationJob(ctx, job)
	}
}

func (service *Service) processCalendarNotificationReconciliationJob(ctx context.Context, job calendarNotificationReconciliationJob) {
	service.calendarNotificationMutex.Lock()
	if service.calendarNotificationStates[job.eventID] != job.state || job.state.generation != job.generation {
		service.calendarNotificationMutex.Unlock()
		return
	}
	job.state.queued = false
	job.state.queuedLive = false
	job.state.running = true
	job.state.pending = false
	job.state.livePending = false
	reconciler := job.state.reconciler
	service.calendarNotificationMutex.Unlock()
	attemptContext, cancel := context.WithTimeout(ctx, calendarNotificationReconciliationTimeout)
	errorValue := reconciler(attemptContext, job.eventID)
	cancel()
	service.finishCalendarNotificationReconciliationJob(ctx, job, errorValue)
}

func (service *Service) finishCalendarNotificationReconciliationJob(ctx context.Context, job calendarNotificationReconciliationJob, reconciliationError error) {
	service.calendarNotificationMutex.Lock()
	if service.calendarNotificationStates[job.eventID] != job.state {
		service.calendarNotificationMutex.Unlock()
		return
	}
	job.state.running = false
	if reconciliationError != nil {
		job.state.pending = true
		job.state.livePending = job.state.livePending || job.live
		job.state.retryScheduled = true
		retryDelay := calendarNotificationRetryDelay(job.eventID, job.state.retryDelay)
		job.state.retryDelay = nextCalendarNotificationRetryDelay(job.state.retryDelay)
		job.state.retryAt = time.Now().Add(retryDelay)
		service.calendarNotificationMutex.Unlock()
		slog.WarnContext(ctx, "calendar notification reconciliation failed", "event_id", job.eventID, "error", reconciliationError)
		return
	}
	job.state.retryDelay = calendarNotificationRetryBaseDelay
	if !job.state.pending {
		delete(service.calendarNotificationStates, job.eventID)
		service.calendarNotificationMutex.Unlock()
		return
	}
	nextJob, shouldQueue := service.queueCalendarNotificationReconciliationLocked(job.eventID, job.state, job.state.livePending)
	liveInput := service.calendarNotificationLive
	repairInput := service.calendarNotificationRepair
	workerContext := service.calendarNotificationCtx
	service.calendarNotificationMutex.Unlock()
	if shouldQueue {
		service.queueCalendarNotificationReconciliation(workerContext, liveInput, repairInput, nextJob)
	}
}

func (service *Service) scheduleCalendarNotificationReconciliations(ctx context.Context) {
	ticker := time.NewTicker(calendarNotificationSchedulerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			service.enqueueReadyCalendarNotificationReconciliations(ctx, now)
		}
	}
}

func (service *Service) enqueueReadyCalendarNotificationReconciliations(ctx context.Context, now time.Time) {
	service.calendarNotificationMutex.Lock()
	jobs := make([]calendarNotificationReconciliationJob, 0, calendarNotificationInputBufferSize*2)
	for eventID, state := range service.calendarNotificationStates {
		if len(jobs) >= calendarNotificationInputBufferSize*2 {
			break
		}
		if state.running || state.queued || !state.pending {
			continue
		}
		if state.retryScheduled && now.Before(state.retryAt) {
			continue
		}
		state.retryScheduled = false
		job, shouldQueue := service.queueCalendarNotificationReconciliationLocked(eventID, state, state.livePending)
		if shouldQueue {
			jobs = append(jobs, job)
		}
	}
	liveInput := service.calendarNotificationLive
	repairInput := service.calendarNotificationRepair
	workerContext := service.calendarNotificationCtx
	service.calendarNotificationMutex.Unlock()
	for _, job := range jobs {
		service.queueCalendarNotificationReconciliation(workerContext, liveInput, repairInput, job)
	}
}
