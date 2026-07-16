package admind

import (
	"context"
	"testing"
)

func TestPushConflictRollsBackEventWhenFieldClockPersistenceFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("push-conflict-atomic-rollback", "Original")
	baselineEvent.Location = "Original location"
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-original"`
	baselineEvent.RemoteHref = account.DefaultCalendarURL + baselineEvent.UID + ".ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	localEvent := baselineEvent
	localEvent.Title = "Local title"
	if errorValue := service.writeCalendarEvent(ctx, localEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteEvent := baselineEvent
	remoteEvent.Location = "Remote location"
	remoteICS, errorValue := encodeEventToICS(remoteEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `
CREATE TRIGGER fail_calendar_remote_winning_field_clock
BEFORE INSERT ON calendar_event_field_clocks
WHEN NEW.field = 'location'
BEGIN
	SELECT RAISE(FAIL, 'forced remote-winning field clock failure');
END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &fakeCalDAVPushClient{
		putErrorsQueue: map[string][]error{baselineEvent.RemoteHref: {errCalDAVPreconditionFailed}},
		putETags:       map[string]string{baselineEvent.RemoteHref: `"etag-after-merge"`},
		getObjects: map[string]calDAVCalendarObject{
			baselineEvent.RemoteHref: {Path: baselineEvent.RemoteHref, ETag: `"etag-remote"`, Data: remoteICS},
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != localEvent.Title || storedEvent.Location != baselineEvent.Location || storedEvent.RemoteETag != baselineEvent.RemoteETag || storedEvent.RawICS != baselineEvent.RawICS {
		t.Fatalf("partially committed event=%+v", storedEvent)
	}
	fieldClocks := readCalendarFieldClocksForEventTest(t, service, baselineEvent.UID)
	if !fieldClocks[calendarFieldLocation].IsZero() {
		t.Fatalf("location field clock committed=%s", fieldClocks[calendarFieldLocation])
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 || rows[0].AttemptCount != 1 {
		t.Fatalf("retryable outbox=%+v error=%v", rows, errorValue)
	}
}
