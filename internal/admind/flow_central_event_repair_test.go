package admind

import (
	"context"
	"testing"
)

func writeCalendarEventUID(t *testing.T, service *Service, uid string) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.ExecContext(context.Background(), `
	INSERT INTO calendar_events (id, uid, title, description, location, start_at, end_at, time_zone, is_all_day, color, raw_ics, created_by_email, created_by_name, updated_by_email, updated_by_name, updated_by_at, mattermost_post_id, updated_at, deleted_at, remote_source, remote_etag, remote_href)
	VALUES (?, ?, '회의', '', '', '2026-08-20T01:00:00Z', '2026-08-20T02:00:00Z', 'Asia/Seoul', 0, '', '', '', '', '', '', '', '', '2026-08-20T01:00:00Z', '', '', '', '')`,
		uid, uid); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func newFlowEventRepairTestService(t *testing.T) *Service {
	t.Helper()
	service := newFlowCentralTestService(t)
	service.Configuration.CalendarDatabasePath = service.Configuration.FlowDatabasePath
	return service
}

func TestAFlowTaskMadeFromAMeetingIsRemoved(t *testing.T) {
	service := newFlowEventRepairTestService(t)
	writeCalendarEventUID(t, service, "tool-dd1069c9@internkim")
	mirrored := flowNotificationTestTask("진행")
	mirrored.ID = "tool-dd1069c9@internkim"
	if errorValue := service.writeMirroredFlowTask(context.Background(), mirrored); errorValue != nil {
		t.Fatal(errorValue)
	}

	removed, errorValue := service.removeFlowTasksMadeFromCalendarEvents(context.Background())

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
	if _, found, errorValue := service.readFlowTaskByID(context.Background(), mirrored.ID); errorValue != nil || found {
		t.Fatalf("the meeting is still a task: found=%v error=%v", found, errorValue)
	}
	if entries := flowCentralOutboxEntries(t, service); len(entries) != 0 {
		t.Fatalf("removing the local copy must not queue a delete for the calendar event: %+v", entries)
	}
}

func TestATaskTheDeviceWroteIsLeftAlone(t *testing.T) {
	service := newFlowEventRepairTestService(t)
	writeCalendarEventUID(t, service, "tool-dd1069c9@internkim")
	own := flowNotificationTestTask("진행")
	own.ID = "5b24a6a5446c"
	if errorValue := service.writeFlowTask(context.Background(), own); errorValue != nil {
		t.Fatal(errorValue)
	}

	removed, errorValue := service.removeFlowTasksMadeFromCalendarEvents(context.Background())

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if removed != 0 {
		t.Fatalf("a task the device wrote is not a mirrored meeting, removed = %d", removed)
	}
	if _, found, errorValue := service.readFlowTaskByID(context.Background(), own.ID); errorValue != nil || !found {
		t.Fatalf("found=%v error=%v", found, errorValue)
	}
}
