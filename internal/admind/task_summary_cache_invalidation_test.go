package admind

import (
	"context"
	"testing"
	"time"
)

func TestTaskSummaryCacheInvalidatesOldAndNewTaskScopes(t *testing.T) {
	service := newTaskSummaryCacheTestService(t)
	ctx := context.Background()
	task := taskSummaryInvalidationTask("task-update", "26W28", "2026-06-30", "2026-07-02", taskStatusInProgress, 1024)
	if errorValue := service.writeTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedTask := task
	updatedTask.WeekCode = "26W29"
	updatedTask.StartDate = "2026-07-13"
	updatedTask.EndDate = "2026-08-02"
	if errorValue := service.writeTask(ctx, updatedTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceWeek, Key: "26W28"}, 2)
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceWeek, Key: "26W29"}, 1)
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceMonth, Key: "2026-06"}, 2)
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceMonth, Key: "2026-07"}, 2)
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceMonth, Key: "2026-08"}, 1)
}

func TestTaskSummaryCacheInvalidatesDeletedTaskScopes(t *testing.T) {
	service := newTaskSummaryCacheTestService(t)
	ctx := context.Background()
	task := taskSummaryInvalidationTask("task-delete", "26W28", "2026-06-30", "2026-07-02", taskStatusInProgress, 1024)
	if errorValue := service.writeTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.deleteTaskByID(ctx, task.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range []taskSummarySourceKey{
		{Kind: taskSummarySourceWeek, Key: "26W28"},
		{Kind: taskSummarySourceMonth, Key: "2026-06"},
		{Kind: taskSummarySourceMonth, Key: "2026-07"},
	} {
		assertTaskSummarySourceRevision(t, service, key, 2)
	}
}

func TestTaskSummaryCacheRepairsLegacyMalformedTask(t *testing.T) {
	service := newTaskSummaryCacheTestService(t)
	ctx := context.Background()
	insertLegacyMalformedTask(t, service, "legacy-update", "")
	task := taskSummaryInvalidationTask("legacy-update", "26W28", "2026-07-06", "2026-07-07", taskStatusInProgress, 1024)

	if errorValue := service.writeTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}

	repairedTask := readTaskByIDForTest(t, service, task.ID)
	if repairedTask.WeekCode != task.WeekCode || repairedTask.StartDate != task.StartDate || repairedTask.EndDate != task.EndDate {
		t.Fatalf("repaired task = %+v", repairedTask)
	}
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceWeek, Key: "26W28"}, 1)
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceMonth, Key: "2026-07"}, 1)
	assertTaskSummarySourceRevisionRowCount(t, service, 2)
}

func TestTaskSummaryCacheDeletesLegacyMalformedTask(t *testing.T) {
	service := newTaskSummaryCacheTestService(t)
	ctx := context.Background()
	insertLegacyMalformedTask(t, service, "legacy-delete", "")

	if errorValue := service.deleteTaskByID(ctx, "legacy-delete"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, found, errorValue := service.readTaskByID(ctx, "legacy-delete"); errorValue != nil || found {
		t.Fatalf("found = %v error = %v", found, errorValue)
	}
	assertTaskSummarySourceRevisionRowCount(t, service, 0)
}

func TestTaskSummaryCacheInvalidatesStatusEndTaskScopes(t *testing.T) {
	service := newTaskSummaryCacheTestService(t)
	ctx := context.Background()
	task := taskSummaryInvalidationTask("task-status-end", "26W28", "2026-07-06", "2026-07-07", taskStatusInProgress, 1024)
	if errorValue := service.writeTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedTask := task
	updatedTask.WeekCode = "26W32"
	updatedTask.StartDate = "2026-08-03"
	updatedTask.EndDate = "2026-08-04"
	updatedTask.Status = taskStatusCompleted
	if _, errorValue := service.writeTaskAtStatusEnd(ctx, updatedTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceWeek, Key: "26W28"}, 2)
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceWeek, Key: "26W32"}, 1)
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceMonth, Key: "2026-07"}, 2)
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceMonth, Key: "2026-08"}, 1)
}

