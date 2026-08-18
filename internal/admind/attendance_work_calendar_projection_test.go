package admind

import (
	"testing"
	"time"
)

func TestAttendanceWorkCalendarProjectionUsesHolidaysAndCurrentWorkMode(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if _, errorValue := service.createCalendarCompanyHoliday(
		t.Context(),
		calendarCompanyHolidayInput{Title: "company closure", Date: "2027-01-05"},
		time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	revision := defaultAttendanceWorkPolicyRevision()
	revision.WorkMode = attendanceWorkModeFixed
	revision.FixedStartTime = "09:00"
	revision.FixedEndTime = "18:00"
	revision.CoreTimeEnabled = false
	revision.CoreStartTime = ""
	revision.CoreEndTime = ""
	if _, errorValue := service.saveAttendanceWorkPolicyRevision(
		t.Context(),
		revision,
		"2027-01-04",
		time.Date(2026, time.December, 1, 0, 0, 0, 0, time.UTC),
	); errorValue != nil {
		t.Fatal(errorValue)
	}

	projection, errorValue := service.attendanceWorkCalendarProjection(
		t.Context(),
		time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2027, time.January, 6, 0, 0, 0, 0, time.UTC),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	want := []attendanceWorkCalendarDay{
		{Date: "2027-01-01", WorkMode: attendanceWorkModeFixed, WorkingDate: false, Holiday: true},
		{Date: "2027-01-02", WorkMode: attendanceWorkModeFixed, WorkingDate: false, Holiday: false},
		{Date: "2027-01-03", WorkMode: attendanceWorkModeFixed, WorkingDate: false, Holiday: false},
		{Date: "2027-01-04", WorkMode: attendanceWorkModeFixed, WorkingDate: true, Holiday: false},
		{Date: "2027-01-05", WorkMode: attendanceWorkModeFixed, WorkingDate: false, Holiday: true},
	}
	if !equalAttendanceWorkCalendarDays(projection, want) {
		t.Fatalf("projection = %#v, want %#v", projection, want)
	}
}

func equalAttendanceWorkCalendarDays(got []attendanceWorkCalendarDay, want []attendanceWorkCalendarDay) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
