package admind

import (
	"context"
	"strings"
	"testing"
)

func TestCalendarEventWindowBenchmarkPreflight(t *testing.T) {
	service := newCalendarEventWindowBenchmarkService(t)
	responseRecorder := serveCalendarEventWindowBenchmark(t, service.router())
	response := decodeCalendarEventWindowBenchmarkResponse(t, responseRecorder)
	verifyCalendarEventWindowBenchmarkResponse(t, response)
}

func TestCalendarEventWindowBenchmarkSeparatesEventRowsAndParticipants(t *testing.T) {
	service := newCalendarEventWindowBenchmarkService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	startTime, endTime := calendarEventWindowBenchmarkRange()
	events, errorValue := readCalendarEventRows(context.Background(), database, startTime, endTime)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != calendarEventWindowBenchmarkEventCount {
		t.Fatalf("calendar event row count = %d", len(events))
	}
	if len(events[0].Participants) != 0 {
		t.Fatalf("calendar event row participants = %+v", events[0].Participants)
	}
	events, errorValue = loadCalendarEventListParticipants(context.Background(), database, events)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events[0].Participants) != 2 {
		t.Fatalf("calendar event participants = %+v", events[0].Participants)
	}
}

func TestCalendarEventWindowBenchmarkHistoryFixtureKeepsRequestedWindowStable(t *testing.T) {
	service := newCalendarEventWindowHistoryBenchmarkService(t)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var storedEventCount int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_events").Scan(&storedEventCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	wantStoredEventCount := calendarEventWindowBenchmarkEventCount + calendarEventWindowBenchmarkHistoryEventCount
	if storedEventCount != wantStoredEventCount {
		t.Fatalf("stored calendar event count = %d, want %d", storedEventCount, wantStoredEventCount)
	}
	startTime, endTime := calendarEventWindowBenchmarkRange()
	events, errorValue := service.readCalendarEvents(context.Background(), startTime, endTime)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(events) != calendarEventWindowBenchmarkEventCount {
		t.Fatalf("calendar event window count = %d", len(events))
	}
}

func TestCalendarEventWindowBenchmarkQueryPlanReportsCandidateIndexScan(t *testing.T) {
	service := newCalendarEventWindowHistoryBenchmarkService(t)
	baselinePlan := strings.Join(calendarEventWindowBenchmarkQueryPlan(t, service), "\n")
	t.Logf("baseline calendar event query plan:\n%s", baselinePlan)
	if !strings.Contains(baselinePlan, "SCAN calendar_events") {
		t.Fatalf("baseline calendar event query plan = %q", baselinePlan)
	}
	createCalendarEventWindowBenchmarkRangeIndex(t, service)
	indexedPlan := strings.Join(calendarEventWindowBenchmarkQueryPlan(t, service), "\n")
	t.Logf("indexed calendar event query plan:\n%s", indexedPlan)
	if !strings.Contains(indexedPlan, "SCAN calendar_events USING INDEX "+calendarEventWindowBenchmarkRangeIndexName) {
		t.Fatalf("indexed calendar event query plan = %q", indexedPlan)
	}
}

func BenchmarkCalendarEventWindowEventRows(benchmark *testing.B) {
	benchmark.Run("CurrentData", func(benchmark *testing.B) {
		benchmarkCalendarEventWindowEventRows(benchmark, newCalendarEventWindowBenchmarkService(benchmark))
	})
	benchmark.Run("AccumulatedHistory", func(benchmark *testing.B) {
		benchmarkCalendarEventWindowEventRows(benchmark, newCalendarEventWindowHistoryBenchmarkService(benchmark))
	})
	benchmark.Run("AccumulatedHistoryWithCandidateIndex", func(benchmark *testing.B) {
		service := newCalendarEventWindowHistoryBenchmarkService(benchmark)
		createCalendarEventWindowBenchmarkRangeIndex(benchmark, service)
		benchmarkCalendarEventWindowEventRows(benchmark, service)
	})
}

func BenchmarkCalendarEventWindowParticipantRead(benchmark *testing.B) {
	benchmark.Run("CurrentData", func(benchmark *testing.B) {
		benchmarkCalendarEventWindowParticipantRead(benchmark, newCalendarEventWindowBenchmarkService(benchmark))
	})
	benchmark.Run("AccumulatedHistory", func(benchmark *testing.B) {
		benchmarkCalendarEventWindowParticipantRead(benchmark, newCalendarEventWindowHistoryBenchmarkService(benchmark))
	})
}

func BenchmarkCalendarEventWindowCombinedDatabaseRead(benchmark *testing.B) {
	benchmark.Run("CurrentData", func(benchmark *testing.B) {
		benchmarkCalendarEventWindowCombinedDatabaseRead(benchmark, newCalendarEventWindowBenchmarkService(benchmark))
	})
	benchmark.Run("AccumulatedHistory", func(benchmark *testing.B) {
		benchmarkCalendarEventWindowCombinedDatabaseRead(benchmark, newCalendarEventWindowHistoryBenchmarkService(benchmark))
	})
	benchmark.Run("AccumulatedHistoryWithCandidateIndex", func(benchmark *testing.B) {
		service := newCalendarEventWindowHistoryBenchmarkService(benchmark)
		createCalendarEventWindowBenchmarkRangeIndex(benchmark, service)
		benchmarkCalendarEventWindowCombinedDatabaseRead(benchmark, service)
	})
}

