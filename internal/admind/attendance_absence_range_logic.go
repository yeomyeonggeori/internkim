package admind

import (
	"sort"
	"strings"
	"time"
)

const attendanceAbsenceOccurrenceIDSeparator = "__date_"

type attendanceAbsenceDateRange struct {
	StartDate string
	EndDate   string
}

func normalizeAttendanceAbsenceDateRange(startDate string, endDate string) (string, string) {
	normalizedStartDate := strings.TrimSpace(startDate)
	normalizedEndDate := strings.TrimSpace(endDate)
	if normalizedEndDate == "" {
		normalizedEndDate = normalizedStartDate
	}
	return normalizedStartDate, normalizedEndDate
}

func attendanceMonthGridDateRange(month string) (string, string) {
	firstDate, errorValue := time.Parse("2006-01-02", month+"-01")
	if errorValue != nil {
		return month + "-01", attendanceNextMonth(month) + "-01"
	}
	startDate := firstDate.AddDate(0, 0, -int(firstDate.Weekday()))
	return startDate.Format("2006-01-02"), startDate.AddDate(0, 0, 42).Format("2006-01-02")
}

func attendanceDateAfter(date string) string {
	parsedDate, errorValue := parseAttendanceAbsenceDate(date)
	if errorValue != nil {
		return date
	}
	return parsedDate.AddDate(0, 0, 1).Format("2006-01-02")
}

func attendanceDateBefore(date string) string {
	parsedDate, errorValue := parseAttendanceAbsenceDate(date)
	if errorValue != nil {
		return date
	}
	return parsedDate.AddDate(0, 0, -1).Format("2006-01-02")
}

func attendanceDateRangeOverlapDates(startDate string, endDate string, allowedDates map[string]struct{}) []string {
	dates, errorValue := attendanceAbsenceDates(startDate, endDate)
	if errorValue != nil {
		return []string{}
	}
	overlapDates := []string{}
	for _, date := range dates {
		if _, exists := allowedDates[date]; exists {
			overlapDates = append(overlapDates, date)
		}
	}
	return overlapDates
}

func attendanceDateSet(dates []string) map[string]struct{} {
	result := make(map[string]struct{}, len(dates))
	for _, date := range dates {
		result[date] = struct{}{}
	}
	return result
}

func subtractAttendanceDates(dates []string, removedDates map[string]struct{}) []string {
	result := []string{}
	for _, date := range dates {
		if _, removed := removedDates[date]; removed {
			continue
		}
		result = append(result, date)
	}
	return result
}

func collapseAttendanceDatesToRanges(dates []string) []attendanceAbsenceDateRange {
	if len(dates) == 0 {
		return []attendanceAbsenceDateRange{}
	}
	sortedDates := append([]string{}, dates...)
	sort.Strings(sortedDates)
	ranges := []attendanceAbsenceDateRange{}
	startDate := sortedDates[0]
	previousDate := sortedDates[0]
	for _, date := range sortedDates[1:] {
		if nextAttendanceBusinessDate(previousDate) == date {
			previousDate = date
			continue
		}
		ranges = append(ranges, attendanceAbsenceDateRange{StartDate: startDate, EndDate: previousDate})
		startDate = date
		previousDate = date
	}
	return append(ranges, attendanceAbsenceDateRange{StartDate: startDate, EndDate: previousDate})
}

func nextAttendanceBusinessDate(date string) string {
	parsedDate, errorValue := parseAttendanceAbsenceDate(date)
	if errorValue != nil {
		return date
	}
	for {
		parsedDate = parsedDate.AddDate(0, 0, 1)
		if !isAttendanceAbsenceWeekend(parsedDate) {
			return parsedDate.Format("2006-01-02")
		}
	}
}

func previousAttendanceBusinessDate(date string) string {
	parsedDate, errorValue := parseAttendanceAbsenceDate(date)
	if errorValue != nil {
		return date
	}
	for {
		parsedDate = parsedDate.AddDate(0, 0, -1)
		if !isAttendanceAbsenceWeekend(parsedDate) {
			return parsedDate.Format("2006-01-02")
		}
	}
}

