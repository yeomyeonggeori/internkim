package admind

import (
	"context"
	"time"
)

func (service *Service) readCalendarHolidayDatesForRange(
	ctx context.Context,
	startTime time.Time,
	endTime time.Time,
) (map[string]struct{}, error) {
	countryCode := service.workspaceCountryCode()
	locale := normalizeCalendarHolidayLocale(service.workspaceLanguage())
	nationalHolidays, found, errorValue := service.calendarHolidaysFromStoredRange(
		ctx,
		countryCode,
		locale,
		startTime,
		endTime,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	if !found {
		nationalHolidays, errorValue = service.calendarHolidaysForRange(
			ctx,
			countryCode,
			locale,
			startTime,
			endTime,
			time.Now().UTC(),
		)
		if errorValue != nil {
			return nil, errorValue
		}
	}
	companyHolidays, errorValue := service.readCalendarCompanyHolidaysForRange(ctx, startTime, endTime)
	if errorValue != nil {
		return nil, errorValue
	}
	holidayDates := make(map[string]struct{}, len(nationalHolidays)+len(companyHolidays))
	for _, holiday := range append(nationalHolidays, companyHolidays...) {
		holidayDates[holiday.Date] = struct{}{}
	}
	return holidayDates, nil
}
