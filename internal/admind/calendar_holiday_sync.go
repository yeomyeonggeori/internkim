package admind

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	calendarHolidayCountryCacheTTL   = 24 * time.Hour
	calendarHolidayMaximumAttempts   = 3
	calendarHolidayPreloadYears      = 2
	calendarHolidayMaximumRangeYears = 2
)

func (service *Service) serveCalendarHolidays(responseWriter http.ResponseWriter, request *http.Request) {
	query := request.URL.Query()
	if strings.TrimSpace(query.Get("startISO")) == "" || strings.TrimSpace(query.Get("endISO")) == "" {
		http.Error(responseWriter, "startISO and endISO are required", http.StatusBadRequest)
		return
	}
	startTime, endTime, errorValue := parseCalendarRange(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	locale := normalizeCalendarHolidayLocale(query.Get("locale"))
	countryCode := service.workspaceCountryCode()
	if !service.calendarHolidayRangeIsSupported(startTime, endTime) {
		http.Error(responseWriter, "calendar holiday range cannot exceed two calendar years", http.StatusBadRequest)
		return
	}
	currentTime := time.Now().UTC()
	_, refreshError := service.refreshCalendarHolidaysOnRequest(request.Context(), currentTime)
	if refreshError != nil {
		holidays, found, storedError := service.calendarHolidaysFromStoredRange(
			request.Context(),
			countryCode,
			locale,
			startTime,
			endTime,
		)
		if storedError != nil {
			http.Error(responseWriter, storedError.Error(), http.StatusBadGateway)
			return
		}
		if !found {
			http.Error(responseWriter, refreshError.Error(), http.StatusBadGateway)
			return
		}
		slog.WarnContext(request.Context(), "calendar holiday request refresh failed",
			"country_code", countryCode,
			"error", refreshError,
		)
		service.writeJSON(responseWriter, calendarHolidaysResponse{Holidays: holidays, Source: calendarHolidaySourceAPI})
		return
	}
	holidays, errorValue := service.calendarHolidaysForRange(
		request.Context(),
		countryCode,
		locale,
		startTime,
		endTime,
		currentTime,
	)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, calendarHolidaysResponse{Holidays: holidays, Source: calendarHolidaySourceAPI})
}

func (service *Service) refreshCalendarHolidayCache(ctx context.Context, currentTime time.Time) error {
	service.calendarHolidayLoadMutex.Lock()
	defer service.calendarHolidayLoadMutex.Unlock()
	if errorValue := service.refreshCalendarHolidayCacheLocked(ctx, currentTime); errorValue != nil {
		return errorValue
	}
	service.calendarHolidayRetryAt = time.Time{}
	service.holidayCheckedMonth = service.calendarHolidayMonthKey(currentTime)
	return nil
}

func (service *Service) refreshCalendarHolidayCacheLocked(ctx context.Context, currentTime time.Time) error {
	startTime, endTime := service.calendarHolidayPreloadRange(currentTime)
	workspaceLocation, _ := service.workspaceTimeLocation()
	return service.refreshNagerCalendarHolidayYears(
		ctx,
		service.workspaceCountryCode(),
		startTime.In(workspaceLocation).Year(),
		endTime.In(workspaceLocation).AddDate(0, 0, -1).Year(),
		currentTime,
	)
}

func normalizeCalendarHolidayLocale(locale string) string {
	if strings.EqualFold(strings.TrimSpace(locale), workspaceLanguageKorean) {
		return workspaceLanguageKorean
	}
	return workspaceLanguageEnglish
}

func (service *Service) calendarHolidayPreloadRange(currentTime time.Time) (time.Time, time.Time) {
	workspaceLocation, _ := service.workspaceTimeLocation()
	currentYear := currentTime.In(workspaceLocation).Year()
	startTime := time.Date(currentYear, time.January, 1, 0, 0, 0, 0, workspaceLocation)
	return startTime, startTime.AddDate(calendarHolidayPreloadYears, 0, 0)
}

func (service *Service) calendarHolidayRangeIsSupported(startTime time.Time, endTime time.Time) bool {
	workspaceLocation, _ := service.workspaceTimeLocation()
	startYear := startTime.In(workspaceLocation).Year()
	endYear := endTime.In(workspaceLocation).AddDate(0, 0, -1).Year()
	return endYear-startYear+1 <= calendarHolidayMaximumRangeYears
}

func calendarHolidayRetryDelay(attempt int) time.Duration {
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}
