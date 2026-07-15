package admind

import (
	"context"
	"testing"
	"time"
)

func TestFlowSummaryCacheDoesNotStoreBuildOverlappingTaskWrite(t *testing.T) {
	service := newFlowSummaryCacheTestService(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	members := []flowMember{{ID: "owner", Name: "Owner"}}
	task := flowSummaryInvalidationTask("concurrent-cache-task", "26W28", "2026-07-06", "2026-07-07", flowStatusInProgress, 1024)
	task.Content = "before write"
	if errorValue := service.writeFlowTask(ctx, task); errorValue != nil {
		t.Fatal(errorValue)
	}

	weekStart := time.Date(2026, 7, 6, 0, 0, 0, 0, time.Local)
	keys := flowSummaryDependencyKeysForWeek(task.WeekCode, weekStart)
	fingerprint, errorValue := flowSummaryMemberFingerprint(members)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	buildCompletedSourceRead := make(chan struct{})
	resumeBuild := make(chan struct{})
	readResult := make(chan error, 1)
	go func() {
		_, errorValue := service.readCachedFlowSummaryReadModel(ctx, task.WeekCode, keys, fingerprint, func(ctx context.Context) (flowSummaryReadModel, error) {
			readModel, buildError := service.buildFlowSummaryReadModel(ctx, task.WeekCode, weekStart, members)
			close(buildCompletedSourceRead)
			select {
			case <-resumeBuild:
				return readModel, buildError
			case <-ctx.Done():
				return flowSummaryReadModel{}, ctx.Err()
			}
		})
		readResult <- errorValue
	}()

	select {
	case <-buildCompletedSourceRead:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	updatedTask := task
	updatedTask.Content = "after write"
	if errorValue := service.writeFlowTask(ctx, updatedTask); errorValue != nil {
		close(resumeBuild)
		t.Fatal(errorValue)
	}
	close(resumeBuild)
	select {
	case errorValue := <-readResult:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM flow_summary_cache_entries WHERE week_code = ?", task.WeekCode).Scan(&cacheEntryCount); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 0 {
		t.Fatalf("stale cache entry count = %d, want 0", cacheEntryCount)
	}

	readModel, errorValue := service.readCachedFlowSummaryReadModel(ctx, task.WeekCode, keys, fingerprint, func(ctx context.Context) (flowSummaryReadModel, error) {
		return service.buildFlowSummaryReadModel(ctx, task.WeekCode, weekStart, members)
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(readModel.WeeklyTasks) != 1 || readModel.WeeklyTasks[0].Content != updatedTask.Content {
		t.Fatalf("weekly tasks = %+v, want updated task", readModel.WeeklyTasks)
	}
}
