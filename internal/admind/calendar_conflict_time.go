package admind

import "time"

func parseCalendarConflictTime(value string) time.Time {
	parsed, errorValue := time.Parse(time.RFC3339Nano, value)
	if errorValue == nil {
		return parsed.UTC()
	}
	parsed, errorValue = time.Parse(time.RFC3339, value)
	if errorValue != nil {
		return time.Time{}
	}
	return parsed.UTC()
}
