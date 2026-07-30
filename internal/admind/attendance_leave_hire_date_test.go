package admind

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestAttendanceLeaveDashboardMarksMissingHireDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	dashboard, errorValue := service.readAttendanceLeaveDashboard(
		t.Context(),
		attendanceLeaveEmployee{Email: "missing-hire-date@example.com"},
		time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !dashboard.HireDateRequired {
		t.Fatal("dashboard did not require a hire date")
	}
	for _, leaveType := range dashboard.LeaveTypes {
		if leaveType.ID == "annual" && leaveType.RequiresHireDate {
			return
		}
	}
	t.Fatalf("annual leave type = %+v", dashboard.LeaveTypes)
}

func TestAttendanceLeaveRequestRejectsAccruedTypeWithoutHireDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"other@example.com",
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusConflict,
		attendanceLeaveErrorHireDateRequired,
	)
}

func TestAttendanceLeaveRequestAllowsUntrackedTypeWithoutHireDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"other@example.com",
		`{"leaveTypeID":"sick","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestAttendanceLeaveProfileReadFailureDoesNotLookLikeMissingHireDate(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	organizationDatabasePath := service.organizationDatabasePath()
	if errorValue := os.Remove(organizationDatabasePath); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.Mkdir(organizationDatabasePath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}

	memberRequest := httptest.NewRequest(http.MethodGet, "/attendance/api/summary", nil)
	records, found := service.attendanceUserRecordsForMembers(memberRequest)
	if !found || len(records) == 0 {
		t.Fatalf("attendance records = %+v found = %t", records, found)
	}

	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"annual","unit":"fullDay","startDate":"2027-05-03"}`,
	)
	assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusInternalServerError,
		attendanceLeaveErrorInternal,
	)
}
