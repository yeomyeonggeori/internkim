package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkAttendanceSummarySourcesRaw(b *testing.B) {
	service := newAttendanceSummaryBenchmarkService(b)
	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		events, errorValue := service.readAttendanceEvents(ctx, "2026-07", "")
		if errorValue != nil {
			b.Fatal(errorValue)
		}
		absences, errorValue := service.readAttendanceAbsences(ctx, "2026-07", "")
		if errorValue != nil {
			b.Fatal(errorValue)
		}
		if len(events) != 3100 || len(absences) != 50 {
			b.Fatalf("events = %d, absences = %d", len(events), len(absences))
		}
	}
}

func BenchmarkAttendanceSummarySourcesCacheHit(b *testing.B) {
	service := newAttendanceSummaryBenchmarkService(b)
	ctx := context.Background()
	if _, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", ""); errorValue != nil {
		b.Fatal(errorValue)
	}
	if _, errorValue := service.readCachedAttendanceAbsences(ctx, "2026-07", ""); errorValue != nil {
		b.Fatal(errorValue)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		events, errorValue := service.readCachedAttendanceEvents(ctx, "2026-07", "")
		if errorValue != nil {
			b.Fatal(errorValue)
		}
		absences, errorValue := service.readCachedAttendanceAbsences(ctx, "2026-07", "")
		if errorValue != nil {
			b.Fatal(errorValue)
		}
		if len(events) != 3100 || len(absences) != 50 {
			b.Fatalf("events = %d, absences = %d", len(events), len(absences))
		}
	}
}

func BenchmarkAttendanceSummaryRequestCacheMiss(b *testing.B) {
	service := newAttendanceSummaryBenchmarkService(b)
	if errorValue := service.writeAttendanceTeamViewVisibleToAll(context.Background(), false); errorValue != nil {
		b.Fatal(errorValue)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		b.StopTimer()
		clearAttendanceSummaryBenchmarkCache(b, service)
		b.StartTimer()
		writeAttendanceSummaryBenchmarkResponse(
			b,
			service,
			service.readCachedAttendanceEvents,
			service.readCachedAttendanceAbsences,
		)
	}
}

func BenchmarkAttendanceSummaryRequestCacheHit(b *testing.B) {
	service := newAttendanceSummaryBenchmarkService(b)
	if errorValue := service.writeAttendanceTeamViewVisibleToAll(context.Background(), false); errorValue != nil {
		b.Fatal(errorValue)
	}
	writeAttendanceSummaryBenchmarkResponse(
		b,
		service,
		service.readCachedAttendanceEvents,
		service.readCachedAttendanceAbsences,
	)
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		writeAttendanceSummaryBenchmarkResponse(
			b,
			service,
			service.readCachedAttendanceEvents,
			service.readCachedAttendanceAbsences,
		)
	}
}

func BenchmarkAttendanceSummaryRequestUncached(b *testing.B) {
	service := newAttendanceSummaryBenchmarkService(b)
	if errorValue := service.writeAttendanceTeamViewVisibleToAll(context.Background(), false); errorValue != nil {
		b.Fatal(errorValue)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		writeAttendanceSummaryBenchmarkResponse(
			b,
			service,
			service.readAttendanceEvents,
			service.readAttendanceAbsences,
		)
	}
}

func BenchmarkAttendanceSummaryAuthenticatedRequestCacheMiss(b *testing.B) {
	service := newAuthenticatedAttendanceSummaryBenchmarkService(b)
	verifyAuthenticatedAttendanceSummaryBenchmarkPreflight(
		b,
		service,
		service.readCachedAttendanceEvents,
		service.readCachedAttendanceAbsences,
	)
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		b.StopTimer()
		clearAttendanceSummaryBenchmarkCache(b, service)
		b.StartTimer()
		writeAuthenticatedAttendanceSummaryBenchmarkResponse(
			b,
			service,
			service.readCachedAttendanceEvents,
			service.readCachedAttendanceAbsences,
		)
	}
}

func BenchmarkAttendanceSummaryAuthenticatedRequestCacheHit(b *testing.B) {
	service := newAuthenticatedAttendanceSummaryBenchmarkService(b)
	verifyAuthenticatedAttendanceSummaryBenchmarkPreflight(
		b,
		service,
		service.readCachedAttendanceEvents,
		service.readCachedAttendanceAbsences,
	)
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		writeAuthenticatedAttendanceSummaryBenchmarkResponse(
			b,
			service,
			service.readCachedAttendanceEvents,
			service.readCachedAttendanceAbsences,
		)
	}
}

func BenchmarkAttendanceSummaryAuthenticatedRequestUncached(b *testing.B) {
	service := newAuthenticatedAttendanceSummaryBenchmarkService(b)
	verifyAuthenticatedAttendanceSummaryBenchmarkPreflight(
		b,
		service,
		service.readAttendanceEvents,
		service.readAttendanceAbsences,
	)
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		writeAuthenticatedAttendanceSummaryBenchmarkResponse(
			b,
			service,
			service.readAttendanceEvents,
			service.readAttendanceAbsences,
		)
	}
}

