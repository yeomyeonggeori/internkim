package admind

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func newTaskNotifyTestService(t *testing.T, stateDirectory string) *Service {
	t.Helper()
	return NewService(Configuration{DatabasePath: filepath.Join(stateDirectory, "internkim.sqlite")})
}

func TestTheFirstCycleAdoptsWhatIsAlreadyThereWithoutNotifying(t *testing.T) {
	service := newTaskNotifyTestService(t, t.TempDir())
	ctx := context.Background()
	now := time.Now()

	seeded, errorValue := service.taskNotifyBaselineSeeded(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if seeded {
		t.Fatal("a device that has never run this has no baseline")
	}

	service.adoptTaskRunsWithoutNotifying(ctx, []taskNotifyRun{
		{TaskRunID: "old-completed", Status: "completed"},
		{TaskRunID: "old-waiting", Status: "waiting_approval"},
	}, now)

	seeded, errorValue = service.taskNotifyBaselineSeeded(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !seeded {
		t.Fatal("the baseline has to be recorded, or every restart adopts again and swallows a transition")
	}
	marks, errorValue := service.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if marks["old-completed"] != "completed" || marks["old-waiting"] != "waiting_approval" {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestMarksSurviveARestart(t *testing.T) {
	stateDirectory := t.TempDir()
	ctx := context.Background()
	now := time.Now()

	first := newTaskNotifyTestService(t, stateDirectory)
	first.adoptTaskRunsWithoutNotifying(ctx, []taskNotifyRun{{TaskRunID: "run-1", Status: "running"}}, now)

	// The same state directory, as a restarted process would find it.
	second := newTaskNotifyTestService(t, stateDirectory)
	seeded, errorValue := second.taskNotifyBaselineSeeded(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !seeded {
		t.Fatal("a restart must not re-adopt, or a transition that happened while it was down is lost")
	}
	marks, errorValue := second.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if marks["run-1"] != "running" {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestAMarkMovesOnlyWhenTheStatusDoes(t *testing.T) {
	service := newTaskNotifyTestService(t, t.TempDir())
	ctx := context.Background()
	now := time.Now()

	if errorValue := service.writeTaskNotifyMarks(ctx, map[string]string{"run-1": "running"}, now); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeTaskNotifyMarks(ctx, map[string]string{"run-1": "completed"}, now); errorValue != nil {
		t.Fatal(errorValue)
	}
	marks, errorValue := service.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(marks) != 1 || marks["run-1"] != "completed" {
		t.Fatalf("marks = %+v", marks)
	}
}

func TestMarksForRunsTheLedgerHasPrunedAreForgotten(t *testing.T) {
	service := newTaskNotifyTestService(t, t.TempDir())
	ctx := context.Background()
	now := time.Now()

	if errorValue := service.writeTaskNotifyMarks(ctx, map[string]string{"ancient": "completed"}, now.Add(-90*24*time.Hour)); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeTaskNotifyMarks(ctx, map[string]string{"recent": "completed"}, now); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.forgetStaleTaskNotifyMarks(ctx, now.Add(-taskNotifyMarkLife)); errorValue != nil {
		t.Fatal(errorValue)
	}

	marks, errorValue := service.readTaskNotifyMarks(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, held := marks["ancient"]; held {
		t.Fatalf("marks = %+v", marks)
	}
	if marks["recent"] != "completed" {
		t.Fatalf("marks = %+v", marks)
	}
}
