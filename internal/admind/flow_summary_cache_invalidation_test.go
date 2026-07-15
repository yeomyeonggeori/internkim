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
	if errorValue := service.updateFlowTaskMattermostPostID(ctx, task.ID, "post-1"); errorValue != nil {
		t.Fatal(errorValue)
	}
	assertFlowSummarySourceRevision(t, service, flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: "26W28"}, 2)
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
