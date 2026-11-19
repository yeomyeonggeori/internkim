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

func weekCodeForDate(date time.Time) string {
	year, week := date.ISOWeek()
	return strconv.Itoa(year%100) + "W" + twoDigitNumber(week)
}

func twoDigitNumber(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}
