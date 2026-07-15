package admind

import (
	"context"
	"testing"
)

func TestCalendarEventWindowBenchmarkPreflight(t *testing.T) {
	service := newCalendarEventWindowBenchmarkService(t)
	response := requestCalendarEventWindowBenchmark(t, service.router())
	verifyCalendarEventWindowBenchmarkResponse(t, response)
}

func BenchmarkCalendarEventWindowDatabaseRead(benchmark *testing.B) {
	service := newCalendarEventWindowBenchmarkService(benchmark)
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
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		benchmark.StopTimer()
		service := newCalendarEventWindowBenchmarkService(benchmark)
		handler := service.router()
		benchmark.StartTimer()
		response := requestCalendarEventWindowBenchmark(benchmark, handler)
		verifyCalendarEventWindowBenchmarkResponse(benchmark, response)
	}
}

func BenchmarkCalendarEventWindowHTTPRepeated(benchmark *testing.B) {
	service := newCalendarEventWindowBenchmarkService(benchmark)
	handler := service.router()
	requestCalendarEventWindowBenchmark(benchmark, handler)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for iteration := 0; iteration < benchmark.N; iteration++ {
		response := requestCalendarEventWindowBenchmark(benchmark, handler)
		if len(response.Events) != calendarEventWindowBenchmarkEventCount {
			benchmark.Fatalf("calendar event count = %d", len(response.Events))
		}
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
