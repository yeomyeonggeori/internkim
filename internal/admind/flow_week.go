// Flow 주차 코드와 주간 범위 계산을 담당합니다.
package admind

import (
	"strconv"
	"strings"
	"time"
)

func buildFlowWeek(weekCode string, weekStart time.Time, now time.Time) flowWeek {
	return flowWeek{
		Code:      weekCode,
		StartISO:  weekStart.Format("2006-01-02"),
		EndISO:    weekStart.AddDate(0, 0, 6).Format("2006-01-02"),
		Previous:  weekCodeForDate(weekStart.AddDate(0, 0, -7)),
		Next:      weekCodeForDate(weekStart.AddDate(0, 0, 7)),
		IsCurrent: weekCode == weekCodeForDate(now),
	}
}

func weekStartForCode(weekCode string, fallback time.Time) time.Time {
	trimmedCode := strings.TrimSpace(strings.ToUpper(weekCode))
	if len(trimmedCode) != 5 || trimmedCode[2] != 'W' {
		return startOfISOWeek(fallback)
	}
	yearValue, yearError := strconv.Atoi(trimmedCode[:2])
	weekValue, weekError := strconv.Atoi(trimmedCode[3:])
	if yearError != nil || weekError != nil || weekValue < 1 || weekValue > 53 {
		return startOfISOWeek(fallback)
	}
	year := 2000 + yearValue
	janFourth := time.Date(year, time.January, 4, 0, 0, 0, 0, fallback.Location())
	return startOfISOWeek(janFourth).AddDate(0, 0, (weekValue-1)*7)
}

func weekCodeForDate(date time.Time) string {
	year, week := date.ISOWeek()
	return strconv.Itoa(year%100) + "W" + twoDigitNumber(week)
}

func startOfISOWeek(date time.Time) time.Time {
	year, month, day := date.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, date.Location())
	weekday := int(start.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return start.AddDate(0, 0, 1-weekday)
}

func twoDigitNumber(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
