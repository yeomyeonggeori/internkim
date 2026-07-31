package admind

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type calendarHolidayCacheKey struct {
	CountryCode string
	Locale      string
	Year        int
}

func (service *Service) calendarHolidaysForRange(
	ctx context.Context,
	countryCode string,
	locale string,
	startTime time.Time,
	endTime time.Time,
	currentTime time.Time,
) ([]calendarHoliday, error) {
	workspaceLocation, _ := service.workspaceTimeLocation()
	startYear := startTime.In(workspaceLocation).Year()
	endYear := endTime.In(workspaceLocation).AddDate(0, 0, -1).Year()
	if endYear-startYear+1 > calendarHolidayMaximumRangeYears {
		return nil, fmt.Errorf("calendar holiday range cannot exceed %d calendar years", calendarHolidayMaximumRangeYears)
	}
	startDate := startTime.In(workspaceLocation).Format(time.DateOnly)
	endDate := endTime.In(workspaceLocation).Format(time.DateOnly)
	holidays := make([]calendarHoliday, 0)
	for year := startYear; year <= endYear; year += 1 {
		yearHolidays, errorValue := service.ensureCalendarHolidayYear(
			ctx,
			countryCode,
			locale,
			year,
			currentTime,
		)
		if errorValue != nil {
			return nil, errorValue
		}
		for _, holiday := range yearHolidays {
			if holiday.Date >= startDate && holiday.Date < endDate {
				holidays = append(holidays, holiday)
			}
		}
	}
	return holidays, nil
}

func (service *Service) calendarHolidaysFromStoredRange(
	ctx context.Context,
	countryCode string,
	locale string,
	startTime time.Time,
	endTime time.Time,
) ([]calendarHoliday, bool, error) {
	service.calendarHolidayLoadMutex.Lock()
	defer service.calendarHolidayLoadMutex.Unlock()
	workspaceLocation, _ := service.workspaceTimeLocation()
	startYear := startTime.In(workspaceLocation).Year()
	endYear := endTime.In(workspaceLocation).AddDate(0, 0, -1).Year()
	startDate := startTime.In(workspaceLocation).Format(time.DateOnly)
	endDate := endTime.In(workspaceLocation).Format(time.DateOnly)
	holidays := make([]calendarHoliday, 0)
	for year := startYear; year <= endYear; year += 1 {
		yearHolidays, found, errorValue := service.loadStoredCalendarHolidayYear(
			ctx,
			newCalendarHolidayCacheKey(countryCode, locale, year),
		)
		if errorValue != nil {
			return nil, false, errorValue
		}
		if !found {
			return nil, false, nil
		}
		for _, holiday := range yearHolidays {
			if holiday.Date >= startDate && holiday.Date < endDate {
				holidays = append(holidays, holiday)
			}
		}
	}
	return holidays, true, nil
}

func (service *Service) ensureCalendarHolidayYear(
	ctx context.Context,
	countryCode string,
	locale string,
	year int,
	currentTime time.Time,
) ([]calendarHoliday, error) {
	key := newCalendarHolidayCacheKey(countryCode, locale, year)
	if holidays, found := service.readCalendarHolidayMemoryCache(key); found {
		return holidays, nil
	}
	service.calendarHolidayLoadMutex.Lock()
	defer service.calendarHolidayLoadMutex.Unlock()
	if holidays, found := service.readCalendarHolidayMemoryCache(key); found {
		return holidays, nil
	}
	holidays, found, errorValue := service.loadStoredCalendarHolidayYear(ctx, key)
	if errorValue != nil {
		return nil, errorValue
	}
	if found {
		return holidays, nil
	}
	sourceKey := calendarHolidaySourceKey(key.CountryCode, key.Year, key.Locale)
	if errorValue := service.refreshNagerCalendarHolidayYears(
		ctx,
		key.CountryCode,
		key.Year,
		key.Year,
		currentTime,
	); errorValue != nil {
		return nil, errorValue
	}
	holidays, found = service.readCalendarHolidayMemoryCache(key)
	if !found {
		return nil, fmt.Errorf("calendar holiday cache missing after refresh for %s", sourceKey)
	}
	return holidays, nil
}

func (service *Service) loadStoredCalendarHolidayYear(
	ctx context.Context,
	key calendarHolidayCacheKey,
) ([]calendarHoliday, bool, error) {
	if holidays, found := service.readCalendarHolidayMemoryCache(key); found {
		return holidays, true, nil
	}
	sourceKey := calendarHolidaySourceKey(key.CountryCode, key.Year, key.Locale)
	state, found, errorValue := service.readCalendarHolidaySource(ctx, calendarHolidayProviderNager, sourceKey)
	if errorValue != nil {
		return nil, false, errorValue
	}
	if found && strings.TrimSpace(state.LastSyncedAt) != "" {
		holidays, readError := service.readCalendarHolidayYear(ctx, key)
		if readError != nil {
			return nil, false, readError
		}
		service.writeCalendarHolidayMemoryCache(key, holidays)
		return holidays, true, nil
	}
	return nil, false, nil
}

func (service *Service) readCalendarHolidayYear(
	ctx context.Context,
	key calendarHolidayCacheKey,
) ([]calendarHoliday, error) {
	startDate := time.Date(key.Year, time.January, 1, 0, 0, 0, 0, time.UTC)
	return service.readCalendarHolidays(
		ctx,
		calendarHolidaySourceAPI,
		key.CountryCode,
		key.Locale,
		startDate.Format(time.DateOnly),
		startDate.AddDate(1, 0, 0).Format(time.DateOnly),
	)
}

func newCalendarHolidayCacheKey(countryCode string, locale string, year int) calendarHolidayCacheKey {
	return calendarHolidayCacheKey{
		CountryCode: strings.ToUpper(strings.TrimSpace(countryCode)),
		Locale:      normalizeCalendarHolidayLocale(locale),
		Year:        year,
	}
}

func (service *Service) readCalendarHolidayMemoryCache(
	key calendarHolidayCacheKey,
) ([]calendarHoliday, bool) {
	service.calendarHolidayCacheMutex.RLock()
	defer service.calendarHolidayCacheMutex.RUnlock()
	holidays, found := service.calendarHolidayCache[key]
	return cloneCalendarHolidays(holidays), found
}

func (service *Service) writeCalendarHolidayMemoryCache(
	key calendarHolidayCacheKey,
	holidays []calendarHoliday,
) {
	service.calendarHolidayCacheMutex.Lock()
	defer service.calendarHolidayCacheMutex.Unlock()
	service.calendarHolidayCache[key] = cloneCalendarHolidays(holidays)
}

func (service *Service) clearCalendarHolidayMemoryCache() {
	service.calendarHolidayCacheMutex.Lock()
	defer service.calendarHolidayCacheMutex.Unlock()
	service.calendarHolidayCache = map[calendarHolidayCacheKey][]calendarHoliday{}
}

func cloneCalendarHolidays(holidays []calendarHoliday) []calendarHoliday {
	return append([]calendarHoliday(nil), holidays...)
}
