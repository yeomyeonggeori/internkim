package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCalendarEventWindowCacheHTTPOnlyCachesExplicitRange(t *testing.T) {
	service := newCalendarEventWindowBenchmarkService(t)
	handler := service.router()
	serveCalendarEventWindowBenchmark(t, handler)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var cacheEntryCount int
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries").Scan(&cacheEntryCount); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 1 {
		database.Close()
		t.Fatalf("explicit range cache entries = %d, want 1", cacheEntryCount)
	}
	if _, errorValue := database.ExecContext(context.Background(), "DELETE FROM calendar_event_window_cache_entries"); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	request := httptest.NewRequest(http.MethodGet, "/calendar/api/events?window=upcoming", nil)
	request.RemoteAddr = "198.51.100.10:443"
	request.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("upcoming status = %d body = %s", response.Code, response.Body.String())
	}
	database, errorValue = service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if errorValue := database.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM calendar_event_window_cache_entries").Scan(&cacheEntryCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if cacheEntryCount != 0 {
		t.Fatalf("upcoming cache entries = %d, want 0", cacheEntryCount)
	}
}

func TestCalendarEventWindowCacheKeepsDynamicImagesOutsidePayload(t *testing.T) {
	service := newCalendarEventWindowBenchmarkService(t)
	handler := service.router()
	firstResponse := decodeCalendarEventWindowBenchmarkResponse(t, serveCalendarEventWindowBenchmark(t, handler))
	resetCalendarActorProfileBenchmarkCache(service)
	secondResponse := decodeCalendarEventWindowBenchmarkResponse(t, serveCalendarEventWindowBenchmark(t, handler))
	verifyCalendarEventWindowBenchmarkResponse(t, firstResponse)
	verifyCalendarEventWindowBenchmarkResponse(t, secondResponse)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var payloadJSON []byte
	if errorValue := database.QueryRowContext(context.Background(), "SELECT payload_json FROM calendar_event_window_cache_entries").Scan(&payloadJSON); errorValue != nil {
		t.Fatal(errorValue)
	}
	var payload calendarEventWindowCachePayload
	if errorValue := json.Unmarshal(payloadJSON, &payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(payload.Events) == 0 || len(payload.Events[0].Participants) == 0 {
		t.Fatalf("cached payload events = %+v", payload.Events)
	}
	if payload.Events[0].CreatedByImage != "" || payload.Events[0].UpdatedByImage != "" || payload.Events[0].Participants[0].Image != "" {
		t.Fatalf("cached dynamic images = %q/%q/%q", payload.Events[0].CreatedByImage, payload.Events[0].UpdatedByImage, payload.Events[0].Participants[0].Image)
	}
}
