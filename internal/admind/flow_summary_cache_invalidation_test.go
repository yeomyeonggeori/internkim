package admind

import (
	"context"
	"testing"
	"time"
)

func TestFlowSummaryCacheInvalidatesOldAndNewTaskScopes(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	task := flowSummaryInvalidationTask("task-update", "26W28", "2026-06-30", "2026-07-02", flowStatusInProgress, 1024)
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedTask := task
	updatedTask.WeekCode = "26W29"
	updatedTask.StartDate = "2026-07-13"
	updatedTask.EndDate = "2026-08-02"
	if errorValue := service.writeFlowTask(ctx, updatedTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W28"}, 2)
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W29"}, 1)
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: "2026-06"}, 2)
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: "2026-07"}, 2)
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: "2026-08"}, 1)
}

func TestFlowSummaryCacheInvalidatesDeletedTaskScopes(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	task := flowSummaryInvalidationTask("task-delete", "26W28", "2026-06-30", "2026-07-02", flowStatusInProgress, 1024)
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.deleteFlowTaskByID(ctx, task.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range []flowSummarySourceKey{
		{Kind: flowSummarySourceWeek, Key: "26W28"},
		{Kind: flowSummarySourceMonth, Key: "2026-06"},
		{Kind: flowSummarySourceMonth, Key: "2026-07"},
	} {
		assertFlowSummarySourceRevision(t, service, key, 2)
	}
}

func TestFlowSummaryCacheRepairsLegacyMalformedTask(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	insertLegacyMalformedFlowTask(t, service, "legacy-update")
	task := flowSummaryInvalidationTask("legacy-update", "26W28", "2026-07-06", "2026-07-07", flowStatusInProgress, 1024)

	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}

	repairedTask := readFlowTaskByIDForTest(t, service, task.ID)
	if repairedTask.WeekCode != task.WeekCode || repairedTask.StartDate != task.StartDate || repairedTask.EndDate != task.EndDate {
		t.Fatalf("repaired task = %+v", repairedTask)
	}
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W28"}, 1)
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: "2026-07"}, 1)
	assertFlowSummarySourceRevisionRowCount(t, service, 2)
}

func TestFlowSummaryCacheDeletesLegacyMalformedTask(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	insertLegacyMalformedFlowTask(t, service, "legacy-delete")

	if errorValue := service.deleteFlowTaskByID(ctx, "legacy-delete"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, found, errorValue := service.readFlowTaskByID(ctx, "legacy-delete"); errorValue != nil || found {
		t.Fatalf("found = %v error = %v", found, errorValue)
	}
	assertFlowSummarySourceRevisionRowCount(t, service, 0)
}

func TestFlowSummaryCacheInvalidatesStatusEndTaskScopes(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	task := flowSummaryInvalidationTask("task-status-end", "26W28", "2026-07-06", "2026-07-07", flowStatusInProgress, 1024)
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	updatedTask := task
	updatedTask.WeekCode = "26W32"
	updatedTask.StartDate = "2026-08-03"
	updatedTask.EndDate = "2026-08-04"
	updatedTask.Status = flowStatusCompleted
	if _, errorValue := service.writeFlowTaskAtStatusEnd(ctx, updatedTask); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W28"}, 2)
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W32"}, 1)
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: "2026-07"}, 2)
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: "2026-08"}, 1)
}

