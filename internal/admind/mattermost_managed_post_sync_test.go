package admind

import (
	"context"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestReadFlowTasksWithMattermostPostsSkipsRetiredNotifications(t *testing.T) {
	service := NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
	ctx := context.Background()

	postedTask := flowNotificationTestTask("완료")
	postedTask.ID = "posted-task"
	writeFlowTaskWithSettledPost(t, service, postedTask, "posted-post")

	retiredTask := flowNotificationTestTask("완료")
	retiredTask.ID = "retired-task"
	writeFlowTaskWithSettledPost(t, service, retiredTask, "retired-post")
	backdateFlowMattermostPostCreatedAt(t, service, retiredTask.ID, time.Now().UTC().Add(-20*24*time.Hour))
	settleFlowMattermostPost(t, service, retiredTask.ID, "")

	queuedTask := flowNotificationTestTask("요청")
	queuedTask.ID = "queued-task"
	if errorValue := service.writeFlowTask(ctx, queuedTask); errorValue != nil {
		t.Fatal(errorValue)
	}

	tasks, errorValue := service.readFlowTasksWithMattermostPosts(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	taskIDs := []string{}
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
	}
	if slices.Contains(taskIDs, retiredTask.ID) {
		t.Fatalf("a retention-retired notification would be posted again on restart: %v", taskIDs)
	}
	if !slices.Contains(taskIDs, postedTask.ID) || !slices.Contains(taskIDs, queuedTask.ID) {
		t.Fatalf("reconcile lost a task that still needs a post: %v", taskIDs)
	}
}

func TestReadCalendarEventIDsRequiringMattermostProjectionSkipsRetiredLogs(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()

	loggedEvent := calendarTestEvent("logged-event", "Design review", "Bring agenda")
	writeCalendarEventWithSettledPost(t, service, loggedEvent, "logged-post")

	retiredEvent := calendarTestEvent("retired-event", "Old retro", "Bring notes")
	writeCalendarEventWithSettledPost(t, service, retiredEvent, "retired-post")
	settleCalendarMattermostPost(t, service, retiredEvent.ID, "")

	eventIDs, errorValue := service.readCalendarEventIDsRequiringMattermostProjection(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if slices.Contains(eventIDs, retiredEvent.ID) {
		t.Fatalf("a retention-retired calendar log would be posted again on restart: %v", eventIDs)
	}
	if !slices.Contains(eventIDs, loggedEvent.ID) {
		t.Fatalf("reconcile lost an event that still has a log post: %v", eventIDs)
	}
}

func writeFlowTaskWithSettledPost(t *testing.T, service *Service, task flowTask, postID string) {
	t.Helper()
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	settleFlowMattermostPost(t, service, task.ID, postID)
}

func settleFlowMattermostPost(t *testing.T, service *Service, taskID string, postID string) {
	t.Helper()
	ctx := context.Background()
	if errorValue := service.updateFlowTaskMattermostPostID(ctx, taskID, postID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.deleteFlowMattermostProjectionOutbox(ctx, taskID); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func writeCalendarEventWithSettledPost(t *testing.T, service *Service, event calendarEvent, postID string) {
	t.Helper()
	if errorValue := service.writeCalendarEvent(context.Background(), event); errorValue != nil {
		t.Fatal(errorValue)
	}
	settleCalendarMattermostPost(t, service, event.ID, postID)
}

func settleCalendarMattermostPost(t *testing.T, service *Service, eventID string, postID string) {
	t.Helper()
	ctx := context.Background()
	if errorValue := service.updateCalendarEventMattermostPostID(ctx, eventID, postID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.deleteCalendarMattermostProjectionOutbox(ctx, eventID); errorValue != nil {
		t.Fatal(errorValue)
	}
}
