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

func TestDeleteFlowTaskRollsBackWhenCachePurgeFails(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx := context.Background()
	task := flowSummaryInvalidationTask("task-delete-rollback", "26W28", "2026-07-06", "2026-07-07", flowStatusInProgress, 1024)
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

	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(ctx, `
		CREATE TRIGGER reject_flow_summary_cache_purge
		BEFORE DELETE ON flow_summary_cache_entries
		BEGIN
			SELECT RAISE(ABORT, 'forced cache purge failure');
		END`); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.deleteFlowTaskByID(ctx, task.ID); errorValue == nil {
		t.Fatal("delete flow task succeeded despite cache purge failure")
	}

	storedTask, found, errorValue := service.readFlowTaskByID(ctx, task.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found || storedTask.ID != task.ID {
		t.Fatalf("task after rollback: found=%v id=%q", found, storedTask.ID)
	}
	var revision int64
	if errorValue := database.QueryRowContext(ctx, `
		SELECT revision
		FROM flow_summary_source_revisions
		WHERE source_kind = ? AND source_key = ?`, flowSummarySourceWeek, task.WeekCode).Scan(&revision); errorValue != nil {
		t.Fatal(errorValue)
	}
	if revision != snapshot.RequestedWeekRevision {
		t.Fatalf("week revision after rollback = %d, want %d", revision, snapshot.RequestedWeekRevision)
	}
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM flow_summary_cache_entries WHERE week_code = ?", task.WeekCode).Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 1 {
		t.Fatalf("cache entries after rollback = %d, want 1", cacheEntryCount)
	}
}
