package admind

import (
	"context"
	"testing"
)

func TestATaskWrittenBeforeTheOutboxIsQueuedOnce(t *testing.T) {
	service := newFlowCentralTestService(t)
	older := flowNotificationTestTask("진행")
	older.ID = "written-before-the-outbox"
	if errorValue := service.writeMirroredFlowTask(context.Background(), older); errorValue != nil {
		t.Fatal(errorValue)
	}
	if entries := flowCentralOutboxEntries(t, service); len(entries) != 0 {
		t.Fatalf("a mirrored write does not queue: %+v", entries)
	}

	queued, errorValue := service.queueFlowTasksTheCentralPlaneNeverSaw(context.Background())

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if queued != 1 {
		t.Fatalf("queued = %d", queued)
	}
	entries := flowCentralOutboxEntries(t, service)
	if len(entries) != 1 || entries[0].TaskID != older.ID || entries[0].Intent != flowCentralWriteIntent {
		t.Fatalf("the drain has to carry it the ordinary way: %+v", entries)
	}
}

func TestATaskTheCentralPlaneAlreadyKnowsIsNotQueuedAgain(t *testing.T) {
	service := newFlowCentralTestService(t)
	known := flowNotificationTestTask("진행")
	known.ID = "already-carried"
	if errorValue := service.writeMirroredFlowTask(context.Background(), known); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.rememberFlowCentralIdentityForTask(context.Background(), known.ID, "central-1"); errorValue != nil {
		t.Fatal(errorValue)
	}

	queued, errorValue := service.queueFlowTasksTheCentralPlaneNeverSaw(context.Background())

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if queued != 0 {
		t.Fatalf("queued = %d", queued)
	}
}
