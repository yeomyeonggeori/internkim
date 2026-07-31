package admind

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalendarHolidaysServeMemoryCacheWithoutReadingDeletedDatabaseSnapshot(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = calendarHolidayTestHTTPClient(t, "KR", &requestCount)

	first := requestCalendarHolidaysForYearLocaleTest(t, service, 2026, workspaceLanguageKorean)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatalf("open calendar database: %v", errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), "DELETE FROM calendar_holidays"); errorValue != nil {
		database.Close()
		t.Fatalf("delete calendar holidays: %v", errorValue)
	}
	database.Close()
	second := requestCalendarHolidaysForYearLocaleTest(t, service, 2026, workspaceLanguageKorean)

	if len(first.Holidays) != 1 || len(second.Holidays) != 1 || second.Holidays[0].Title != "새해 첫날" {
		t.Fatalf("holidays = %#v, %#v", first.Holidays, second.Holidays)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("API request count = %d, want one", requestCount.Load())
	}
}

func TestCalendarHolidaysLoadPersistedSnapshotAfterServiceRestart(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = calendarHolidayTestHTTPClient(t, "KR", &requestCount)
	first := requestCalendarHolidaysForYearLocaleTest(t, service, 2026, workspaceLanguageKorean)
	restartedService := NewService(service.Configuration)
	restartedService.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		return nil, io.EOF
	})}

	second := requestCalendarHolidaysForYearLocaleTest(t, restartedService, 2026, workspaceLanguageKorean)

	if len(first.Holidays) != 1 || len(second.Holidays) != 1 || second.Holidays[0].Title != "새해 첫날" {
		t.Fatalf("holidays = %#v, %#v", first.Holidays, second.Holidays)
	}
	if requestCount.Load() != 1 {
		t.Fatalf("API request count = %d, want one before restart", requestCount.Load())
	}
}

func TestCalendarHolidayConcurrentCacheMissFetchesYearOnce(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		time.Sleep(10 * time.Millisecond)
		return calendarHolidayTestResponse(request, "KR", "2026"), nil
	})}
	var waitGroup sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, errorValue := service.ensureCalendarHolidayYear(
				context.Background(),
				"KR",
				workspaceLanguageKorean,
				2026,
				time.Now().UTC(),
			)
			errors <- errorValue
		}()
	}
	waitGroup.Wait()
	close(errors)
	for errorValue := range errors {
		if errorValue != nil {
			t.Fatalf("ensure holiday year: %v", errorValue)
		}
	}
	if requestCount.Load() != 1 {
		t.Fatalf("API request count = %d, want one", requestCount.Load())
	}
}