func TestFlowSummaryCacheInvalidatesBoardMoveStatusAndRankChanges(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	tasks := []flowTask{
		flowSummaryInvalidationTask("moved", "26W30", "2026-07-20", "2026-07-21", flowStatusPlanned, 1024),
		flowSummaryInvalidationTask("target-one", "26W31", "2026-08-01", "2026-08-02", flowStatusInProgress, 1),
		flowSummaryInvalidationTask("target-two", "26W36", "2026-09-01", "2026-09-02", flowStatusInProgress, 2),
	}
	for _, task := range tasks {
		if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	beforeTaskID := "target-one"
	if _, errorValue := service.writeFlowTaskBoardMove(ctx, flowTaskBoardMoveRequest{TaskID: "moved", TargetStatus: flowStatusInProgress, BeforeTaskID: &beforeTaskID}, func(flowTask) bool { return true }); errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, key := range []flowSummarySourceKey{
		{Kind: flowSummarySourceWeek, Key: "26W30"},
		{Kind: flowSummarySourceWeek, Key: "26W31"},
		{Kind: flowSummarySourceWeek, Key: "26W36"},
		{Kind: flowSummarySourceMonth, Key: "2026-07"},
		{Kind: flowSummarySourceMonth, Key: "2026-08"},
		{Kind: flowSummarySourceMonth, Key: "2026-09"},
	} {
		assertFlowSummarySourceRevision(t, service, key, 2)
	}
}

func TestFlowSummaryCacheInvalidatesMattermostPostWeek(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	task := flowSummaryInvalidationTask("task-post", "26W28", "2026-07-06", "2026-07-07", flowStatusCompleted, 1024)
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.updateFlowTaskMattermostPostID(ctx, task.ID, "post-1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W28"}, 2)
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: "2026-07"}, 1)
	fixedPostCreatedAt := "2026-07-01T01:02:03Z"
	fixedUpdatedAt := "2026-07-02T01:02:03Z"
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, "UPDATE flow_tasks SET status_rank = ?, mattermost_post_created_at = ?, updated_at = ? WHERE id = ?", 4096, fixedPostCreatedAt, fixedUpdatedAt, task.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.updateFlowTaskMattermostPostID(ctx, task.ID, "post-1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W28"}, 2)
	assertFlowTaskWriteMetadata(t, service, task.ID, 4096, fixedPostCreatedAt, fixedUpdatedAt)
	if errorValue := service.updateFlowTaskMattermostPostID(ctx, task.ID, ""); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W28"}, 3)
}

func TestFlowSummaryCacheInvalidatesDefinitions(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	definitions := flowDefinitions{Types: []string{"회의"}, Sizes: defaultFlowSizeDefinitions()}
	if errorValue := service.writeFlowDefinitions(context.Background(), definitions); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceDefinitions, Key: "global"}, 1)
}

func TestFlowSummaryCacheInvalidatesWeeklyAndMonthlyComparisons(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	keys := flowSummaryCacheTestKeys()
	snapshot := readFlowSummaryDependencySnapshotForTest(t, service, keys, "members-v1")
	if stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(ctx, "26W28", keys, snapshot, validFlowSummaryCachePayloadForTest(), time.Now()); errorValue != nil || !stored {
		t.Fatalf("stored = %v error = %v", stored, errorValue)
	}
	task := flowSummaryInvalidationTask("comparison-task", "26W27", "2026-06-29", "2026-07-01", flowStatusCompleted, 1024)
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, updatedSnapshot, found, errorValue := service.readFlowSummaryCacheSnapshot(ctx, "26W28", keys, "members-v1"); errorValue != nil || found {
		t.Fatalf("found = %v snapshot = %+v error = %v", found, updatedSnapshot, errorValue)
	}
}

func flowSummaryInvalidationTask(id string, weekCode string, startDate string, endDate string, status string, statusRank int) flowTask {
	return flowTask{
		ID:               id,
		WeekCode:         weekCode,
		OwnerID:          "owner",
		OwnerName:        "Owner",
		ParticipantIDs:   []string{"owner"},
		ParticipantNames: []string{"Owner"},
		Business:         "개발",
		Type:             "회의",
		Content:          id,
		Goal:             "cache invalidation",
		Size:             "XS",
		Status:           status,
		StatusRank:       statusRank,
		StartDate:        startDate,
		EndDate:          endDate,
	}
}

func assertFlowSummarySourceRevision(t *testing.T, service *Service, key flowSummarySourceKey, expected int64) {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
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

func insertLegacyMalformedFlowTask(t *testing.T, service *Service, taskID string) {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(context.Background(), `
	INSERT INTO flow_tasks (
		id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, status_rank, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		taskID,
		"not-a-week",
		"owner",
		"Owner",
		`["owner"]`,
		`["Owner"]`,
		"개발",
		"회의",
		taskID,
		"legacy malformed row",
		"XS",
		flowStatusInProgress,
		1024,
		"July",
		"2026-13-01",
		0,
		"",
		"",
		"",
		"2026-07-01T00:00:00Z",
		"2026-07-01T00:00:00Z",
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func assertFlowSummarySourceRevisionRowCount(t *testing.T, service *Service, expected int) {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
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

func assertFlowTaskWriteMetadata(t *testing.T, service *Service, taskID string, expectedStatusRank int, expectedPostCreatedAt string, expectedUpdatedAt string) {
	t.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
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
