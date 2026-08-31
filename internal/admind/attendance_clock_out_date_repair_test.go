package admind

import (
	"context"
	"testing"
	"time"
)

func TestRepairAttendanceClockOutDatesCorrectsLegacyRowsAndPreservesOverrides(t *testing.T) {
	service, _ := newAttendanceActionTestService(t)
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{TimeZone: "Asia/Seoul"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	location, errorValue := time.LoadLocation("Asia/Seoul")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	legacyEvent := legacyAttendanceClockOutEvent(service, "legacy-clock-out", "member-1", "member-1@example.com", location)
	overriddenEvent := legacyAttendanceClockOutEvent(service, "overridden-clock-out", "member-2", "member-2@example.com", location)
	if errorValue := service.insertAttendanceEvent(context.Background(), database, legacyEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.insertAttendanceEvent(context.Background(), database, overriddenEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.insertAttendanceEventOverride(context.Background(), database, attendanceEventOverride{
		ID:                   "manual-override",
		EventID:              overriddenEvent.ID,
		EditedBy:             overriddenEvent.Email,
		EditedAt:             "2026-06-02T03:00:00+09:00",
		Reason:               "Manual correction",
		OriginalOccurredAt:   overriddenEvent.OccurredAt,
		OriginalLocalDate:    overriddenEvent.LocalDate,
		OriginalLocalTime:    overriddenEvent.LocalTime,
		OriginalLocationID:   overriddenEvent.LocationID,
		OriginalLocationName: overriddenEvent.LocationName,
		OverrideOccurredAt:   overriddenEvent.OccurredAt,
		OverrideLocalDate:    overriddenEvent.LocalDate,
		OverrideLocalTime:    overriddenEvent.LocalTime,
		OverrideLocationID:   overriddenEvent.LocationID,
		OverrideLocationName: overriddenEvent.LocationName,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	repairedCount, errorValue := service.repairAttendanceClockOutDates(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if repairedCount != 1 {
		t.Fatalf("repaired count = %d, want 1", repairedCount)
	}
	database, errorValue = service.openAttendanceDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	repairedEvent, _, errorValue := service.readAttendanceEventByID(context.Background(), database, legacyEvent.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	preservedEvent, _, errorValue := service.readAttendanceEventByID(context.Background(), database, overriddenEvent.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if repairedEvent.LocalDate != "2026-06-02" {
		t.Fatalf("repaired local date = %q", repairedEvent.LocalDate)
	}
	if preservedEvent.LocalDate != "2026-06-01" {
		t.Fatalf("overridden base local date = %q", preservedEvent.LocalDate)
	}
	secondRepairCount, errorValue := service.repairAttendanceClockOutDates(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if secondRepairCount != 0 {
		t.Fatalf("second repaired count = %d, want 0", secondRepairCount)
	}
}

func legacyAttendanceClockOutEvent(service *Service, eventID string, userID string, email string, location *time.Location) attendanceEvent {
	occurredAt := time.Date(2026, 6, 2, 2, 0, 0, 0, location).UTC()
	workspaceLocation, timeZoneName := service.workspaceTimeLocation()
	return attendanceEvent{
		ID:                 eventID,
		MattermostUserID:   userID,
		MattermostUsername: userID,
		Email:              email,
		DisplayName:        userID,
		Kind:               attendanceKindClockOut,
		OccurredAt:         occurredAt.Format(time.RFC3339Nano),
		LocalDate:          "2026-06-01",
		LocalTime:          occurredAt.In(workspaceLocation).Format("15:04:05"),
		TimeZoneAtEvent:    timeZoneName,
		Source:             attendanceSourceMattermostButton,
	}
}
