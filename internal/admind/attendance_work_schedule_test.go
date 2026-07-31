package admind

import (
	"strings"
	"testing"
	"time"
)

func TestAttendanceWorkScheduleDefaults(t *testing.T) {
	schedule := defaultAttendanceWorkSchedule()

	if schedule.Version != attendanceWorkScheduleVersion ||
		schedule.WorkMode != attendanceWorkModeFlexible ||
		schedule.ScheduledWorkMinutes != 480 ||
		schedule.ReferenceStartTime != "09:00" ||
		schedule.FixedStartTime != "" ||
		schedule.FixedEndTime != "" ||
		len(schedule.BreakPeriods) != 1 ||
		schedule.BreakPeriods[0].StartTime != "12:00" ||
		schedule.BreakPeriods[0].EndTime != "13:00" {
		t.Fatalf("defaults = %+v", schedule)
	}
	if expectedWeekdays := []int{1, 2, 3, 4, 5}; !equalIntSlices(schedule.WorkingWeekdays, expectedWeekdays) {
		t.Fatalf("working weekdays = %v", schedule.WorkingWeekdays)
	}
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&schedule); errorValue != nil {
		t.Fatalf("validate defaults: %v", errorValue)
	}
}

func TestAttendanceWorkScheduleNormalizesOrderedValues(t *testing.T) {
	schedule := defaultAttendanceWorkSchedule()
	schedule.WorkMode = " flexible "
	schedule.ReferenceStartTime = " 09:00 "
	schedule.FixedStartTime = "invalid"
	schedule.FixedEndTime = "invalid"
	schedule.WorkingWeekdays = []int{5, 1, 3}
	schedule.BreakPeriods = []attendanceWorkScheduleBreakPeriod{
		{StartTime: " 15:00 ", EndTime: "15:30"},
		{StartTime: "12:00", EndTime: "13:00"},
	}
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&schedule); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !equalIntSlices(schedule.WorkingWeekdays, []int{1, 3, 5}) {
		t.Fatalf("working weekdays = %v", schedule.WorkingWeekdays)
	}
	if schedule.BreakPeriods[0].StartTime != "12:00" ||
		schedule.BreakPeriods[1].StartTime != "15:00" ||
		schedule.FixedStartTime != "" ||
		schedule.FixedEndTime != "" {
		t.Fatalf("normalized schedule = %+v", schedule)
	}
}

func TestAttendanceWorkScheduleRejectsInvalidContracts(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*attendanceWorkSchedule)
	}{
		{name: "version", mutate: func(schedule *attendanceWorkSchedule) { schedule.Version = 2 }},
		{name: "work mode", mutate: func(schedule *attendanceWorkSchedule) { schedule.WorkMode = "hybrid" }},
		{name: "empty weekdays", mutate: func(schedule *attendanceWorkSchedule) { schedule.WorkingWeekdays = nil }},
		{name: "weekday range", mutate: func(schedule *attendanceWorkSchedule) { schedule.WorkingWeekdays = []int{1, 8} }},
		{name: "duplicate weekday", mutate: func(schedule *attendanceWorkSchedule) { schedule.WorkingWeekdays = []int{1, 1} }},
		{name: "zero minutes", mutate: func(schedule *attendanceWorkSchedule) { schedule.ScheduledWorkMinutes = 0 }},
		{name: "day limit", mutate: func(schedule *attendanceWorkSchedule) {
			schedule.ScheduledWorkMinutes = attendanceWorkScheduleMinutesPerDay
		}},
		{name: "reference format", mutate: func(schedule *attendanceWorkSchedule) { schedule.ReferenceStartTime = "9:00" }},
		{name: "reference digits", mutate: func(schedule *attendanceWorkSchedule) { schedule.ReferenceStartTime = "+1:00" }},
		{name: "break range", mutate: func(schedule *attendanceWorkSchedule) {
			schedule.BreakPeriods = []attendanceWorkScheduleBreakPeriod{{StartTime: "13:00", EndTime: "12:00"}}
		}},
		{name: "break overlap", mutate: func(schedule *attendanceWorkSchedule) {
			schedule.BreakPeriods = []attendanceWorkScheduleBreakPeriod{
				{StartTime: "12:00", EndTime: "13:00"},
				{StartTime: "12:30", EndTime: "14:00"},
			}
		}},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			schedule := defaultAttendanceWorkSchedule()
			testCase.mutate(&schedule)
			if validateAndNormalizeAttendanceWorkSchedule(&schedule) == nil {
				t.Fatal("expected validation failure")
			}
		})
	}
}

