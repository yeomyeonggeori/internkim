package admind

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func failFlowCentralOutboxEntry(t *testing.T, service *Service, taskID string, times int, failure error) {
	t.Helper()
	for attempt := 0; attempt < times; attempt++ {
		if errorValue := service.markFlowCentralOutboxAttempt(context.Background(), taskID, failure); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
}

func TestAnEntryThatKeepsFailingStopsBeingDrained(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "unqueueable-task"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	failFlowCentralOutboxEntry(t, service, task.ID, flowCentralOutboxAttemptLimit, errors.New("who has no messenger account here"))

	if entries := flowCentralOutboxEntries(t, service); len(entries) != 0 {
		t.Fatalf("a refusal that never changes has to stop being retried, got %+v", entries)
	}
}

func TestAHeldBackEntryKeepsItsWorkAndItsReason(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "held-task"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	failFlowCentralOutboxEntry(t, service, task.ID, flowCentralOutboxAttemptLimit, errors.New("no messenger account here"))

	held, errorValue := service.readFlowCentralOutboxHeldBack(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(held) != 1 || held[0].TaskID != task.ID {
		t.Fatalf("work nobody can carry is still work, so it stays: %+v", held)
	}
	if held[0].LastError != "no messenger account here" {
		t.Fatalf("lastError = %q", held[0].LastError)
	}
}

func TestQueueingATaskAgainGivesItItsAttemptsBack(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "released-task"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	failFlowCentralOutboxEntry(t, service, task.ID, flowCentralOutboxAttemptLimit, errors.New("no messenger account here"))

	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	entries := flowCentralOutboxEntries(t, service)
	if len(entries) != 1 || entries[0].AttemptCount != 0 {
		t.Fatalf("once the reason is gone the task has to drain again, got %+v", entries)
	}
}

func TestTheHeldBackLineIsWrittenOnceRatherThanEveryMinute(t *testing.T) {
	failure := errors.New("no messenger account here")

	stillTrying := flowCentralDrainFailureLine(flowCentralOutboxEntry{TaskID: "task", AttemptCount: flowCentralOutboxAttemptLimit - 2}, failure)
	lastOne := flowCentralDrainFailureLine(flowCentralOutboxEntry{TaskID: "task", AttemptCount: flowCentralOutboxAttemptLimit - 1}, failure)

	if strings.Contains(stillTrying, "held back") {
		t.Fatalf("an entry with attempts left is not held back: %q", stillTrying)
	}
	if !strings.Contains(lastOne, "held back") || !strings.Contains(lastOne, "flow-central-held") {
		t.Fatalf("the line that ends the retries has to say where to look: %q", lastOne)
	}
}

func TestTheHeldBackReportNamesEachTaskAndItsReason(t *testing.T) {
	report := flowCentralHeldBackReport([]flowCentralOutboxEntry{{
		TaskID:          "790b5d1ee362",
		Intent:          flowCentralWriteIntent,
		AttemptCount:    flowCentralOutboxAttemptLimit,
		LastError:       "task 790b5d1ee362 belongs to 3d8407a63e8b, who has no messenger account here",
		LastAttemptedAt: "2026-08-14T09:00:00Z",
	}})

	for _, expected := range []string{"790b5d1ee362", "3d8407a63e8b", "2026-08-14T09:00:00Z", "flow-central-backfill"} {
		if !strings.Contains(report, expected) {
			t.Fatalf("a report a person reads has to carry %q:\n%s", expected, report)
		}
	}
}

func TestAnEmptyHeldBackReportSaysSo(t *testing.T) {
	if report := flowCentralHeldBackReport(nil); !strings.Contains(report, "nothing") {
		t.Fatalf("report = %q", report)
	}
}
