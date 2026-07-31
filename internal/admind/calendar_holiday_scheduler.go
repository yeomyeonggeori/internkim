package admind

import (
	"context"
	"log/slog"
	"time"
)

const (
	calendarHolidayMonthlyRefreshMinute = 5
	calendarHolidayRefreshRetryDelay    = 24 * time.Hour
	calendarHolidayRefreshTimeout       = 30 * time.Second
)

func (service *Service) startCalendarHolidayScheduler(ctx context.Context) {
	go func() {
		nextDelay := service.runCalendarHolidayMonthlyRefresh(ctx, time.Now().UTC())
		for {
			timer := time.NewTimer(nextDelay)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case currentTime := <-timer.C:
				nextDelay = service.runCalendarHolidayMonthlyRefresh(ctx, currentTime.UTC())
			}
		}
	}()
}

func (service *Service) runCalendarHolidayMonthlyRefresh(
	ctx context.Context,
	currentTime time.Time,
) time.Duration {
	refreshContext, cancel := context.WithTimeout(ctx, calendarHolidayRefreshTimeout)
	defer cancel()
	if errorValue := service.refreshCalendarHolidaysIfDue(refreshContext, currentTime); errorValue != nil {
		slog.WarnContext(ctx, "monthly calendar holiday refresh failed",
			"country_code", service.workspaceCountryCode(),
			"error", errorValue,
		)
		return calendarHolidayRefreshRetryDelay
	}
	return service.calendarHolidayNextMonthlyRefresh(currentTime).Sub(currentTime)
}

func (service *Service) refreshCalendarHolidaysIfDue(
	ctx context.Context,
	currentTime time.Time,
) error {
	service.calendarHolidayLoadMutex.Lock()
	defer service.calendarHolidayLoadMutex.Unlock()
	due, errorValue := service.calendarHolidayMonthlyRefreshDue(ctx, currentTime)
	if errorValue != nil {
		return errorValue
	}
	if !due {
		return nil
	}
	return service.refreshCalendarHolidayCacheLocked(ctx, currentTime)
}

func (service *Service) calendarHolidayMonthlyRefreshDue(
	ctx context.Context,
	currentTime time.Time,
) (bool, error) {
	workspaceLocation, _ := service.workspaceTimeLocation()
	localCurrentTime := currentTime.In(workspaceLocation)
	startTime, endTime := service.calendarHolidayPreloadRange(currentTime)
	startYear := startTime.In(workspaceLocation).Year()
	endYear := endTime.In(workspaceLocation).AddDate(0, 0, -1).Year()
	countryCode := service.workspaceCountryCode()
	for year := startYear; year <= endYear; year += 1 {
		for _, locale := range [...]string{workspaceLanguageKorean, workspaceLanguageEnglish} {
			sourceKey := calendarHolidaySourceKey(countryCode, year, locale)
			state, found, errorValue := service.readCalendarHolidaySource(
				ctx,
				calendarHolidayProviderNager,
				sourceKey,
			)
			if errorValue != nil {
				return false, errorValue
			}
			if !found || !calendarHolidaySourceSyncedInMonth(state.LastSyncedAt, localCurrentTime) {
				return true, nil
			}
		}
	}
	return false, nil
}

func calendarHolidaySourceSyncedInMonth(timestamp string, currentTime time.Time) bool {
	parsed, errorValue := time.Parse(time.RFC3339, timestamp)
	if errorValue != nil {
		return false
	}
	localSyncedAt := parsed.In(currentTime.Location())
	return localSyncedAt.Year() == currentTime.Year() && localSyncedAt.Month() == currentTime.Month()
}

func (service *Service) calendarHolidayNextMonthlyRefresh(currentTime time.Time) time.Time {
	workspaceLocation, _ := service.workspaceTimeLocation()
	localCurrentTime := currentTime.In(workspaceLocation)
	return time.Date(
		localCurrentTime.Year(),
		localCurrentTime.Month()+1,
		1,
		0,
		calendarHolidayMonthlyRefreshMinute,
		0,
		0,
		workspaceLocation,
	).UTC()
}