func TestAuthenticatedAttendanceSummaryBenchmarkPreflight(t *testing.T) {
	service := newAuthenticatedAttendanceSummaryBenchmarkService(t)
	preflight, errorValue := runAuthenticatedAttendanceSummaryBenchmarkPreflight(
		service,
		service.readCachedAttendanceEvents,
		service.readCachedAttendanceAbsences,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertAuthenticatedAttendanceSummaryBenchmarkPreflight(t, preflight)
}

type authenticatedAttendanceSummaryBenchmarkPreflight struct {
	StatusCode     int
	Response       attendanceSummaryResponse
	EventTargets   []string
	AbsenceTargets []string
}

func runAuthenticatedAttendanceSummaryBenchmarkPreflight(
	service *Service,
	eventsReader attendanceEventsReader,
	absencesReader attendanceAbsencesReader,
) (authenticatedAttendanceSummaryBenchmarkPreflight, error) {
	eventTargets := []string{}
	absenceTargets := []string{}
	observedEventsReader := func(ctx context.Context, month string, email string) ([]attendanceEvent, error) {
		eventTargets = append(eventTargets, email)
		return eventsReader(ctx, month, email)
	}
	observedAbsencesReader := func(ctx context.Context, month string, email string) ([]attendanceAbsence, error) {
		absenceTargets = append(absenceTargets, email)
		return absencesReader(ctx, month, email)
	}
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month=2026-07", nil)
	request.Header.Set("X-Forwarded-Email", "member-00@example.com")
	recorder := httptest.NewRecorder()
	service.writeAttendanceSummaryWithReaders(recorder, request, observedEventsReader, observedAbsencesReader)
	response := attendanceSummaryResponse{}
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &response); errorValue != nil {
		return authenticatedAttendanceSummaryBenchmarkPreflight{}, errorValue
	}
	return authenticatedAttendanceSummaryBenchmarkPreflight{
		StatusCode:     recorder.Code,
		Response:       response,
		EventTargets:   eventTargets,
		AbsenceTargets: absenceTargets,
	}, nil
}

func verifyAuthenticatedAttendanceSummaryBenchmarkPreflight(
	testContext testing.TB,
	service *Service,
	eventsReader attendanceEventsReader,
	absencesReader attendanceAbsencesReader,
) {
	testContext.Helper()
	preflight, errorValue := runAuthenticatedAttendanceSummaryBenchmarkPreflight(service, eventsReader, absencesReader)
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	assertAuthenticatedAttendanceSummaryBenchmarkPreflight(testContext, preflight)
}

func assertAuthenticatedAttendanceSummaryBenchmarkPreflight(
	testContext testing.TB,
	preflight authenticatedAttendanceSummaryBenchmarkPreflight,
) {
	testContext.Helper()
	if preflight.StatusCode != http.StatusOK {
		testContext.Fatalf("status = %d, want %d", preflight.StatusCode, http.StatusOK)
	}
	if preflight.Response.CurrentUserEmail != "member-00@example.com" {
		testContext.Fatalf("current user email = %q", preflight.Response.CurrentUserEmail)
	}
	if len(preflight.EventTargets) != 2 || preflight.EventTargets[0] != "" || preflight.EventTargets[1] != "member-00@example.com" {
		testContext.Fatalf("event targets = %v", preflight.EventTargets)
	}
	if len(preflight.AbsenceTargets) != 1 || preflight.AbsenceTargets[0] != "" {
		testContext.Fatalf("absence targets = %v", preflight.AbsenceTargets)
	}
}

func writeAttendanceSummaryBenchmarkResponse(
	b *testing.B,
	service *Service,
	eventsReader func(context.Context, string, string) ([]attendanceEvent, error),
	absencesReader func(context.Context, string, string) ([]attendanceAbsence, error),
) {
	writeAttendanceSummaryBenchmarkResponseForActor(b, service, eventsReader, absencesReader, "")
}

func writeAuthenticatedAttendanceSummaryBenchmarkResponse(
	b *testing.B,
	service *Service,
	eventsReader func(context.Context, string, string) ([]attendanceEvent, error),
	absencesReader func(context.Context, string, string) ([]attendanceAbsence, error),
) {
	writeAttendanceSummaryBenchmarkResponseForActor(
		b,
		service,
		eventsReader,
		absencesReader,
		"member-00@example.com",
	)
}

func writeAttendanceSummaryBenchmarkResponseForActor(
	b *testing.B,
	service *Service,
	eventsReader func(context.Context, string, string) ([]attendanceEvent, error),
	absencesReader func(context.Context, string, string) ([]attendanceAbsence, error),
	actorEmail string,
) {
	b.Helper()
	request := httptest.NewRequest(http.MethodGet, "/attendance/api/summary?month=2026-07", nil)
	if actorEmail != "" {
		request.Header.Set("X-Forwarded-Email", actorEmail)
	}
	recorder := httptest.NewRecorder()
	service.writeAttendanceSummaryWithReaders(recorder, request, eventsReader, absencesReader)
	if recorder.Code != http.StatusOK {
		b.Fatalf("attendance summary status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}