func sortedAttendanceDatesFromSet(dates map[string]struct{}) []string {
	result := make([]string, 0, len(dates))
	for date := range dates {
		result = append(result, date)
	}
	sort.Strings(result)
	return result
}

func expandAttendanceAbsenceRanges(absenceRanges []attendanceAbsenceRange, startDate string, endDate string) []attendanceAbsence {
	occurrences := []attendanceAbsence{}
	for _, absenceRange := range absenceRanges {
		occurrences = append(occurrences, expandAttendanceAbsenceRange(absenceRange, startDate, endDate)...)
	}
	sort.SliceStable(occurrences, func(leftIndex int, rightIndex int) bool {
		left := occurrences[leftIndex]
		right := occurrences[rightIndex]
		if left.Date != right.Date {
			return left.Date < right.Date
		}
		if left.Email != right.Email {
			return left.Email < right.Email
		}
		return left.CreatedAt < right.CreatedAt
	})
	return occurrences
}

func expandAttendanceAbsenceRange(absenceRange attendanceAbsenceRange, startDate string, endDate string) []attendanceAbsence {
	rangeStartDate := maxAttendanceDate(absenceRange.StartDate, startDate)
	rangeEndDate := minAttendanceDate(absenceRange.EndDate, attendanceDateBefore(endDate))
	dates, errorValue := attendanceAbsenceDates(rangeStartDate, rangeEndDate)
	if errorValue != nil {
		return []attendanceAbsence{}
	}
	allDates, errorValue := attendanceAbsenceDates(absenceRange.StartDate, absenceRange.EndDate)
	if errorValue != nil {
		return []attendanceAbsence{}
	}
	allDateSet := attendanceDateSet(allDates)
	occurrences := make([]attendanceAbsence, 0, len(dates))
	for index, date := range dates {
		_, hasPreviousCalendarDate := allDateSet[attendanceDateBefore(date)]
		_, hasNextCalendarDate := allDateSet[attendanceDateAfter(date)]
		occurrences = append(occurrences, attendanceAbsence{
			ID:           joinAttendanceAbsenceOccurrenceID(absenceRange.ID, date),
			RangeID:      absenceRange.ID,
			Email:        absenceRange.Email,
			Kind:         absenceRange.Kind,
			LabelKey:     absenceRange.Kind,
			Date:         date,
			StartDate:    absenceRange.StartDate,
			EndDate:      absenceRange.EndDate,
			Reason:       absenceRange.Reason,
			CreatedBy:    absenceRange.CreatedBy,
			CreatedAt:    absenceRange.CreatedAt,
			CanceledAt:   absenceRange.CanceledAt,
			IsRangeStart: index == 0 && date == firstAttendanceBusinessDate(allDates),
			IsRangeEnd:   index == len(dates)-1 && date == lastAttendanceBusinessDate(allDates),
			IsChunkStart: !hasPreviousCalendarDate,
			IsChunkEnd:   !hasNextCalendarDate,
		})
	}
	return occurrences
}

func firstAttendanceBusinessDate(dates []string) string {
	if len(dates) == 0 {
		return ""
	}
	return dates[0]
}

func lastAttendanceBusinessDate(dates []string) string {
	if len(dates) == 0 {
		return ""
	}
	return dates[len(dates)-1]
}

func maxAttendanceDate(left string, right string) string {
	if left > right {
		return left
	}
	return right
}

func minAttendanceDate(left string, right string) string {
	if left < right {
		return left
	}
	return right
}

func joinAttendanceAbsenceOccurrenceID(rangeID string, date string) string {
	return rangeID + attendanceAbsenceOccurrenceIDSeparator + date
}

func splitAttendanceAbsenceOccurrenceID(absenceID string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(absenceID), attendanceAbsenceOccurrenceIDSeparator, 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}
