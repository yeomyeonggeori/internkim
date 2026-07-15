package admind

import (
	"context"
	"testing"
)

func BenchmarkCalendarEventWindowCachedRead(benchmark *testing.B) {
	service := newCalendarEventWindowBenchmarkService(benchmark)
	startTime, endTime := calendarEventWindowBenchmarkRange()
	if _, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime); errorValue != nil {
		benchmark.Fatal(errorValue)
	}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		events, errorValue := service.readCalendarEventWindow(context.Background(), startTime, endTime)
		if errorValue != nil {
			benchmark.Fatal(errorValue)
		}
		if len(events) != calendarEventWindowBenchmarkEventCount {
			benchmark.Fatalf("calendar event count = %d", len(events))
		}
	}
}