func TestCalendarHolidaysRejectOversizedRangeWithoutAPIRequest(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = calendarHolidayTestHTTPClient(t, "KR", &requestCount)
	startTime := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	request := httptest.NewRequest(
		http.MethodGet,
		"/calendar/api/holidays?startISO="+startTime.Format(time.RFC3339)+
			"&endISO="+startTime.AddDate(3, 0, 0).Format(time.RFC3339),
		nil,
	)
	recorder := httptest.NewRecorder()

	service.serveCalendarHolidays(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if requestCount.Load() != 0 {
		t.Fatalf("API request count = %d, want none", requestCount.Load())
	}
}

func TestCalendarHolidayMonthlyRefreshRunsOncePerMonth(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = calendarHolidayTestHTTPClient(t, "KR", &requestCount)
	currentTime := time.Date(2026, time.July, 1, 0, 5, 0, 0, time.FixedZone("KST", 9*60*60))

	if errorValue := service.refreshCalendarHolidaysIfDue(context.Background(), currentTime); errorValue != nil {
		t.Fatalf("first monthly refresh: %v", errorValue)
	}
	if errorValue := service.refreshCalendarHolidaysIfDue(context.Background(), currentTime.AddDate(0, 0, 20)); errorValue != nil {
		t.Fatalf("same-month refresh: %v", errorValue)
	}
	if requestCount.Load() != calendarHolidayPreloadYears {
		t.Fatalf("same-month API request count = %d, want %d", requestCount.Load(), calendarHolidayPreloadYears)
	}
	if errorValue := service.refreshCalendarHolidaysIfDue(context.Background(), currentTime.AddDate(0, 1, 0)); errorValue != nil {
		t.Fatalf("next-month refresh: %v", errorValue)
	}
	if requestCount.Load() != calendarHolidayPreloadYears*2 {
		t.Fatalf("next-month API request count = %d, want %d", requestCount.Load(), calendarHolidayPreloadYears*2)
	}
}

func TestCalendarHolidayManualRefreshIgnoresMonthlyState(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = calendarHolidayTestHTTPClient(t, "KR", &requestCount)
	currentTime := time.Date(2026, time.July, 1, 0, 5, 0, 0, time.FixedZone("KST", 9*60*60))

	for range 2 {
		if errorValue := service.refreshCalendarHolidayCache(context.Background(), currentTime); errorValue != nil {
			t.Fatalf("manual refresh: %v", errorValue)
		}
	}

	if requestCount.Load() != calendarHolidayPreloadYears*2 {
		t.Fatalf("API request count = %d, want %d", requestCount.Load(), calendarHolidayPreloadYears*2)
	}
}

func TestCalendarHolidayRefreshReplacesDatabaseAndMemorySnapshots(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		currentRequest := requestCount.Add(1)
		body := `[{"date":"2026-01-01","localName":"기존 휴일","name":"Old Holiday","countryCode":"KR","types":["Public"]}]`
		if currentRequest == 2 {
			body = `[{"date":"2026-03-01","localName":"새 휴일","name":"New Holiday","countryCode":"KR","types":["Public"]}]`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})}
	currentTime := time.Date(2026, time.July, 1, 0, 5, 0, 0, time.UTC)

	for range 2 {
		if errorValue := service.refreshNagerCalendarHolidayYears(
			context.Background(),
			"KR",
			2026,
			2026,
			currentTime,
		); errorValue != nil {
			t.Fatalf("refresh holiday year: %v", errorValue)
		}
	}

	key := newCalendarHolidayCacheKey("KR", workspaceLanguageKorean, 2026)
	memoryHolidays, found := service.readCalendarHolidayMemoryCache(key)
	if !found || len(memoryHolidays) != 1 || memoryHolidays[0].Title != "새 휴일" || memoryHolidays[0].Date != "2026-03-01" {
		t.Fatalf("memory holidays = %#v found=%v", memoryHolidays, found)
	}
	databaseHolidays, errorValue := service.readCalendarHolidayYear(context.Background(), key)
	if errorValue != nil {
		t.Fatalf("read database holidays: %v", errorValue)
	}
	if len(databaseHolidays) != 1 || databaseHolidays[0].Title != "새 휴일" || databaseHolidays[0].Date != "2026-03-01" {
		t.Fatalf("database holidays = %#v", databaseHolidays)
	}
}

func TestCalendarHolidayNextMonthlyRefreshUsesWorkspaceTimeZone(t *testing.T) {
	service := newCalendarTestService(t)
	currentTime := time.Date(2026, time.July, 31, 14, 30, 0, 0, time.UTC)

	nextRefresh := service.calendarHolidayNextMonthlyRefresh(currentTime)

	expected := time.Date(2026, time.July, 31, 15, 5, 0, 0, time.UTC)
	if !nextRefresh.Equal(expected) {
		t.Fatalf("next refresh = %s, want %s", nextRefresh.Format(time.RFC3339), expected.Format(time.RFC3339))
	}
}

func calendarHolidayTestHTTPClient(
	t *testing.T,
	countryCode string,
	requestCount *atomic.Int64,
) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if !strings.Contains(request.URL.Path, "/api/v3/PublicHolidays/") {
			t.Fatalf("unexpected Nager request path = %q", request.URL.Path)
		}
		requestCount.Add(1)
		segments := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
		year := segments[len(segments)-2]
		return calendarHolidayTestResponse(request, countryCode, year), nil
	})}
}

func calendarHolidayTestResponse(
	request *http.Request,
	countryCode string,
	year string,
) *http.Response {
	body := strings.NewReplacer(
		"YEAR",
		year,
		"COUNTRY",
		countryCode,
	).Replace(`[{"date":"YEAR-01-01","localName":"새해 첫날","name":"New Year's Day","countryCode":"COUNTRY","types":["Public"]}]`)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}
