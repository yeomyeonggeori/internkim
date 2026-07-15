package admind

import "testing"

func TestCalendarEventWindowBenchmarkPreflight(t *testing.T) {
	service := newCalendarEventWindowBenchmarkService(t)
	response := requestCalendarEventWindowBenchmark(t, service.router())
	verifyCalendarEventWindowBenchmarkResponse(t, response)
}
