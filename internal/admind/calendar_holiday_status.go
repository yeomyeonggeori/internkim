package admind

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
)

const calendarHolidayStatusErrorLimit = 4096

type calendarHolidayYearStatus struct {
	Year          int    `json:"year"`
	Status        string `json:"status"`
	CacheCount    int    `json:"cacheCount"`
	LastSyncedAt  string `json:"lastSyncedAt,omitempty"`
	LastAttemptAt string `json:"lastAttemptAt,omitempty"`
	LastError     string `json:"lastError,omitempty"`
	NextRetryAt   string `json:"nextRetryAt,omitempty"`
}

type calendarHolidayStatusResponse struct {
	Status      string                      `json:"status"`
	CountryCode string                      `json:"countryCode"`
	Provider    string                      `json:"provider"`
	Years       []calendarHolidayYearStatus `json:"years"`
}

func (service *Service) writeCalendarHolidayStatus(responseWriter http.ResponseWriter, request *http.Request) {
	status, errorValue := service.calendarHolidayStatus(request.Context(), time.Now().UTC())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, status)
}

func (service *Service) refreshCalendarHolidayStatus(responseWriter http.ResponseWriter, request *http.Request) {
	currentTime := time.Now().UTC()
	refreshContext, cancel := context.WithTimeout(request.Context(), calendarHolidayRefreshTimeout)
	defer cancel()
	refreshError := service.refreshCalendarHolidayCache(refreshContext, currentTime)
	if refreshError != nil {
		var providerError *calendarHolidayProviderRefreshError
		if !errors.As(refreshError, &providerError) {
			http.Error(responseWriter, refreshError.Error(), http.StatusInternalServerError)
			return
		}
	}
	status, errorValue := service.calendarHolidayStatus(request.Context(), currentTime)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, status)
}

func (service *Service) calendarHolidayStatus(ctx context.Context, currentTime time.Time) (calendarHolidayStatusResponse, error) {
	countryCode := service.workspaceCountryCode()
	locale := normalizeCalendarHolidayLocale(service.workspaceLanguage())
	startTime, endTime := service.calendarHolidayPreloadRange(currentTime)
	workspaceLocation, _ := service.workspaceTimeLocation()
	startYear := startTime.In(workspaceLocation).Year()
	endYear := endTime.In(workspaceLocation).AddDate(0, 0, -1).Year()
	status := calendarHolidayStatusResponse{
		Status:      "healthy",
		CountryCode: countryCode,
		Provider:    calendarHolidayProviderNager,
		Years:       make([]calendarHolidayYearStatus, 0, endYear-startYear+1),
	}
	for year := startYear; year <= endYear; year += 1 {
		yearStatus, errorValue := service.calendarHolidayStatusForYear(ctx, countryCode, locale, year, currentTime.In(workspaceLocation))
		if errorValue != nil {
			return calendarHolidayStatusResponse{}, errorValue
		}
		status.Years = append(status.Years, yearStatus)
		if yearStatus.Status == "degraded" {
			status.Status = "degraded"
		} else if yearStatus.Status == "neverSynced" && status.Status == "healthy" {
			status.Status = "neverSynced"
		}
	}
	return status, nil
}

func (service *Service) calendarHolidayStatusForYear(
	ctx context.Context,
	countryCode string,
	locale string,
	year int,
	currentTime time.Time,
) (calendarHolidayYearStatus, error) {
	sourceKey := calendarHolidaySourceKey(countryCode, year, locale)
	source, found, errorValue := service.readCalendarHolidaySource(ctx, calendarHolidayProviderNager, sourceKey)
	if errorValue != nil {
		return calendarHolidayYearStatus{}, errorValue
	}
	cacheCount, errorValue := service.calendarHolidayCacheCount(ctx, sourceKey)
	if errorValue != nil {
		return calendarHolidayYearStatus{}, errorValue
	}
	retryState := service.calendarHolidayRetryState(countryCode, year)
	status := calendarHolidayYearStatus{
		Year:         year,
		Status:       "neverSynced",
		CacheCount:   cacheCount,
		LastSyncedAt: source.LastSyncedAt,
		LastError: boundedCalendarHolidayError(firstNonEmpty(
			retryState.LastError,
			source.LastError,
			service.calendarHolidayRetryLoadErrorMessage(),
		)),
	}
	if !retryState.LastAttempt.IsZero() {
		status.LastAttemptAt = retryState.LastAttempt.Format(time.RFC3339)
	}
	if !retryState.NextRetryAt.IsZero() {
		status.NextRetryAt = retryState.NextRetryAt.Format(time.RFC3339)
	}
	if status.LastError != "" {
		status.Status = "degraded"
	} else if found && calendarHolidaySourceSyncedInMonth(source.LastSyncedAt, currentTime) {
		status.Status = "healthy"
	}
	return status, nil
}

func (service *Service) calendarHolidayCacheCount(ctx context.Context, sourceKey string) (int, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return 0, errorValue
	}
	defer database.Close()
	var count int
	errorValue = database.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM calendar_holidays
WHERE source = ? AND source_key = ?`, calendarHolidaySourceAPI, sourceKey).Scan(&count)
	return count, errorValue
}

func boundedCalendarHolidayError(message string) string {
	trimmed := strings.TrimSpace(message)
	characters := []rune(trimmed)
	if len(characters) <= calendarHolidayStatusErrorLimit {
		return trimmed
	}
	return string(characters[:calendarHolidayStatusErrorLimit])
}
