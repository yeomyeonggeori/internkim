package admind

import (
	"context"
	"fmt"
	"time"
)

const (
	calendarHolidayRefreshRetryDelay = 24 * time.Hour
	calendarHolidayRefreshTimeout    = 30 * time.Second
)

func (service *Service) refreshCalendarHolidaysOnRequest(
	ctx context.Context,
	currentTime time.Time,
) (bool, error) {
	service.calendarHolidayLoadMutex.Lock()
	defer service.calendarHolidayLoadMutex.Unlock()
	if currentTime.Before(service.calendarHolidayRetryAt) {
		return false, fmt.Errorf(
			"calendar holiday refresh retry is delayed until %s",
			service.calendarHolidayRetryAt.Format(time.RFC3339),
		)
	}
	due, errorValue := service.calendarHolidayMonthlyRefreshDue(ctx, currentTime)
	if errorValue != nil {
		return false, errorValue
	}
	if !due {
		return false, nil
	}
	refreshContext, cancel := context.WithTimeout(ctx, calendarHolidayRefreshTimeout)
	defer cancel()
	if errorValue := service.refreshCalendarHolidayCacheLocked(refreshContext, currentTime); errorValue != nil {
		service.calendarHolidayRetryAt = currentTime.Add(calendarHolidayRefreshRetryDelay)
		return false, errorValue
	}
	service.calendarHolidayRetryAt = time.Time{}
	return true, nil
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
