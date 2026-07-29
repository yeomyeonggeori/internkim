package admind

import (
	"context"
	"testing"
)

func TestAttendanceLeaveRequestSchemaCreatesPrivateRequestTables(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()

	for _, tableName := range []string{
		"attendance_leave_requests",
		"attendance_leave_request_occurrences",
		"attendance_leave_request_events",
		"attendance_leave_request_attachments",
		"attendance_leave_request_absence_ranges",
	} {
		if !attendanceTableExists(t, database, tableName) {
			t.Fatalf("missing table %s", tableName)
		}
	}
}
