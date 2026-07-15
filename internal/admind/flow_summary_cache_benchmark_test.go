package admind

import (
	"context"
	"testing"
	"time"
)

func BenchmarkFlowSummaryUncached(b *testing.B) {
	service := newFlowSummaryBenchmarkService(b)
	members := flowSummaryBenchmarkMembers()
	weekStart := weekStartForCode("26W28", time.Date(2026, 7, 6, 0, 0, 0, 0, time.Local))
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, errorValue := service.buildFlowSummaryReadModel(context.Background(), "26W28", weekStart, members); errorValue != nil {
			b.Fatal(errorValue)
		}
	}
}

func BenchmarkFlowSummaryCold(b *testing.B) {
	service := newFlowSummaryBenchmarkService(b)
	members := flowSummaryBenchmarkMembers()
	weekStart := weekStartForCode("26W28", time.Date(2026, 7, 6, 0, 0, 0, 0, time.Local))
	keys := flowSummaryDependencyKeysForWeek("26W28", weekStart)
	fingerprint, errorValue := flowSummaryMemberFingerprint(members)
	if errorValue != nil {
		b.Fatal(errorValue)
	}
	build := func(ctx context.Context) (flowSummaryReadModel, error) {
		return service.buildFlowSummaryReadModel(ctx, "26W28", weekStart, members)
	}
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		deleteFlowSummaryBenchmarkEntry(b, service, "26W28")
		b.StartTimer()
		if _, errorValue := service.readCachedFlowSummaryReadModel(context.Background(), "26W28", keys, fingerprint, build); errorValue != nil {
			b.Fatal(errorValue)
		}
	}
}

func BenchmarkFlowSummaryCacheHit(b *testing.B) {
	service := newFlowSummaryBenchmarkService(b)
	members := flowSummaryBenchmarkMembers()
	weekStart := weekStartForCode("26W28", time.Date(2026, 7, 6, 0, 0, 0, 0, time.Local))
	keys := flowSummaryDependencyKeysForWeek("26W28", weekStart)
	fingerprint, errorValue := flowSummaryMemberFingerprint(members)
	if errorValue != nil {
		b.Fatal(errorValue)
	}
	build := func(ctx context.Context) (flowSummaryReadModel, error) {
		return service.buildFlowSummaryReadModel(ctx, "26W28", weekStart, members)
	}
	if _, errorValue := service.readCachedFlowSummaryReadModel(context.Background(), "26W28", keys, fingerprint, build); errorValue != nil {
		b.Fatal(errorValue)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		if _, errorValue := service.readCachedFlowSummaryReadModel(context.Background(), "26W28", keys, fingerprint, build); errorValue != nil {
			b.Fatal(errorValue)
		}
	}
}

func deleteFlowSummaryBenchmarkEntry(testContext testing.TB, service *Service, weekCode string) {
	testContext.Helper()
	database, errorValue := service.openFlowDatabase(context.Background())
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(context.Background(), "DELETE FROM flow_summary_cache_entries WHERE week_code = ?", weekCode); errorValue != nil {
		testContext.Fatal(errorValue)
	}
}
