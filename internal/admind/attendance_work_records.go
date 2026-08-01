package admind

import (
	"sort"
	"strings"
	"time"
)

type attendanceWorkSegment struct {
	Start time.Time
	End   time.Time
}

type attendanceWorkRecords struct {
	CompletedSegments []attendanceWorkSegment
	Provisional       *attendanceWorkSegment
	IncompleteDates   map[string]struct{}
}

func attendanceWorkRecordsForEmail(
	events []attendanceEvent,
	email string,
	now time.Time,
	location *time.Location,
) attendanceWorkRecords {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	filtered := make([]attendanceEvent, 0, len(events))
	for _, event := range events {
		if event.CanceledAt != "" || !strings.EqualFold(event.Email, normalizedEmail) {
			continue
		}
		filtered = append(filtered, event)
	}
	sort.Slice(filtered, func(left int, right int) bool {
		return filtered[left].OccurredAt < filtered[right].OccurredAt
	})
	result := attendanceWorkRecords{
		CompletedSegments: make([]attendanceWorkSegment, 0, len(filtered)/2),
		IncompleteDates:   make(map[string]struct{}),
	}
	var startedAt time.Time
	for _, event := range filtered {
		occurredAt, errorValue := parseAttendanceEventTime(event.OccurredAt)
		if errorValue != nil {
			continue
		}
		if event.Kind == attendanceKindClockIn {
			if !startedAt.IsZero() {
				result.IncompleteDates[startedAt.In(location).Format(time.DateOnly)] = struct{}{}
			}
			startedAt = occurredAt
			continue
		}
		if event.Kind != attendanceKindClockOut {
			continue
		}
		if startedAt.IsZero() || !occurredAt.After(startedAt) {
			result.IncompleteDates[occurredAt.In(location).Format(time.DateOnly)] = struct{}{}
			continue
		}
		result.CompletedSegments = append(
			result.CompletedSegments,
			attendanceWorkSegment{Start: startedAt, End: occurredAt},
		)
		startedAt = time.Time{}
	}
	if !startedAt.IsZero() && now.After(startedAt) {
		if startedAt.In(location).Format(time.DateOnly) == now.In(location).Format(time.DateOnly) {
			result.Provisional = &attendanceWorkSegment{Start: startedAt, End: now}
		} else {
			result.IncompleteDates[startedAt.In(location).Format(time.DateOnly)] = struct{}{}
		}
	}
	return result
}

func attendanceLeaveByDate(
	occurrences []attendanceApprovedLeaveOccurrence,
	email string,
) map[string][]attendanceApprovedLeaveOccurrence {
	result := make(map[string][]attendanceApprovedLeaveOccurrence)
	for _, occurrence := range occurrences {
		if strings.EqualFold(occurrence.Email, email) {
			result[occurrence.Date] = append(result[occurrence.Date], occurrence)
		}
	}
	return result
}

func attendanceLeaveIntervals(
	date time.Time,
	occurrences []attendanceApprovedLeaveOccurrence,
	location *time.Location,
) []attendanceWorkSegment {
	intervals := make([]attendanceWorkSegment, 0, len(occurrences))
	for _, occurrence := range occurrences {
		startMinute, startError := attendanceWorkScheduleTimeMinutes(occurrence.StartTime)
		endMinute, endError := attendanceWorkScheduleTimeMinutes(occurrence.EndTime)
		if startError != nil || endError != nil || endMinute <= startMinute {
			continue
		}
		dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, location)
		intervals = append(intervals, attendanceWorkSegment{
			Start: dayStart.Add(time.Duration(startMinute) * time.Minute),
			End:   dayStart.Add(time.Duration(endMinute) * time.Minute),
		})
	}
	return intervals
}

func attendanceSegmentsOverlap(
	workSegments []attendanceWorkSegment,
	leaveIntervals []attendanceWorkSegment,
) bool {
	for _, workSegment := range workSegments {
		for _, leaveInterval := range leaveIntervals {
			if earliestTime(workSegment.End, leaveInterval.End).After(
				latestTime(workSegment.Start, leaveInterval.Start),
			) {
				return true
			}
		}
	}
	return false
}
