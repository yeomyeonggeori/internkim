package admind

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestClockingInSurvivesAPlaneThatIsNotThere(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	forgetCentralPlaneForTest()
	service.Configuration.CentralPlaneAppURL = "http://127.0.0.1:1"
	service.Configuration.CentralPlaneProjectURL = "http://127.0.0.1:1"
	service.Configuration.CentralPlanePublishableKey = "publishable"

	ctx := context.Background()
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	event := attendanceEvent{
		ID:               "event-1",
		MattermostUserID: "U1",
		Email:            "someone@example.test",
		Kind:             attendanceKindClockIn,
		OccurredAt:       time.Now().UTC().Format(time.RFC3339),
		LocalDate:        "2026-08-04",
		LocalTime:        "09:00",
		TimeZoneAtEvent:  "Asia/Seoul",
		Source:           "mattermost",
		LocationName:     "사무실",
	}

	if errorValue := service.insertAttendanceEvent(ctx, database, event); errorValue != nil {
		t.Fatalf("a clock-in must not depend on the plane: %v", errorValue)
	}

	var recorded int
	if errorValue := database.QueryRow("SELECT COUNT(*) FROM attendance_events WHERE id = ?", event.ID).Scan(&recorded); errorValue != nil {
		t.Fatalf("reading the record back failed: %v", errorValue)
	}
	if recorded != 1 {
		t.Fatalf("the device stays the record, found %d rows", recorded)
	}
}

func TestNothingIsSentWhenNoPlaneIsConfigured(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	forgetCentralPlaneForTest()
	if service.centralPlane() != nil {
		t.Fatal("without settings there is nowhere to send anything")
	}
}

func forgetCentralPlaneForTest() {
	centralPlaneOnce = sync.Once{}
	centralPlaneClient = nil
}
