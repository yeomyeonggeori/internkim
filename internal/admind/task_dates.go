package admind

import (
	"strings"
	"time"
)


type normalizedTaskDates struct {
	StartDate string
	EndDate   string
	WeekCode  string
}

func normalizeTaskDates(payload taskWriteRequest, status string, now time.Time) normalizedTaskDates {
	return normalizeTaskStatusDates(payload.StartDate, payload.EndDate, payload.WeekCode, status, now)
}

func canonicalTaskWeekCode(weekCode string, now time.Time) (string, error) {
	trimmedWeekCode := strings.TrimSpace(weekCode)
	if trimmedWeekCode == "" {
		return "", nil
	}
	canonicalWeekCode := canonicalTaskWeekCodeOfValue(trimmedWeekCode, now)
	if canonicalWeekCode == "" {
		return "", taskValidationError("weekCode must be a valid ISO week")
	}
	return canonicalWeekCode, nil
}

func validateTaskDateInput(startDate string, endDate string, location *time.Location) error {
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "startDate", value: startDate},
		{name: "endDate", value: endDate},
	} {
		trimmedValue := strings.TrimSpace(field.value)
		if trimmedValue == "" {
			continue
		}
		if _, errorValue := time.ParseInLocation("2006-01-02", trimmedValue, location); errorValue != nil {
			return taskValidationError(field.name + " must be a valid YYYY-MM-DD date")
		}
	}
	return nil
}

func normalizeTaskStatusDates(startDate string, endDate string, weekCode string, status string, now time.Time) normalizedTaskDates {
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)
	weekCode = strings.TrimSpace(weekCode)
	today := now.Format("2006-01-02")
	switch {
	case isTaskCompletedStatus(status):
		if endDate == "" {
			endDate = today
		}
		if startDate == "" {
			startDate = endDate
		}
		weekCode = weekCodeForTaskDate(endDate, now)
	case isTaskPlannedStatus(status):
		if startDate == "" {
			weekCode = weekCodeForDate(now)
			break
		}
		weekCode = weekCodeForTaskDate(startDate, now)
	default:
		if weekCode == "" {
			weekCode = weekCodeForDate(now)
		}
	}
	return normalizedTaskDates{StartDate: startDate, EndDate: endDate, WeekCode: weekCode}
}

func weekCodeForTaskDate(dateText string, fallback time.Time) string {
	date, errorValue := time.ParseInLocation("2006-01-02", strings.TrimSpace(dateText), fallback.Location())
	if errorValue != nil {
		return weekCodeForDate(fallback)
	}
	return weekCodeForDate(date)
}

