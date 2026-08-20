package admind

import (
	"context"
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func TestEveryDeviceStatusSurvivesTheRoundTrip(t *testing.T) {
	for _, deviceStatus := range []string{"요청", "예정", "진행", "완료", "일시정지", "기각", "중단"} {
		central := centralFlowStatus(deviceStatus)

		if back := deviceFlowStatus(central, deviceStatus); back != deviceStatus {
			t.Fatalf("%s went out as %s and came back as %s; a board that says nothing about a status must not move it",
				deviceStatus, central, back)
		}
	}
}

func TestAStatusTheBoardActuallyChangedIsTaken(t *testing.T) {
	if moved := deviceFlowStatus("done", "진행"); moved != "완료" {
		t.Fatalf("the board finished it, so the device has to agree: %q", moved)
	}
}

func TestATaskTheBoardMadeArrivesAsPlanned(t *testing.T) {
	if status := deviceFlowStatus("todo", ""); status != "예정" {
		t.Fatalf("status = %q", status)
	}
}

func TestTheLaterWriteWins(t *testing.T) {
	if !centralChangeIsNewer("2026-08-20T10:00:00Z", "2026-08-20T09:00:00Z") {
		t.Fatal("the board wrote after the device did")
	}
	if centralChangeIsNewer("2026-08-20T09:00:00Z", "2026-08-20T10:00:00Z") {
		t.Fatal("the device wrote after the board did, so its copy stands")
	}
}

func TestATaskTheDeviceHasNeverWrittenTakesWhateverTheBoardSays(t *testing.T) {
	if !centralChangeIsNewer("2026-08-20T10:00:00Z", "") {
		t.Fatal("there is nothing here to compare against, so the board's copy is the only one")
	}
}

func TestAGoalAndAReasonComeApartTheWayTheyWereJoined(t *testing.T) {
	note := centralFlowNote(flowTask{Goal: "출시", RequestReason: "고객 요청"})

	goal, reason := flowGoalAndReasonOf(note)

	if goal != "출시" || reason != "고객 요청" {
		t.Fatalf("goal = %q reason = %q", goal, reason)
	}
}

func TestANoteThatWasOnlyAReasonStaysOne(t *testing.T) {
	goal, reason := flowGoalAndReasonOf("고객 요청")

	if goal != "" || reason != "고객 요청" {
		t.Fatalf("goal = %q reason = %q", goal, reason)
	}
}

func TestABoardTaskBecomesADeviceTaskWithSomebodyOwningIt(t *testing.T) {
	people := map[string]adminUserMutation{
		stableFlowID("lee@example.test"): {Email: "lee@example.test", Name: "이샘플"},
	}

	task := deviceFlowTaskOf(centralplane.ChangedTask{
		CentralID:        "central-1",
		Title:            "보드에서 만든 업무",
		Status:           "in_progress",
		StartsAt:         "2026-08-20T00:00:00+09:00",
		ParticipantMails: []string{"lee@example.test"},
	}, flowTask{}, false, people)

	if task.OwnerID != stableFlowID("lee@example.test") || task.OwnerName != "이샘플" {
		t.Fatalf("the device shows work by who owns it, got %q/%q", task.OwnerID, task.OwnerName)
	}
	if task.Status != "진행" || task.StartDate != "2026-08-20" {
		t.Fatalf("status = %q start = %q", task.Status, task.StartDate)
	}
	if task.ID == "" {
		t.Fatal("a task the board made needs an identifier here")
	}
}

func TestMirroringDoesNotQueueTheTaskStraightBack(t *testing.T) {
	service := newFlowCentralTestService(t)
	task := flowNotificationTestTask("진행")
	task.ID = "mirrored-task"

	if errorValue := service.writeMirroredFlowTask(context.Background(), task); errorValue != nil {
		t.Fatal(errorValue)
	}

	if entries := flowCentralOutboxEntries(t, service); len(entries) != 0 {
		t.Fatalf("sending it back would arrive as another change, and so on: %+v", entries)
	}
}
