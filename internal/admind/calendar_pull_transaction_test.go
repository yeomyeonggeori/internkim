package admind

import (
	"context"
	"strings"
	"testing"
)

func TestPullConflictRollsBackEventWhenOutboxRetentionFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("pull-transaction", "Original")
	baselineEvent.Location = "Original Room"
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-transaction.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	localEvent := baselineEvent
	localEvent.Title = "Local Title"
	if errorValue := service.writeCalendarEvent(ctx, localEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE TRIGGER fail_calendar_outbox_retention
BEFORE UPDATE ON calendar_outbox
BEGIN
	SELECT RAISE(FAIL, 'forced outbox retention failure');
END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}

	remoteEvent := baselineEvent
	remoteEvent.Location = "Remote Room"
	remoteEvent.RemoteETag = `"etag-remote"`
	remoteICS, errorValue := encodeEventToICS(remoteEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteEvent.RawICS = string(remoteICS)
	errorValue = service.applyPulledRemoteEvent(ctx, account, baselineEvent, true, remoteEvent)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced outbox retention failure") {
		t.Fatalf("error=%v", errorValue)
	}

	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != "Local Title" {
		t.Fatalf("title=%q want Local Title", storedEvent.Title)
	}
	if storedEvent.Location != "Original Room" {
		t.Fatalf("location=%q want Original Room", storedEvent.Location)
	}
	if storedEvent.RemoteETag != `"etag-base"` {
		t.Fatalf("remote etag=%q want etag-base", storedEvent.RemoteETag)
	}
}
