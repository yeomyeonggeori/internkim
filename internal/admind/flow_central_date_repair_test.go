package admind

import (
	"context"
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func TestTheRepairTakesTheDatesAndLeavesTheRest(t *testing.T) {
	service := newFlowCentralTestService(t)
	existing := flowNotificationTestTask("진행")
	existing.ID = "mis-dated"
	existing.StartDate = "2026-06-17"
	existing.EndDate = "2026-06-22"
	existing.Size = "M"
	existing.Content = "기기에서 고친 제목"
	if errorValue := service.writeMirroredFlowTask(context.Background(), existing); errorValue != nil {
		t.Fatal(errorValue)
	}

	moved, errorValue := service.repairOneFlowTaskDate(context.Background(), centralplane.ChangedTask{
		CentralID:    "central-1",
		DeviceTaskID: existing.ID,
		Title:        "보드가 들고 있는 옛 제목",
		Status:       "in_progress",
		Size:         "XL",
		StartsAt:     "2026-06-17T15:00:00+00:00",
		EndsAt:       "2026-06-22T14:59:00+00:00",
	}, map[string]adminUserMutation{})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !moved {
		t.Fatal("the start was a day early, so the repair had work to do")
	}

	repaired, found, errorValue := service.readFlowTaskByID(context.Background(), existing.ID)
	if errorValue != nil || !found {
		t.Fatalf("found=%v error=%v", found, errorValue)
	}
	if repaired.StartDate != "2026-06-18" || repaired.EndDate != "2026-06-22" {
		t.Fatalf("start=%q end=%q", repaired.StartDate, repaired.EndDate)
	}
	if repaired.Content != "기기에서 고친 제목" || repaired.Size != "M" {
		t.Fatalf("a date repair must not overwrite what the device edited: content=%q size=%q", repaired.Content, repaired.Size)
	}
}

func TestARowWithTheRightDatesIsLeftAlone(t *testing.T) {
	service := newFlowCentralTestService(t)
	existing := flowNotificationTestTask("진행")
	existing.ID = "already-right"
	existing.StartDate = "2026-06-18"
	existing.EndDate = "2026-06-22"
	existing.WeekCode = weekCodeForFlowDate("2026-06-18", flowDateNow())
	if errorValue := service.writeMirroredFlowTask(context.Background(), existing); errorValue != nil {
		t.Fatal(errorValue)
	}

	moved, errorValue := service.repairOneFlowTaskDate(context.Background(), centralplane.ChangedTask{
		CentralID:    "central-2",
		DeviceTaskID: existing.ID,
		Status:       "in_progress",
		StartsAt:     "2026-06-17T15:00:00+00:00",
		EndsAt:       "2026-06-22T14:59:00+00:00",
	}, map[string]adminUserMutation{})

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if moved {
		t.Fatal("nothing to repair here, so nothing should have been written")
	}
}
