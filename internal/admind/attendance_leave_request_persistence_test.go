package admind

import (
	"net/http"
	"testing"
	"time"
)

func TestAttendanceLeaveRequestPersistsAcrossRestartAndScopesDashboardToOwner(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"sick","unit":"quarterDay","startDate":"2027-05-03","partialPeriod":"morning","reason":"Medical appointment"}`,
	)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d body = %s", recorder.Code, recorder.Body.String())
	}

	restartedService := NewService(service.Configuration)
	ownerDashboard, errorValue := restartedService.readAttendanceLeaveDashboard(
		t.Context(),
		attendanceLeaveEmployee{Email: "staff@example.com"},
		time.Now(),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(ownerDashboard.Requests) != 1 ||
		ownerDashboard.Requests[0].Reason != "Medical appointment" {
		t.Fatalf("owner requests = %+v", ownerDashboard.Requests)
	}
	otherDashboard, errorValue := restartedService.readAttendanceLeaveDashboard(
		t.Context(),
		attendanceLeaveEmployee{Email: "other@example.com"},
		time.Now(),
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(otherDashboard.Requests) != 0 {
		t.Fatalf("other requests = %+v", otherDashboard.Requests)
	}
}
