package admind

import (
	"context"
	"testing"
	"time"
)

func TestDeleteFlowTaskRemovesCachedTaskPayload(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	marker := "deleted-task-sensitive-marker"
	task := flowSummaryInvalidationTask("task-delete-cache", "26W28", "2026-07-06", "2026-07-07", flowStatusInProgress, 1024)
	task.Content = marker
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}

	weekStart := weekStartForCode(task.WeekCode, time.Date(2026, 7, 6, 0, 0, 0, 0, time.Local))
	keys := flowSummaryDependencyKeysForWeek(task.WeekCode, weekStart)
	_, snapshot, _, errorValue := service.readFlowSummaryCacheSnapshot(ctx, task.WeekCode, keys, "members")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	payload, errorValue := encodeFlowSummaryCachePayload(flowSummaryReadModel{WeeklyTasks: []flowTask{task}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	stored, errorValue := service.writeFlowSummaryCacheEntryIfCurrent(ctx, task.WeekCode, keys, snapshot, payload, time.Now())
	if errorValue != nil || !stored {
		t.Fatalf("store cache entry: stored=%v error=%v", stored, errorValue)
	}

	if errorValue := service.deleteFlowTaskByID(ctx, task.ID); errorValue != nil {
		t.Fatal(errorValue)
	}

	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var count int
	if errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM flow_summary_cache_entries WHERE instr(payload_json, ?) > 0", marker).Scan(&count); errorValue != nil {
		t.Fatal(errorValue)
	}
	if count != 0 {
		t.Fatalf("cache entries containing deleted task = %d, want 0", count)
	}
}
