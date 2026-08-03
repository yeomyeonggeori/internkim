package admind

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalendarHolidayRefreshKeepsSuccessfulYearWhenAnotherYearFails(t *testing.T) {
	service := newCalendarTestService(t)
	var requestCount atomic.Int64
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestCount.Add(1)
		if strings.Contains(request.URL.Path, "/2027/") {
			return nil, io.EOF
		}
		return jsonResponse(http.StatusOK, `[{"date":"2026-01-01","localName":"신정","name":"New Year's Day","countryCode":"KR","types":["Public"]}]`, nil), nil
	})}
	currentTime := time.Date(2026, time.July, 1, 0, 5, 0, 0, time.UTC)

	errorValue := service.refreshNagerCalendarHolidaySelectedYears(
		context.Background(),
		"KR",
		[]int{2026, 2027},
		currentTime,
		true,
	)
	var providerError *calendarHolidayProviderRefreshError
	if !errors.As(errorValue, &providerError) || len(providerError.Failures) != 1 || providerError.Failures[0].Year != 2027 {
		t.Fatalf("provider error = %#v", errorValue)
	}
	key := newCalendarHolidayCacheKey("KR", workspaceLanguageKorean, 2026)
	holidays, found := service.readCalendarHolidayMemoryCache(key)
	if !found || len(holidays) != 1 || holidays[0].Title != "신정" {
		t.Fatalf("2026 holidays = %#v found=%t", holidays, found)
	}
	if requestCount.Load() != 1+calendarHolidayMaximumAttempts {
		t.Fatalf("request count = %d", requestCount.Load())
	}
}

func TestCalendarHolidayStatusExposesStoredProviderErrorAndRetry(t *testing.T) {
	service := newCalendarTestService(t)
	currentTime := time.Date(2026, time.July, 1, 0, 5, 0, 0, time.UTC)
	refreshError := errors.New("nager API status 503 for KR/2026: maintenance")
	if _, errorValue := service.recordCalendarHolidayRefreshFailure("KR", 2026, currentTime, refreshError); errorValue != nil {
		t.Fatalf("record refresh failure: %v", errorValue)
	}
	if errorValue := service.recordNagerCalendarHolidayError(context.Background(), "KR", 2026, refreshError); errorValue != nil {
		t.Fatalf("record provider error: %v", errorValue)
	}

	status, errorValue := service.calendarHolidayStatus(context.Background(), currentTime)
	if errorValue != nil {
		t.Fatalf("calendar holiday status: %v", errorValue)
	}
	if status.Status != "degraded" || len(status.Years) != calendarHolidayPreloadYears {
		t.Fatalf("status = %#v", status)
	}
	if status.Years[0].LastError != refreshError.Error() {
		t.Fatalf("last error = %q", status.Years[0].LastError)
	}
	wantRetryAt := currentTime.Add(calendarHolidayProviderRetryDelays[0]).Format(time.RFC3339)
	if status.Years[0].NextRetryAt != wantRetryAt {
		t.Fatalf("next retry = %q, want %q", status.Years[0].NextRetryAt, wantRetryAt)
	}
}

func TestCalendarHolidayRetryBackoffIsBounded(t *testing.T) {
	service := newCalendarTestService(t)
	currentTime := time.Date(2026, time.July, 1, 0, 5, 0, 0, time.UTC)
	for failureIndex := range 6 {
		state, errorValue := service.recordCalendarHolidayRefreshFailure("KR", 2026, currentTime, errors.New("provider unavailable"))
		if errorValue != nil {
			t.Fatalf("record refresh failure: %v", errorValue)
		}
		wantDelay := calendarHolidayProviderRetryDelays[min(failureIndex, len(calendarHolidayProviderRetryDelays)-1)]
		if state.NextRetryAt.Sub(currentTime) != wantDelay {
			t.Fatalf("failure %d delay = %s, want %s", failureIndex+1, state.NextRetryAt.Sub(currentTime), wantDelay)
		}
	}
}

func TestCalendarHolidayRefreshSuccessKeepsLastAttemptAndClearsFailure(t *testing.T) {
	service := newCalendarTestService(t)
	currentTime := time.Date(2026, time.July, 1, 0, 5, 0, 0, time.UTC)
	if _, errorValue := service.recordCalendarHolidayRefreshFailure("KR", 2026, currentTime.Add(-time.Minute), errors.New("provider unavailable")); errorValue != nil {
		t.Fatalf("record refresh failure: %v", errorValue)
	}
	if errorValue := service.recordCalendarHolidayRefreshSuccess("KR", 2026, currentTime); errorValue != nil {
		t.Fatalf("record refresh success: %v", errorValue)
	}

	state := service.calendarHolidayRetryState("KR", 2026)
	if !state.LastAttempt.Equal(currentTime) || state.FailureCount != 0 || !state.NextRetryAt.IsZero() || state.LastError != "" {
		t.Fatalf("retry state = %#v", state)
	}
}

func TestCalendarHolidayRetryStateSurvivesServiceRestart(t *testing.T) {
	service := newCalendarTestService(t)
	currentTime := time.Date(2026, time.July, 1, 0, 5, 0, 0, time.UTC)
	refreshError := errors.New("provider unavailable")
	if _, errorValue := service.recordCalendarHolidayRefreshFailure("KR", 2026, currentTime, refreshError); errorValue != nil {
		t.Fatalf("record refresh failure: %v", errorValue)
	}

	restartedService := NewService(service.Configuration)
	state := restartedService.calendarHolidayRetryState("kr", 2026)

	if state.FailureCount != 1 || !state.LastAttempt.Equal(currentTime) || !state.NextRetryAt.Equal(currentTime.Add(time.Minute)) || state.LastError != refreshError.Error() {
		t.Fatalf("retry state after restart = %#v", state)
	}
	if restartedService.calendarHolidayRetryAllowed("KR", 2026, currentTime.Add(30*time.Second)) {
		t.Fatal("retry was allowed before persisted cooldown elapsed")
	}
}

func TestCalendarHolidayRetryStateLoadFailureBlocksAutomaticRetryAndAppearsInStatus(t *testing.T) {
	service := newCalendarTestService(t)
	if errorValue := os.MkdirAll(service.Configuration.StateDirectory, 0o700); errorValue != nil {
		t.Fatalf("create state directory: %v", errorValue)
	}
	if errorValue := os.WriteFile(service.calendarHolidayRetryStatePath(), []byte("{"), 0o600); errorValue != nil {
		t.Fatalf("write malformed retry state: %v", errorValue)
	}

	restartedService := NewService(service.Configuration)
	currentTime := time.Date(2026, time.July, 1, 0, 5, 0, 0, time.UTC)
	if restartedService.calendarHolidayRetryAllowed("KR", 2026, currentTime) {
		t.Fatal("automatic retry was allowed with unreadable persisted state")
	}
	status, errorValue := restartedService.calendarHolidayStatus(context.Background(), currentTime)
	if errorValue != nil {
		t.Fatalf("calendar holiday status: %v", errorValue)
	}
	if status.Status != "degraded" || !strings.Contains(status.Years[0].LastError, "decode calendar holiday retry state") {
		t.Fatalf("status after retry state load failure = %#v", status)
	}
}
