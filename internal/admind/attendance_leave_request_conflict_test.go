package admind

import (
	"net/http"
	"testing"
	"time"
)

func TestAttendanceLeaveRequestRejectsExistingLeaveOverlap(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	firstRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"sick","unit":"halfDay","startDate":"2027-05-03","partialPeriod":"morning","reason":"Medical appointment"}`,
	)
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first status = %d body = %s", firstRecorder.Code, firstRecorder.Body.String())
	}

	secondRecorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"sick","unit":"quarterDay","startDate":"2027-05-03","partialPeriod":"morning","reason":"Follow-up appointment"}`,
	)
	assertAttendanceLeaveErrorResponse(
		t,
		secondRecorder,
		http.StatusConflict,
		attendanceLeaveErrorLeaveConflict,
	)
}

func TestAttendanceLeaveRequestOpenClockInConflictsOnlyThroughCurrentTime(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	location := service.workspaceTimeZone().location
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{
		ID:       "user-1",
		Username: "staff",
		Email:    "staff@example.com",
		Nickname: "Staff",
	}
	clockIn := service.createAttendanceEvent(
		userRecord,
		attendanceKindClockIn,
		time.Date(2027, 5, 3, 10, 0, 0, 0, location),
		"team-1",
		"attendance-channel",
		"entry-post",
		"open-clock-in-post",
		service.attendanceLocationByID("office"),
	)
	if errorValue := service.insertAttendanceEvent(t.Context(), database, clockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()
	mutationDatabase, errorValue := service.openAttendanceLeaveMutationDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer mutationDatabase.Close()
	transaction, errorValue := mutationDatabase.BeginTx(t.Context(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer transaction.Rollback()

	conflict, errorValue := attendanceLeaveRequestHasConfirmedWorkConflict(
		t.Context(),
		transaction,
		"staff@example.com",
		attendanceLeaveRequestOccurrence{
			Date:      "2027-05-03",
			StartTime: "10:30",
			EndTime:   "12:00",
		},
		time.Date(2027, 5, 3, 11, 0, 0, 0, location),
		location,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !conflict {
		t.Fatal("expected open clock-in overlap through current time")
	}
	futureConflict, errorValue := attendanceLeaveRequestHasConfirmedWorkConflict(
		t.Context(),
		transaction,
		"staff@example.com",
		attendanceLeaveRequestOccurrence{
			Date:      "2027-05-03",
			StartTime: "11:30",
			EndTime:   "12:00",
		},
		time.Date(2027, 5, 3, 11, 0, 0, 0, location),
		location,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if futureConflict {
		t.Fatal("open clock-in must not extend beyond the current time")
	}
}

func TestAttendanceLeaveRequestDetectsClosedOvernightWorkByInstant(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	location := service.workspaceTimeZone().location
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	userRecord := mattermostUserRecord{
		ID:       "user-1",
		Username: "staff",
		Email:    "staff@example.com",
		Nickname: "Staff",
	}
	events := []attendanceEvent{
		service.createAttendanceEvent(
			userRecord,
			attendanceKindClockOut,
			time.Date(2027, 7, 1, 4, 0, 0, 0, location),
			"team-1",
			"attendance-channel",
			"entry-post",
			"overnight-clock-out-post",
			service.attendanceLocationByID("office"),
		),
		service.createAttendanceEvent(
			userRecord,
			attendanceKindClockIn,
			time.Date(2027, 6, 30, 21, 0, 0, 0, location),
			"team-1",
			"attendance-channel",
			"entry-post",
			"overnight-clock-in-post",
			service.attendanceLocationByID("office"),
		),
	}
	for _, event := range events {
		if errorValue := service.insertAttendanceEvent(t.Context(), database, event); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	database.Close()
	mutationDatabase, errorValue := service.openAttendanceLeaveMutationDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer mutationDatabase.Close()
	transaction, errorValue := mutationDatabase.BeginTx(t.Context(), nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer transaction.Rollback()

	conflict, errorValue := attendanceLeaveRequestHasConfirmedWorkConflict(
		t.Context(),
		transaction,
		"staff@example.com",
		attendanceLeaveRequestOccurrence{
			Date:      "2027-07-01",
			StartTime: "03:00",
			EndTime:   "05:00",
		},
		time.Date(2027, 7, 1, 12, 0, 0, 0, location),
		location,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !conflict {
		t.Fatal("expected overnight work to overlap the next-day leave occurrence")
	}
	nonConflict, errorValue := attendanceLeaveRequestHasConfirmedWorkConflict(
		t.Context(),
		transaction,
		"staff@example.com",
		attendanceLeaveRequestOccurrence{
			Date:      "2027-07-01",
			StartTime: "04:30",
			EndTime:   "05:00",
		},
		time.Date(2027, 7, 1, 12, 0, 0, 0, location),
		location,
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if nonConflict {
		t.Fatal("closed overnight work must not extend beyond clock-out")
	}
}

func TestAttendanceLeaveRequestRejectsConfirmedWorkOverlap(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	database, errorValue := service.openAttendanceDatabase(t.Context())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	location := service.workspaceTimeZone().location
	userRecord := mattermostUserRecord{
		ID:       "user-1",
		Username: "staff",
		Email:    "staff@example.com",
		Nickname: "Staff",
	}
	clockIn := service.createAttendanceEvent(
		userRecord,
		attendanceKindClockIn,
		time.Date(2027, 5, 3, 10, 0, 0, 0, location),
		"team-1",
		"attendance-channel",
		"entry-post",
		"clock-in-post",
		service.attendanceLocationByID("office"),
	)
	clockOut := service.createAttendanceEvent(
		userRecord,
		attendanceKindClockOut,
		time.Date(2027, 5, 3, 11, 0, 0, 0, location),
		"team-1",
		"attendance-channel",
		"entry-post",
		"clock-out-post",
		service.attendanceLocationByID("office"),
	)
	if errorValue := service.insertAttendanceEvent(t.Context(), database, clockIn); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.insertAttendanceEvent(t.Context(), database, clockOut); errorValue != nil {
		t.Fatal(errorValue)
	}
	database.Close()

	recorder := performAttendanceLeaveMultipartRequest(
		t,
		service,
		http.MethodPost,
		"/attendance/api/leave-requests",
		"staff@example.com",
		`{"leaveTypeID":"sick","unit":"halfDay","startDate":"2027-05-03","partialPeriod":"custom","startTime":"09:00","reason":"Medical appointment"}`,
	)
	assertAttendanceLeaveErrorResponse(
		t,
		recorder,
		http.StatusConflict,
		attendanceLeaveErrorWorkConflict,
	)
}
