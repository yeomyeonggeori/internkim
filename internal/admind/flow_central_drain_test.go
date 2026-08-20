package admind

import (
	"context"
	"testing"
)

func TestTheQueueIsLeftAloneWhenTheCentralPlaneIsNotConfigured(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "unconfigured-task"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	service.drainFlowTasksToCentralPlane(context.Background())

	if entries := flowCentralOutboxEntries(t, service); len(entries) != 1 {
		t.Fatalf("a device with nowhere to send has to keep what it recorded, got %+v", entries)
	}
}

func TestATaskRemembersWhatTheCentralPlaneCalledIt(t *testing.T) {
	service := newFlowCentralTestService(t)

	if errorValue := service.rememberFlowCentralIdentityForTask(context.Background(), "device-task", "central-uuid"); errorValue != nil {
		t.Fatal(errorValue)
	}

	centralID, errorValue := service.readFlowCentralIdentityByTaskID(context.Background(), "device-task")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if centralID != "central-uuid" {
		t.Fatalf("without this every drain would make a second copy, got %q", centralID)
	}
}

func TestATaskTheCentralPlaneNeverTookHasNoIdentity(t *testing.T) {
	service := newFlowCentralTestService(t)

	centralID, errorValue := service.readFlowCentralIdentityByTaskID(context.Background(), "never-sent")
	if errorValue != nil {
		t.Fatalf("not having been sent yet is an ordinary answer: %v", errorValue)
	}
	if centralID != "" {
		t.Fatalf("centralID = %q", centralID)
	}
}

func TestTheIdentityOutlivesTheTaskUntilTheRemovalIsCarried(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "doomed-task"
	if errorValue := service.writeFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.rememberFlowCentralIdentityForTask(context.Background(), task.ID, "central-uuid"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.deleteFlowTaskByID(context.Background(), task.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	centralID, errorValue := service.readFlowCentralIdentityByTaskID(context.Background(), task.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if centralID != "central-uuid" {
		t.Fatal("the removal has to name the task there, and the task here is already gone")
	}
}

func TestAParticipantTheOrgChartDoesNotCarryIsLeftOut(t *testing.T) {
	people := map[string]adminUserMutation{
		"known": {Email: "known@example.test"},
	}
	task := flowTask{ParticipantIDs: []string{"known", "stranger"}}

	addresses := participantAddresses(task, people)

	if len(addresses) != 1 || addresses[0] != "known@example.test" {
		t.Fatalf("addresses = %+v", addresses)
	}
}
