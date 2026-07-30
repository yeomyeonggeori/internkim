package admind

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	calendarHolidayCacheTTL        = 24 * time.Hour
	calendarHolidayMaximumAttempts = 3
	calendarHolidayPreloadYears    = 2
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
	source, syncError := service.ensureCalendarHolidaysForRange(request.Context(), locale, startTime, endTime, time.Now().UTC())
	countryCode := service.workspaceCountryCode()
	workspaceLocation, _ := service.workspaceTimeLocation()
	holidays, errorValue := service.readCalendarHolidays(
		request.Context(),
		source,
		countryCode,
		locale,
		startTime.In(workspaceLocation).Format(time.DateOnly),
		endTime.In(workspaceLocation).Format(time.DateOnly),
	)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if syncError != nil {
		if len(holidays) == 0 {
			http.Error(responseWriter, syncError.Error(), http.StatusBadGateway)
			return
		}
		slog.WarnContext(request.Context(), "calendar holiday sync failed; serving cached holidays",
			"source", source,
			"country_code", countryCode,
			"error", syncError,
		)
	}
	service.writeJSON(responseWriter, calendarHolidaysResponse{Holidays: holidays, Source: source})
}

func (service *Service) refreshCalendarHolidays(responseWriter http.ResponseWriter, request *http.Request) {
	currentTime := time.Now().UTC()
	errorValue := service.refreshCalendarHolidayCache(request.Context(), currentTime)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]string{"source": calendarHolidaySourceAPI})
}

func (service *Service) refreshCalendarHolidayCache(ctx context.Context, currentTime time.Time) error {
	if errorValue := service.invalidateCalendarHolidaySyncState(ctx); errorValue != nil {
		return errorValue
	}
	startTime, endTime := service.calendarHolidayPreloadRange(currentTime)
	for _, locale := range [...]string{workspaceLanguageKorean, workspaceLanguageEnglish} {
		if errorValue := service.syncNagerCalendarHolidays(
			ctx,
			service.workspaceCountryCode(),
			locale,
			startTime,
			endTime,
			currentTime,
		); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) invalidateCalendarHolidaySyncState(ctx context.Context) error {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
UPDATE calendar_holiday_sources
SET last_synced_at = '', last_error = ''
WHERE source_key <> ?`,
		calendarHolidayCountriesSourceKey,
	)
	return errorValue
}

func (service *Service) ensureCalendarHolidaysForRange(
	ctx context.Context,
	locale string,
	startTime time.Time,
	endTime time.Time,
	currentTime time.Time,
) (string, error) {
	countryCode := service.workspaceCountryCode()
	preloadStart, preloadEnd := service.calendarHolidayPreloadRange(currentTime)
	preloadError := service.syncNagerCalendarHolidays(ctx, countryCode, locale, preloadStart, preloadEnd, currentTime)
	if !startTime.Before(preloadStart) && !endTime.After(preloadEnd) {
		return calendarHolidaySourceAPI, preloadError
	}
	if errorValue := service.syncNagerCalendarHolidays(ctx, countryCode, locale, startTime, endTime, currentTime); errorValue != nil {
		return calendarHolidaySourceAPI, errorValue
	}
	return calendarHolidaySourceAPI, preloadError
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

func calendarHolidayRetryDelay(attempt int) time.Duration {
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}
