package admind

import (
	"context"
	"testing"
	"time"
)

const attendanceSummaryBenchmarkReadsPerMutation = 10

func BenchmarkAttendanceSummaryMutationWorkloadCached(b *testing.B) {
	service := newAttendanceSummaryBenchmarkService(b)
	ctx := context.Background()
	if _, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", ""); errorValue != nil {
		b.Fatal(errorValue)
	}
	if _, errorValue := service.readCachedAttendanceAbsences(ctx, "2026-07", ""); errorValue != nil {
		b.Fatal(errorValue)
	}
	benchmarkAttendanceSummaryMutationWorkload(
		b,
		service,
		service.readCachedAttendanceEvents,
		service.readCachedAttendanceAbsences,
	)
}

func BenchmarkAttendanceSummaryMutationWorkloadUncached(b *testing.B) {
	service := newAttendanceSummaryBenchmarkService(b)
	benchmarkAttendanceSummaryMutationWorkload(
		b,
		service,
		service.readAttendanceEvents,
		service.readAttendanceAbsences,
	)
}

func BenchmarkAttendanceSummaryHistoricalMonthReadIndexed(b *testing.B) {
	benchmarkAttendanceSummaryHistoricalMonthRead(b, false)
}

func BenchmarkAttendanceSummaryHistoricalMonthReadWithoutLocalDateIndex(b *testing.B) {
	benchmarkAttendanceSummaryHistoricalMonthRead(b, true)
}

func benchmarkAttendanceSummaryHistoricalMonthRead(b *testing.B, shouldDropIndex bool) {
	service := newAttendanceSummaryBenchmarkService(b)
	seedAttendanceSummaryHistoricalBenchmark(b, service)
	ctx := context.Background()
	if shouldDropIndex {
		database, errorValue := service.openAttendanceDatabase(ctx)
		if errorValue != nil {
			b.Fatal(errorValue)
		}
		if _, errorValue := database.Exec("DROP INDEX attendance_events_local_date"); errorValue != nil {
			database.Close()
			b.Fatal(errorValue)
		}
		if errorValue := database.Close(); errorValue != nil {
			b.Fatal(errorValue)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		events, errorValue := service.readAttendanceEvents(ctx, "2026-07", "")
		if errorValue != nil {
			b.Fatal(errorValue)
		}
		if len(events) != 3100 {
			b.Fatalf("events = %d", len(events))
		}
	}
}

func benchmarkAttendanceSummaryMutationWorkload(
	b *testing.B,
	service *Service,
	eventsReader attendanceEventsReader,
	absencesReader attendanceAbsencesReader,
) {
	b.Helper()
	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		b.Fatal(errorValue)
	}
	defer database.Close()
	event := attendanceEvent{ID: "event-00-01-0"}
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		localTime := time.Date(2026, 7, 1, 9, 0, benchmarkIndex%2, 0, time.UTC)
		if errorValue := service.updateAttendanceEventTime(ctx, database, event, localTime); errorValue != nil {
			b.Fatal(errorValue)
		}
		for readIndex := 0; readIndex < attendanceSummaryBenchmarkReadsPerMutation; readIndex++ {
			readAttendanceSummaryBenchmarkSources(b, ctx, eventsReader, absencesReader)
		}
	}
}

func readAttendanceSummaryBenchmarkSources(
	b *testing.B,
	ctx context.Context,
	eventsReader attendanceEventsReader,
	absencesReader attendanceAbsencesReader,
) {
	b.Helper()
	events, errorValue := eventsReader(ctx, "2026-07", "")
	if errorValue != nil {
		b.Fatal(errorValue)
	}
	absences, errorValue := absencesReader(ctx, "2026-07", "")
	if errorValue != nil {
		b.Fatal(errorValue)
	}
	if len(events) != 3100 || len(absences) != 50 {
		b.Fatalf("events = %d, absences = %d", len(events), len(absences))
	}
}
