package admind

import (
	"strings"
	"time"
)

var flowDateTimezone = loadFlowDateTimezone()

type normalizedFlowTaskDates struct {
	StartDate string
	EndDate   string
	WeekCode  string
}

func normalizeFlowTaskDates(payload flowTaskWriteRequest, status string, now time.Time) normalizedFlowTaskDates {
	return normalizeFlowStatusDates(payload.StartDate, payload.EndDate, payload.WeekCode, status, now)
}

func normalizeFlowStatusDates(startDate string, endDate string, weekCode string, status string, now time.Time) normalizedFlowTaskDates {
	startDate = strings.TrimSpace(startDate)
	endDate = strings.TrimSpace(endDate)
	weekCode = strings.TrimSpace(weekCode)
	today := now.Format("2006-01-02")
	switch {
	case isFlowCompletedStatus(status):
		if endDate == "" {
			endDate = today
		}
		if startDate == "" {
			startDate = endDate
		}
		weekCode = weekCodeForFlowDate(endDate, now)
	case isFlowPlannedStatus(status):
		if startDate == "" {
			startDate = today
		}
		weekCode = weekCodeForFlowDate(startDate, now)
	default:
		if weekCode == "" {
			weekCode = weekCodeForDate(now)
		}
	}
	return normalizedFlowTaskDates{StartDate: startDate, EndDate: endDate, WeekCode: weekCode}
}

func weekCodeForFlowDate(dateText string, fallback time.Time) string {
	date, errorValue := time.ParseInLocation("2006-01-02", strings.TrimSpace(dateText), fallback.Location())
	if errorValue != nil {
		return weekCodeForDate(fallback)
	}
	return weekCodeForDate(date)
}

func flowDateNow() time.Time {
	return time.Now().In(flowDateLocation())
}

func flowDateLocation() *time.Location {
	return flowDateTimezone
}

func loadFlowDateTimezone() *time.Location {
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		return time.FixedZone("Asia/Seoul", 9*60*60)
	}
	return location
}
