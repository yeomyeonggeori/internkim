package admind

import (
	"fmt"
	"sort"
	"time"
)

func attendanceEventCacheMonthsForDates(dates []string) ([]string, error) {
	monthSet := map[string]struct{}{}
	for _, date := range dates {
		parsedDate, errorValue := time.Parse("2006-01-02", date)
		if errorValue != nil {
			return nil, fmt.Errorf("parse attendance event cache date %q: %w", date, errorValue)
		}
		monthSet[parsedDate.Format("2006-01")] = struct{}{}
	}
	return sortedAttendanceSummaryCacheMonths(monthSet), nil
}

func attendanceAbsenceCacheMonthsForDates(dates []string) ([]string, error) {
	monthSet := map[string]struct{}{}
	for _, date := range dates {
		parsedDate, errorValue := time.Parse("2006-01-02", date)
		if errorValue != nil {
			return nil, fmt.Errorf("parse attendance absence cache date %q: %w", date, errorValue)
		}
		monthStart := time.Date(parsedDate.Year(), parsedDate.Month(), 1, 0, 0, 0, 0, time.UTC)
		for monthOffset := -1; monthOffset <= 1; monthOffset++ {
			month := monthStart.AddDate(0, monthOffset, 0).Format("2006-01")
			gridStartDate, gridEndDate := attendanceMonthGridDateRange(month)
			if date >= gridStartDate && date < gridEndDate {
				monthSet[month] = struct{}{}
			}
		}
	}
	return sortedAttendanceSummaryCacheMonths(monthSet), nil
}

func sortedAttendanceSummaryCacheMonths(monthSet map[string]struct{}) []string {
	months := make([]string, 0, len(monthSet))
	for month := range monthSet {
		months = append(months, month)
	}
	sort.Strings(months)
	return months
}