func BenchmarkCalendarEventWindowParticipantProjection(benchmark *testing.B) {
	service := newCalendarEventWindowBenchmarkService(benchmark)
	startTime, endTime := calendarEventWindowBenchmarkRange()
	events, errorValue := service.readCalendarEvents(context.Background(), startTime, endTime)
	if errorValue != nil {
		benchmark.Fatal(errorValue)
	}
	request := calendarEventWindowBenchmarkRequest()
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		projectedEvents := service.calendarEventsWithParticipantImages(request, events)
		if len(projectedEvents) != calendarEventWindowBenchmarkEventCount {
			benchmark.Fatalf("calendar event count = %d", len(projectedEvents))
		}
	}
}

func BenchmarkCalendarEventWindowActorProfilesCold(benchmark *testing.B) {
	service, events := newCalendarEventWindowActorBenchmarkFixture(benchmark)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		benchmark.StopTimer()
		resetCalendarActorProfileBenchmarkCache(service)
		benchmark.StartTimer()
		projectedEvents := service.calendarEventsWithActorProfiles(context.Background(), events)
		if len(projectedEvents) != calendarEventWindowBenchmarkEventCount {
			benchmark.Fatalf("calendar event count = %d", len(projectedEvents))
		}
	}
}

func BenchmarkCalendarEventWindowActorProfilesWarm(benchmark *testing.B) {
	service, events := newCalendarEventWindowActorBenchmarkFixture(benchmark)
	service.calendarEventsWithActorProfiles(context.Background(), events)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		projectedEvents := service.calendarEventsWithActorProfiles(context.Background(), events)
		if len(projectedEvents) != calendarEventWindowBenchmarkEventCount {
			benchmark.Fatalf("calendar event count = %d", len(projectedEvents))
		}
	}
}

func BenchmarkCalendarEventWindowHTTPCold(benchmark *testing.B) {
	service := newCalendarEventWindowBenchmarkService(benchmark)
	handler := service.router()
	serveCalendarEventWindowBenchmark(benchmark, handler)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		benchmark.StopTimer()
		resetCalendarEventWindowBenchmarkCache(benchmark, service)
		benchmark.StartTimer()
		serveCalendarEventWindowBenchmark(benchmark, handler)
	}
}

func BenchmarkCalendarEventWindowHTTPRepeated(benchmark *testing.B) {
	service := newCalendarEventWindowBenchmarkService(benchmark)
	handler := service.router()
	serveCalendarEventWindowBenchmark(benchmark, handler)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		serveCalendarEventWindowBenchmark(benchmark, handler)
	}
}

func newCalendarEventWindowActorBenchmarkFixture(testContext testing.TB) (*Service, []calendarEvent) {
	testContext.Helper()
	service := newCalendarEventWindowBenchmarkService(testContext)
	startTime, endTime := calendarEventWindowBenchmarkRange()
	events, errorValue := service.readCalendarEvents(context.Background(), startTime, endTime)
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	events = service.calendarEventsWithParticipantImages(calendarEventWindowBenchmarkRequest(), events)
	return service, events
}

func benchmarkCalendarEventWindowEventRows(benchmark *testing.B, service *Service) {
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		benchmark.Fatal(errorValue)
	}
	defer database.Close()
	startTime, endTime := calendarEventWindowBenchmarkRange()
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		events, errorValue := readCalendarEventRows(context.Background(), database, startTime, endTime)
		if errorValue != nil {
			benchmark.Fatal(errorValue)
		}
		if len(events) != calendarEventWindowBenchmarkEventCount {
			benchmark.Fatalf("calendar event row count = %d", len(events))
		}
	}
}

func benchmarkCalendarEventWindowParticipantRead(benchmark *testing.B, service *Service) {
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		benchmark.Fatal(errorValue)
	}
	defer database.Close()
	startTime, endTime := calendarEventWindowBenchmarkRange()
	eventRows, errorValue := readCalendarEventRows(context.Background(), database, startTime, endTime)
	if errorValue != nil {
		benchmark.Fatal(errorValue)
	}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		benchmark.StopTimer()
		events := append([]calendarEvent(nil), eventRows...)
		benchmark.StartTimer()
		events, errorValue = loadCalendarEventListParticipants(context.Background(), database, events)
		if errorValue != nil {
			benchmark.Fatal(errorValue)
		}
		if len(events) != calendarEventWindowBenchmarkEventCount || len(events[0].Participants) != 2 {
			benchmark.Fatalf("calendar event participants = %+v", events[0].Participants)
		}
	}
}

func benchmarkCalendarEventWindowCombinedDatabaseRead(benchmark *testing.B, service *Service) {
	startTime, endTime := calendarEventWindowBenchmarkRange()
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		events, errorValue := service.readCalendarEvents(context.Background(), startTime, endTime)
		if errorValue != nil {
			benchmark.Fatal(errorValue)
		}
		if len(events) != calendarEventWindowBenchmarkEventCount {
			benchmark.Fatalf("calendar event count = %d", len(events))
		}
	}
}
