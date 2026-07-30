package admind

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAttendanceLeaveRequestPreviewExcludesWeekends(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)

	request := httptest.NewRequest(http.MethodPost, "/attendance/api/leave-requests/preview", strings.NewReader(`{
		"leaveTypeID": "annual",
		"unit": "fullDay",
		"startDate": "2027-05-07",
		"endDate": "2027-05-10"
	}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	recorder := httptest.NewRecorder()

	service.handleAttendance(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Occurrences []struct {
			Date               string `json:"date"`
			StartTime          string `json:"startTime"`
			EndTime            string `json:"endTime"`
			DeductionMilliDays int    `json:"deductionMilliDays"`
		} `json:"occurrences"`
		ExcludedDates []struct {
			Date   string `json:"date"`
			Reason string `json:"reason"`
		} `json:"excludedDates"`
		TotalDeductionMilliDays int `json:"totalDeductionMilliDays"`
	}
	if errorValue := json.NewDecoder(recorder.Body).Decode(&response); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(response.Occurrences) != 2 ||
		response.Occurrences[0].Date != "2027-05-07" ||
		response.Occurrences[0].StartTime != "09:00" ||
		response.Occurrences[0].EndTime != "18:00" ||
		response.Occurrences[0].DeductionMilliDays != 1000 ||
		response.Occurrences[1].Date != "2027-05-10" ||
		response.TotalDeductionMilliDays != 2000 {
		t.Fatalf("preview occurrences = %+v total = %d", response.Occurrences, response.TotalDeductionMilliDays)
	}
	if len(response.ExcludedDates) != 2 ||
		response.ExcludedDates[0].Date != "2027-05-08" ||
		response.ExcludedDates[0].Reason != "nonWorkingDay" ||
		response.ExcludedDates[1].Date != "2027-05-09" ||
		response.ExcludedDates[1].Reason != "nonWorkingDay" {
		t.Fatalf("excluded dates = %+v", response.ExcludedDates)
	}
}

func TestAttendanceLeaveRequestPreviewRejectsPastDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	request := httptest.NewRequest(
		http.MethodPost,
		"/attendance/api/leave-requests/preview",
		strings.NewReader(`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2020-01-02"}`),
	)
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("X-Forwarded-Email", "staff@example.com")
	recorder := httptest.NewRecorder()

	service.handleAttendance(recorder, request)

	response := assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusBadRequest,
		attendanceLeaveErrorInvalidInput,
	)
	if !strings.Contains(response.Error, "past") {
		t.Fatalf("error = %q", response.Error)
	}
}

func TestAttendanceLeaveRequestPreviewAllowsTodayRegardlessOfStartTime(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	location := service.workspaceTimeZone().location
	now := time.Date(2027, 5, 3, 13, 30, 0, 0, location)

	inputs := []attendanceLeaveRequestInput{
		{LeaveTypeID: "sick", Unit: attendanceWorkScheduleFullDay, StartDate: "2027-05-03"},
		{
			LeaveTypeID:   "sick",
			Unit:          attendanceWorkScheduleHalfDay,
			StartDate:     "2027-05-03",
			PartialPeriod: attendanceLeavePartialPeriodMorning,
		},
		{
			LeaveTypeID:   "sick",
			Unit:          attendanceWorkScheduleQuarterDay,
			StartDate:     "2027-05-03",
			PartialPeriod: attendanceLeavePartialPeriodCustom,
			StartTime:     "13:00",
		},
	}
	for _, input := range inputs {
		if _, errorValue := service.previewAttendanceLeaveRequest(t.Context(), input, now); errorValue != nil {
			t.Fatalf("today input %+v: %v", input, errorValue)
		}
	}
}

func TestAttendanceLeaveRequestDateRangeUsesCalendarDayLimit(t *testing.T) {
	location, errorValue := time.LoadLocation("Pacific/Apia")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	now := time.Date(2011, 1, 1, 12, 0, 0, 0, location)
	input := attendanceLeaveRequestInput{
		Unit:      attendanceWorkScheduleFullDay,
		StartDate: "2011-01-01",
		EndDate:   "2012-01-02",
	}

	_, _, errorValue = attendanceLeaveRequestDateRange(input, now)

	if !errors.Is(errorValue, errAttendanceLeaveInvalidInput) {
		t.Fatalf("error = %v", errorValue)
	}
}
