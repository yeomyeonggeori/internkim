package admind

import (
	"strings"
	"time"
)

func attendanceStatusForEvents(events []attendanceEvent, localDate string, now time.Time) string {
	for _, event := range events {
		if event.LocalDate != localDate || event.CanceledAt != "" {
			continue
		}
		if attendanceEventIsAfter(event, now) {
			continue
		}
		if event.Kind == attendanceKindClockIn {
			return "clocked_in"
		}
		if event.Kind == attendanceKindClockOut {
			return "clocked_out"
		}
	}
	return "not_clocked_in"
}

func attendanceEventIsAfter(event attendanceEvent, now time.Time) bool {
	occurredAt, errorValue := parseAttendanceEventTime(event.OccurredAt)
	return errorValue == nil && occurredAt.After(now)
}

func normalizeAttendanceMonth(value string, fallback time.Time) string {
	trimmedValue := strings.TrimSpace(value)
	if _, errorValue := time.Parse("2006-01", trimmedValue); errorValue == nil {
		return trimmedValue
	}
	return fallback.Format("2006-01")
}

func attendanceNextMonth(month string) string {
	parsedTime, errorValue := time.Parse("2006-01", month)
	if errorValue != nil {
		return month
	}
	return parsedTime.AddDate(0, 1, 0).Format("2006-01")
}

func attendanceEventOccurredWithin(event attendanceEvent, now time.Time, window time.Duration) bool {
	occurredAt, errorValue := parseAttendanceEventTime(event.OccurredAt)
	return errorValue == nil && now.Sub(occurredAt) >= 0 && now.Sub(occurredAt) <= window
}

func parseAttendanceEventTime(value string) (time.Time, error) {
	occurredAt, errorValue := time.Parse(time.RFC3339Nano, value)
	if errorValue == nil {
		return occurredAt, nil
	}
	return time.Parse(time.RFC3339, value)
}

func attendanceMessageForKind(text localizedAdminText, kind string) string {
	if kind == attendanceKindClockOut {
		return text.AttendanceClockOut
	}
	return text.AttendanceClockIn
}

func nextAttendanceKind(event attendanceEvent, hasEvent bool) string {
	if hasEvent && event.Kind == attendanceKindClockIn {
		return attendanceKindClockOut
	}
	return attendanceKindClockIn
}

func shouldIgnoreAttendanceAction(kind string, event attendanceEvent, hasEvent bool) bool {
	if kind == attendanceKindClockIn {
		return hasEvent && event.Kind == attendanceKindClockIn
	}
	if kind == attendanceKindClockOut {
		return !hasEvent || event.Kind != attendanceKindClockIn
	}
	return true
}