func TestAttendanceWorkScheduleRoundsPartialUnitsUpToMinutePrecision(t *testing.T) {
	schedule := defaultAttendanceWorkSchedule()
	schedule.BreakPeriods = nil
	schedule.ScheduledWorkMinutes = 481
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&schedule); errorValue != nil {
		t.Fatal(errorValue)
	}
	if schedule.BreakPeriods == nil {
		t.Fatal("breakPeriods must normalize to an empty array")
	}
	if minutes, errorValue := attendanceWorkScheduleMinutesForUnit(schedule, attendanceWorkScheduleFullDay); errorValue != nil || minutes != 481 {
		t.Fatalf("full-day minutes = %d error = %v", minutes, errorValue)
	}
	if minutes, errorValue := attendanceWorkScheduleMinutesForUnit(schedule, attendanceWorkScheduleHalfDay); errorValue != nil || minutes != 241 {
		t.Fatalf("half-day minutes = %d error = %v", minutes, errorValue)
	}
	if minutes, errorValue := attendanceWorkScheduleMinutesForUnit(schedule, attendanceWorkScheduleQuarterDay); errorValue != nil || minutes != 121 {
		t.Fatalf("quarter-day minutes = %d error = %v", minutes, errorValue)
	}

	schedule.ScheduledWorkMinutes = 482
	if minutes, errorValue := attendanceWorkScheduleMinutesForUnit(schedule, attendanceWorkScheduleHalfDay); errorValue != nil || minutes != 241 {
		t.Fatalf("half-day minutes = %d error = %v", minutes, errorValue)
	}
	if minutes, errorValue := attendanceWorkScheduleMinutesForUnit(schedule, attendanceWorkScheduleQuarterDay); errorValue != nil || minutes != 121 {
		t.Fatalf("quarter-day minutes = %d error = %v", minutes, errorValue)
	}

	schedule.ScheduledWorkMinutes = 450
	if minutes, errorValue := attendanceWorkScheduleMinutesForUnit(schedule, attendanceWorkScheduleQuarterDay); errorValue != nil || minutes != 113 {
		t.Fatalf("quarter-day minutes = %d error = %v", minutes, errorValue)
	}
	if deduction, errorValue := attendanceLeaveRequestDeductionMilliDays(attendanceWorkScheduleHalfDay); errorValue != nil || deduction != 500 {
		t.Fatalf("half-day deduction = %d error = %v", deduction, errorValue)
	}
	if deduction, errorValue := attendanceLeaveRequestDeductionMilliDays(attendanceWorkScheduleQuarterDay); errorValue != nil || deduction != 250 {
		t.Fatalf("quarter-day deduction = %d error = %v", deduction, errorValue)
	}
}

