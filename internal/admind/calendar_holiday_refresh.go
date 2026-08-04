package admind

import (
	"context"
	"time"
)

const calendarHolidayRefreshTimeout = 30 * time.Second

func (service *Service) refreshCalendarHolidaysOnRequest(
	ctx context.Context,
	currentTime time.Time,
) (bool, error) {
	service.calendarHolidayLoadMutex.Lock()
	defer service.calendarHolidayLoadMutex.Unlock()
	monthKey := service.calendarHolidayMonthKey(currentTime)
	if service.holidayCheckedMonth == monthKey {
		return false, nil
	}
	years, errorValue := service.calendarHolidayYearsDue(ctx, currentTime)
	if errorValue != nil {
		return false, errorValue
	}
	if len(years) == 0 {
		service.holidayCheckedMonth = monthKey
		return false, nil
	}
	refreshContext, cancel := context.WithTimeout(ctx, calendarHolidayRefreshTimeout)
	defer cancel()
	if errorValue := service.refreshNagerCalendarHolidaySelectedYears(refreshContext, service.workspaceCountryCode(), years, currentTime, false); errorValue != nil {
		return false, errorValue
	}
	service.holidayCheckedMonth = monthKey
	return true, nil
}

func (service *Service) calendarHolidayMonthKey(currentTime time.Time) string {
	workspaceLocation, _ := service.workspaceTimeLocation()
	return service.workspaceCountryCode() + ":" + currentTime.In(workspaceLocation).Format("2006-01")
}

func (service *Service) calendarHolidayYearsDue(
	ctx context.Context,
	currentTime time.Time,
) ([]int, error) {
	workspaceLocation, _ := service.workspaceTimeLocation()
	localCurrentTime := currentTime.In(workspaceLocation)
	startTime, endTime := service.calendarHolidayPreloadRange(currentTime)
	startYear := startTime.In(workspaceLocation).Year()
	endYear := endTime.In(workspaceLocation).AddDate(0, 0, -1).Year()
	countryCode := service.workspaceCountryCode()
	years := make([]int, 0, endYear-startYear+1)
	for year := startYear; year <= endYear; year += 1 {
		due := false
		for _, locale := range [...]string{workspaceLanguageKorean, workspaceLanguageEnglish} {
			sourceKey := calendarHolidaySourceKey(countryCode, year, locale)
			state, found, errorValue := service.readCalendarHolidaySource(
				ctx,
				calendarHolidayProviderNager,
				sourceKey,
			)
			if errorValue != nil {
				return nil, errorValue
			}
			if !found || !calendarHolidaySourceSyncedInMonth(state.LastSyncedAt, localCurrentTime) {
				due = true
				break
			}
		}
		if due {
			years = append(years, year)
		}
	}
	return years, nil
}

func calendarHolidaySourceSyncedInMonth(timestamp string, currentTime time.Time) bool {
	parsed, errorValue := time.Parse(time.RFC3339, timestamp)
	if errorValue != nil {
		return false
	}
	localSyncedAt := parsed.In(currentTime.Location())
	return localSyncedAt.Year() == currentTime.Year() && localSyncedAt.Month() == currentTime.Month()
}
