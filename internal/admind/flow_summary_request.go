package admind

import (
	"fmt"
	"strings"
	"time"
)

func parseFlowSummaryWeekCode(value string, now time.Time) (string, error) {
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return weekCodeForDate(now), nil
	}
	weekCode := canonicalFlowSummaryWeekCode(trimmedValue, now)
	if weekCode == "" {
		return "", fmt.Errorf("invalid flow summary week %q", value)
	}
	return weekCode, nil
}

func canonicalFlowSummaryWeekCode(value string, now time.Time) string {
	if flowWeekCodePattern.FindString(value) != value {
		return ""
	}
	weekCode := canonicalWeekCode(value)
	if weekCode == "" {
		return ""
	}
	year, week := weekStartForCode(weekCode, now).ISOWeek()
	if twoDigitNumber(year%100)+"W"+twoDigitNumber(week) != weekCode {
		return ""
	}
	return weekCode
}
