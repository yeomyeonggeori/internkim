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
	weekCode := canonicalWeekCode(trimmedValue)
	if weekCode == "" {
		return "", fmt.Errorf("invalid flow summary week %q", value)
	}
	return weekCode, nil
}
