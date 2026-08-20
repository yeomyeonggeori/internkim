package admind

import (
	"context"
	"path/filepath"
	"testing"
)

func newFlowCentralTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(Configuration{FlowDatabasePath: filepath.Join(t.TempDir(), "flow.sqlite")})
}

func TestWritingATaskQueuesItForTheCentralPlane(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "queued-task"

	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	entries := flowCentralOutboxEntries(t, service)
	if len(entries) != 1 || entries[0].TaskID != task.ID {
		t.Fatalf("a task the central plane has not seen has to be queued, got %+v", entries)
	}
	if entries[0].Intent != flowCentralWriteIntent {
		t.Fatalf("intent = %q", entries[0].Intent)
	}
}

func TestRemovingATaskQueuesTheRemovalRatherThanForgettingIt(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "removed-task"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.deleteFlowTaskByID(context.Background(), task.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	entries := flowCentralOutboxEntries(t, service)
	if len(entries) != 1 || entries[0].Intent != flowCentralDeleteIntent {
		t.Fatalf("a task removed here is still held there, so the removal has to travel: %+v", entries)
	}
}

func TestAFailedAttemptKeepsTheTaskQueuedWithItsReason(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "stubborn-task"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.markFlowCentralOutboxAttempt(context.Background(), task.ID, context.DeadlineExceeded); errorValue != nil {
		t.Fatal(errorValue)
	}

	entries := flowCentralOutboxEntries(t, service)
	if len(entries) != 1 || entries[0].AttemptCount != 1 {
		t.Fatalf("the attempt has to be counted, got %+v", entries)
	}
	if entries[0].LastError != context.DeadlineExceeded.Error() {
		t.Fatalf("a queue that forgets why it failed cannot be diagnosed: %q", entries[0].LastError)
	}
}

func TestATaskTheCentralPlaneTookLeavesTheQueue(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "accepted-task"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.deleteFlowCentralOutbox(context.Background(), task.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	if entries := flowCentralOutboxEntries(t, service); len(entries) != 0 {
		t.Fatalf("queue = %+v", entries)
	}
}

func flowCentralOutboxEntries(t *testing.T, service *Service) []flowCentralOutboxEntry {
	t.Helper()
	entries, errorValue := service.readFlowCentralOutbox(context.Background(), 100)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return entries
}
