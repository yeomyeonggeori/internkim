package admind

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var taskWeekCodePattern = regexp.MustCompile(`(\d{2,4})\D*[Ww]?\D*(\d{1,2})`)

func canonicalWeekCode(weekCode string) string {
	match := taskWeekCodePattern.FindStringSubmatch(strings.TrimSpace(weekCode))
	if match == nil {
		return ""
	}
	yearValue, _ := strconv.Atoi(match[1])
	weekValue, _ := strconv.Atoi(match[2])
	if weekValue < 1 || weekValue > 53 {
		return ""
	}
	return twoDigitNumber(yearValue%100) + "W" + twoDigitNumber(weekValue)
}

func buildTaskWeek(weekCode string, weekStart time.Time, now time.Time) taskWeek {
	return taskWeek{
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
