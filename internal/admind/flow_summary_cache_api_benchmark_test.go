package admind

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkFlowSummaryHTTPCold(b *testing.B) {
	service := newFlowSummaryBenchmarkService(b)
	handler := service.router()
	b.ReportAllocs()
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		deleteFlowSummaryBenchmarkEntry(b, service, "26W28")
		b.StartTimer()
		requestFlowSummaryHTTPBenchmark(b, handler)
	}
}

func BenchmarkFlowSummaryHTTPCacheHit(b *testing.B) {
	service := newFlowSummaryBenchmarkService(b)
	handler := service.router()
	requestFlowSummaryHTTPBenchmark(b, handler)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		requestFlowSummaryHTTPBenchmark(b, handler)
	}
}

func requestFlowSummaryHTTPBenchmark(testContext testing.TB, handler http.Handler) {
	testContext.Helper()
	request := httptest.NewRequest(http.MethodGet, "/flow/api/summary?week=26W28", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("Cf-Access-Authenticated-User-Email", "staff@example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		testContext.Fatalf("summary status = %d body = %s", response.Code, response.Body.String())
	}
}
