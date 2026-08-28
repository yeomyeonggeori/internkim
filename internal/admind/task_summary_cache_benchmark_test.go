package admind

import (
	"context"
	"testing"
	"time"
)

func BenchmarkTaskSummaryUncached(b *testing.B) {
	service := newTaskSummaryBenchmarkService(b)
	members := taskSummaryBenchmarkMembers()
	weekStart := weekStartForCode("26W28", time.Date(2026, 7, 6, 0, 0, 0, 0, time.Local))
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, errorValue := service.buildTaskSummaryReadModel(context.Background(), "26W28", weekStart, members); errorValue != nil {
			b.Fatal(errorValue)
		}
	}
}

func BenchmarkTaskSummaryCold(b *testing.B) {
	service := newTaskSummaryBenchmarkService(b)
	members := taskSummaryBenchmarkMembers()
	weekStart := weekStartForCode("26W28", time.Date(2026, 7, 6, 0, 0, 0, 0, time.Local))
	keys := taskSummaryDependencyKeysForWeek("26W28", weekStart)
	fingerprint, errorValue := taskSummaryMemberFingerprint(members)
	if errorValue != nil {
		b.Fatal(errorValue)
	}
	build := func(ctx context.Context) (taskSummaryReadModel, error) {
		return service.buildTaskSummaryReadModel(ctx, "26W28", weekStart, members)
	}
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		deleteTaskSummaryBenchmarkEntry(b, service, "26W28")
		b.StartTimer()
		if _, errorValue := service.readCachedTaskSummaryReadModel(context.Background(), "26W28", keys, fingerprint, build); errorValue != nil {
			b.Fatal(errorValue)
		}
	}
}

func BenchmarkTaskSummaryCacheHit(b *testing.B) {
	service := newTaskSummaryBenchmarkService(b)
	members := taskSummaryBenchmarkMembers()
	weekStart := weekStartForCode("26W28", time.Date(2026, 7, 6, 0, 0, 0, 0, time.Local))
	keys := taskSummaryDependencyKeysForWeek("26W28", weekStart)
	fingerprint, errorValue := taskSummaryMemberFingerprint(members)
	if errorValue != nil {
		b.Fatal(errorValue)
	}
	build := func(ctx context.Context) (taskSummaryReadModel, error) {
		return service.buildTaskSummaryReadModel(ctx, "26W28", weekStart, members)
	}
	if _, errorValue := service.readCachedTaskSummaryReadModel(context.Background(), "26W28", keys, fingerprint, build); errorValue != nil {
		b.Fatal(errorValue)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, errorValue := service.readCachedTaskSummaryReadModel(context.Background(), "26W28", keys, fingerprint, build); errorValue != nil {
			b.Fatal(errorValue)
		}
	}
}

func deleteTaskSummaryBenchmarkEntry(testContext testing.TB, service *Service, weekCode string) {
	testContext.Helper()
	database, errorValue := service.openTaskDatabase(context.Background())
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(context.Background(), "DELETE FROM flow_summary_cache_entries WHERE week_code = ?", weekCode); errorValue != nil {
		testContext.Fatal(errorValue)
	}
}
