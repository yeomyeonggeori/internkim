package admind

import (
	"context"
	"slices"
	"testing"
)

func insertTaskRowForTest(t *testing.T, service *Service, taskID string, ownerName string, status string) {
	t.Helper()
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(context.Background(), `
INSERT INTO flow_tasks (
	id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, start_date, end_date, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID, "26W23", "member-1", ownerName, `["member-1"]`, `["`+ownerName+`"]`,
		"샘플거리", "기능", taskID, "M", status, "2026-06-01", "", "2026-06-01T12:34:56Z", "2026-06-01T12:34:56Z")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func taskIDsOf(tasks []Task) []string {
	identifiers := make([]string, 0, len(tasks))
	for _, task := range tasks {
		identifiers = append(identifiers, task.ID)
	}
	return identifiers
}

func TestRequestedWorkFloatsWhicheverWayItWasSpelled(t *testing.T) {
	service := newTaskAuthorizationTestService(t)
	insertTaskRowForTest(t, service, "english-requested-task", "박예시", taskStatusRequested)
	insertTaskRowForTest(t, service, "planned-task", "이샘플", taskStatusPlanned)
	insertTaskRowForTest(t, service, "korean-requested-task", "최견본", "요청")

	weeklyTasks, errorValue := service.readTasks(context.Background(), "26W23", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	everyTask, errorValue := service.readAllTasks(context.Background(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	wanted := []string{"english-requested-task", "korean-requested-task", "planned-task"}
	if identifiers := taskIDsOf(weeklyTasks); !slices.Equal(identifiers, wanted) {
		t.Fatalf("the week reads %v, and requested work of either spelling belongs first: %v", identifiers, wanted)
	}
	if identifiers := taskIDsOf(everyTask); !slices.Equal(identifiers, wanted) {
		t.Fatalf("every task reads %v, and requested work of either spelling belongs first: %v", identifiers, wanted)
	}
}