func TestTaskSummaryCacheInvalidatesBoardMoveStatusAndRankChanges(t *testing.T) {
	service := newTaskSummaryCacheTestService(t)
	ctx := context.Background()
	tasks := []Task{
		taskSummaryInvalidationTask("moved", "26W30", "2026-07-20", "2026-07-21", taskStatusPlanned, 1024),
		taskSummaryInvalidationTask("target-one", "26W31", "2026-08-01", "2026-08-02", taskStatusInProgress, 1),
		taskSummaryInvalidationTask("target-two", "26W36", "2026-09-01", "2026-09-02", taskStatusInProgress, 2),
	}
	for _, task := range tasks {
		if errorValue := service.writeTask(ctx, task); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	beforeTaskID := "target-one"
	if _, errorValue := service.writeTaskBoardMove(ctx, taskBoardMoveRequest{TaskID: "moved", TargetStatus: taskStatusInProgress, BeforeTaskID: &beforeTaskID}, func(Task) bool { return true }); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range []taskSummarySourceKey{
		{Kind: taskSummarySourceWeek, Key: "26W30"},
		{Kind: taskSummarySourceWeek, Key: "26W31"},
		{Kind: taskSummarySourceWeek, Key: "26W36"},
		{Kind: taskSummarySourceMonth, Key: "2026-07"},
		{Kind: taskSummarySourceMonth, Key: "2026-08"},
		{Kind: taskSummarySourceMonth, Key: "2026-09"},
	} {
		assertTaskSummarySourceRevision(t, service, key, 2)
	}
}

func TestTaskSummaryCacheInvalidatesDefinitions(t *testing.T) {
	service := newTaskSummaryCacheTestService(t)
	definitions := taskDefinitions{Types: []string{"회의"}, Sizes: defaultTaskSizeDefinitions()}
	if errorValue := service.writeTaskDefinitions(context.Background(), definitions); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertTaskSummarySourceRevision(t, service, taskSummarySourceKey{Kind: taskSummarySourceDefinitions, Key: "global"}, 1)
}

func TestTaskSummaryCacheInvalidatesWeeklyAndMonthlyComparisons(t *testing.T) {
	service := newTaskSummaryCacheTestService(t)
	ctx := context.Background()
	keys := taskSummaryCacheTestKeys()
	snapshot := readTaskSummaryDependencySnapshotForTest(t, service, keys, "members-v1")
	if stored, errorValue := service.writeTaskSummaryCacheEntryIfCurrent(ctx, "26W28", keys, snapshot, validTaskSummaryCachePayloadForTest(), time.Now()); errorValue != nil || !stored {
		t.Fatalf("stored = %v error = %v", stored, errorValue)
	}
	task := taskSummaryInvalidationTask("comparison-task", "26W27", "2026-06-29", "2026-07-01", taskStatusCompleted, 1024)
	if errorValue := service.writeTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, updatedSnapshot, found, errorValue := service.readTaskSummaryCacheSnapshot(ctx, "26W28", keys, "members-v1"); errorValue != nil || found {
		t.Fatalf("found = %v snapshot = %+v error = %v", found, updatedSnapshot, errorValue)
	}
}

func taskSummaryInvalidationTask(id string, weekCode string, startDate string, endDate string, status string, statusRank int) Task {
	return Task{
		ID:               id,
		WeekCode:         weekCode,
		OwnerID:          "owner",
		OwnerName:        "Owner",
		ParticipantIDs:   []string{"owner"},
		ParticipantNames: []string{"Owner"},
		Business:         "개발",
		Type:             "회의",
		Content:          id,
		Size:             "XS",
		Status:           status,
		StatusRank:       statusRank,
		StartDate:        startDate,
		EndDate:          endDate,
	}
}

func assertTaskSummarySourceRevision(t *testing.T, service *Service, key taskSummarySourceKey, expected int64) {
	t.Helper()
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var revision int64
	if errorValue := database.QueryRowContext(context.Background(), `
	SELECT COALESCE((SELECT revision FROM flow_summary_source_revisions WHERE source_kind = ? AND source_key = ?), 0)`, key.Kind, key.Key).Scan(&revision); errorValue != nil {
		t.Fatal(errorValue)
	}
	if revision != expected {
		t.Fatalf("revision %s/%s = %d, want %d", key.Kind, key.Key, revision, expected)
	}
}

func insertLegacyMalformedTask(t *testing.T, service *Service, taskID string, mattermostPostID string) {
	t.Helper()
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(context.Background(), `
	INSERT INTO flow_tasks (
		id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, size, status, status_rank, start_date, end_date, mattermost_post_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID,
		"not-a-week",
		"owner",
		"Owner",
		`["owner"]`,
		`["Owner"]`,
		"개발",
		"회의",
		taskID,
		"XS",
		taskStatusInProgress,
		1024,
		"July",
		"2026-13-01",
		mattermostPostID,
		"2026-07-01T00:00:00Z",
		"2026-07-01T00:00:00Z",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func assertTaskSummarySourceRevisionRowCount(t *testing.T, service *Service, expected int) {
	t.Helper()
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM flow_summary_source_revisions").Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	if count != expected {
		t.Fatalf("source revision rows = %d, want %d", count, expected)
	}
}

func assertTaskWriteMetadata(t *testing.T, service *Service, taskID string, expectedStatusRank int, expectedPostCreatedAt string, expectedUpdatedAt string) {
	t.Helper()
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var statusRank int
	var postCreatedAt string
	var updatedAt string
	if errorValue := database.QueryRowContext(context.Background(), "SELECT status_rank, mattermost_post_created_at, updated_at FROM flow_tasks WHERE id = ?", taskID).Scan(&statusRank, &postCreatedAt, &updatedAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	if statusRank != expectedStatusRank || postCreatedAt != expectedPostCreatedAt || updatedAt != expectedUpdatedAt {
		t.Fatalf("status rank = %d post created at = %q updated at = %q, want %d, %q, %q", statusRank, postCreatedAt, updatedAt, expectedStatusRank, expectedPostCreatedAt, expectedUpdatedAt)
	}
}