func TestAttendanceWorkScheduleAllowsMaximumScheduledMinutes(t *testing.T) {
	schedule := defaultAttendanceWorkSchedule()
	schedule.ReferenceStartTime = "00:00"
	schedule.ScheduledWorkMinutes = attendanceWorkScheduleMaximumMinutes
	schedule.BreakPeriods = nil

	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&schedule); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestAttendanceWorkScheduleValidatesFixedAndFlexibleModes(t *testing.T) {
	fixedSchedule := defaultAttendanceWorkSchedule()
	fixedSchedule.WorkMode = attendanceWorkModeFixed
	fixedSchedule.ReferenceStartTime = "invalid"
	fixedSchedule.FixedStartTime = "09:00"
	fixedSchedule.FixedEndTime = "18:00"
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&fixedSchedule); errorValue != nil {
		t.Fatalf("valid fixed schedule: %v", errorValue)
	}
	if fixedSchedule.ReferenceStartTime != "09:00" ||
		attendanceWorkSchedulePartialLeaveStartTime(fixedSchedule) != "09:00" {
		t.Fatalf("fixed schedule = %+v", fixedSchedule)
	}

	invalidFixedSchedule := fixedSchedule
	invalidFixedSchedule.FixedEndTime = "17:00"
	if validateAndNormalizeAttendanceWorkSchedule(&invalidFixedSchedule) == nil {
		t.Fatal("expected fixed duration mismatch")
	}

	fixedScheduleWithoutBreak := fixedSchedule
	fixedScheduleWithoutBreak.BreakPeriods = nil
	fixedScheduleWithoutBreak.FixedEndTime = "17:00"
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&fixedScheduleWithoutBreak); errorValue != nil {
		t.Fatalf("fixed schedule without break: %v", errorValue)
	}

	flexibleSchedule := defaultAttendanceWorkSchedule()
	flexibleSchedule.FixedStartTime = "08:00"
	flexibleSchedule.FixedEndTime = "17:00"
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&flexibleSchedule); errorValue != nil {
		t.Fatalf("valid flexible schedule: %v", errorValue)
	}
	if flexibleSchedule.FixedStartTime != "" ||
		flexibleSchedule.FixedEndTime != "" ||
		attendanceWorkSchedulePartialLeaveStartTime(flexibleSchedule) != "09:00" {
		t.Fatalf("flexible schedule = %+v", flexibleSchedule)
	}

	overflowingFlexibleSchedule := defaultAttendanceWorkSchedule()
	overflowingFlexibleSchedule.ReferenceStartTime = "16:00"
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&overflowingFlexibleSchedule); errorValue == nil ||
		!strings.Contains(errorValue.Error(), "day boundary") {
		t.Fatalf("flexible day boundary error = %v", errorValue)
	}
}

