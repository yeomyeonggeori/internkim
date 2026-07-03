package admind

import (
	"context"
	"testing"
	"time"
)

func TestReadExpiredCalendarMattermostPostEventIDsFiltersByAge(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()

	oldEvent := calendarTestEvent("old-event", "Old sync", "")
	if errorValue := service.writeCalendarEvent(ctx, oldEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.updateCalendarEventMattermostPostID(ctx, oldEvent.ID, "old-post"); errorValue != nil {
		t.Fatal(errorValue)
	}
	backdateCalendarMattermostPostCreatedAt(t, service, oldEvent.ID, time.Now().UTC().Add(-20*24*time.Hour))

	freshEvent := calendarTestEvent("fresh-event", "Fresh sync", "")
	if errorValue := service.writeCalendarEvent(ctx, freshEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.updateCalendarEventMattermostPostID(ctx, freshEvent.ID, "fresh-post"); errorValue != nil {
		t.Fatal(errorValue)
	}

	unpostedEvent := calendarTestEvent("unposted-event", "Unposted", "")
	if errorValue := service.writeCalendarEvent(ctx, unpostedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	eventIDs, errorValue := service.readExpiredCalendarMattermostPostEventIDs(ctx, time.Now().UTC().Add(-mattermostChannelPostRetentionDuration))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(eventIDs) != 1 || eventIDs[0] != "old-event" {
		t.Fatalf("expected only old-event, got %+v", eventIDs)
	}
}

func backdateCalendarMattermostPostCreatedAt(t *testing.T, service *Service, eventID string, postCreatedAt time.Time) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec("UPDATE calendar_events SET mattermost_post_created_at = ? WHERE id = ?", postCreatedAt.Format(time.RFC3339), eventID); errorValue != nil {
		t.Fatal(errorValue)
	}
}
