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