func TestAttendanceWorkScheduleWorkingDatesExcludeWeekendsAndHolidays(t *testing.T) {
	schedule := defaultAttendanceWorkSchedule()
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&schedule); errorValue != nil {
		t.Fatal(errorValue)
	}
	holidayDates := map[string]struct{}{"2026-07-27": {}}

	tests := []struct {
		date     string
		expected bool
	}{
		{date: "2026-07-27", expected: false},
		{date: "2026-07-28", expected: true},
		{date: "2026-08-01", expected: false},
	}
	for _, testCase := range tests {
		actual, errorValue := attendanceWorkScheduleIsWorkingDate(schedule, testCase.date, holidayDates)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if actual != testCase.expected {
			t.Fatalf("%s working = %t", testCase.date, actual)
		}
	}
	if _, errorValue := attendanceWorkScheduleIsWorkingDate(schedule, "2026-02-29", holidayDates); errorValue == nil {
		t.Fatal("expected invalid date failure")
	}

	schedule.WorkingWeekdays = []int{6, 7}
	if errorValue := validateAndNormalizeAttendanceWorkSchedule(&schedule); errorValue != nil {
		t.Fatal(errorValue)
	}
	saturdayIsWorking, errorValue := attendanceWorkScheduleIsWorkingDate(schedule, "2026-08-01", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	tuesdayIsWorking, errorValue := attendanceWorkScheduleIsWorkingDate(schedule, "2026-07-28", nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !saturdayIsWorking || tuesdayIsWorking {
		t.Fatalf("custom weekdays: Saturday = %t Tuesday = %t", saturdayIsWorking, tuesdayIsWorking)
	}
}

func TestAttendanceWorkScheduleWorkingInstantUsesCompanyTimeZone(t *testing.T) {
	schedule := defaultAttendanceWorkSchedule()
	seoul, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	instant := time.Date(2026, time.July, 26, 15, 30, 0, 0, time.UTC)

	isWorkingInSeoul, errorValue := attendanceWorkScheduleIsWorkingInstant(schedule, instant, seoul, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	isWorkingInUTC, errorValue := attendanceWorkScheduleIsWorkingInstant(schedule, instant, time.UTC, nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !isWorkingInSeoul || isWorkingInUTC {
		t.Fatalf("working in Seoul = %t, UTC = %t", isWorkingInSeoul, isWorkingInUTC)
	}
}

func TestAttendanceWorkScheduleCalculatesPartialLeaveAcrossBreaks(t *testing.T) {
	schedule := defaultAttendanceWorkSchedule()
	tests := []struct {
		name      string
		startTime string
		unit      string
		expected  string
	}{
		{name: "full day", startTime: "09:00", unit: attendanceWorkScheduleFullDay, expected: "18:00"},
		{name: "half day", startTime: "09:00", unit: attendanceWorkScheduleHalfDay, expected: "14:00"},
		{name: "quarter day", startTime: "09:00", unit: attendanceWorkScheduleQuarterDay, expected: "11:00"},
		{name: "quarter overlaps break", startTime: "11:30", unit: attendanceWorkScheduleQuarterDay, expected: "14:30"},
		{name: "starts inside break", startTime: "12:30", unit: attendanceWorkScheduleQuarterDay, expected: "15:00"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			endTime, errorValue := calculateAttendanceWorkScheduleEndTime(
				schedule,
				"2026-07-28",
				testCase.startTime,
				testCase.unit,
				time.UTC,
			)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if actual := endTime.Format("15:04"); actual != testCase.expected {
				t.Fatalf("end time = %s", actual)
			}
		})
	}
}

func TestAttendanceWorkScheduleCalculatesAcrossAdjacentBreaks(t *testing.T) {
	schedule := defaultAttendanceWorkSchedule()
	schedule.BreakPeriods = []attendanceWorkScheduleBreakPeriod{
		{StartTime: "12:00", EndTime: "13:00"},
		{StartTime: "13:00", EndTime: "14:00"},
	}

	endTime, errorValue := calculateAttendanceWorkScheduleEndTime(
		schedule,
		"2026-07-28",
		"12:30",
		attendanceWorkScheduleQuarterDay,
		time.UTC,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if actual := endTime.Format("15:04"); actual != "16:00" {
		t.Fatalf("end time = %s", actual)
	}
}

func TestAttendanceWorkScheduleRejectsDayBoundaryAndMissingWallTime(t *testing.T) {
	schedule := defaultAttendanceWorkSchedule()
	if _, errorValue := calculateAttendanceWorkScheduleEndTime(
		schedule,
		"2026-07-28",
		"20:00",
		attendanceWorkScheduleFullDay,
		time.UTC,
	); errorValue == nil || !strings.Contains(errorValue.Error(), "day boundary") {
		t.Fatalf("day boundary error = %v", errorValue)
	}

	newYork, errorValue := time.LoadLocation("America/New_York")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue = calculateAttendanceWorkScheduleEndTime(
		schedule,
		"2026-03-08",
		"02:30",
		attendanceWorkScheduleQuarterDay,
		newYork,
	); errorValue == nil || !strings.Contains(errorValue.Error(), "company time zone") {
		t.Fatalf("missing wall time error = %v", errorValue)
	}
	if _, errorValue = calculateAttendanceWorkScheduleEndTime(
		schedule,
		"2026-11-01",
		"01:30",
		attendanceWorkScheduleQuarterDay,
		newYork,
	); errorValue == nil || !strings.Contains(errorValue.Error(), "ambiguous") {
		t.Fatalf("ambiguous wall time error = %v", errorValue)
	}
}

func equalIntSlices(left []int, right []int) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
