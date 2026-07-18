package admind

import (
	"context"
	"testing"
	"time"
)

func TestPullAdvancesRemoteWinningFieldClockWithinSameSecond(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("same-second-remote-clock", "Original")
	baselineObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, event, `"etag-original"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"ctag-original"`, objects: []calDAVCalendarObject{baselineObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	previousClock := remoteModifiedAt.Add(500 * time.Millisecond)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_event_field_clocks SET changed_at = ? WHERE event_uid = ? AND field = ?`, previousClock.Format(time.RFC3339Nano), event.UID, calendarFieldTitle); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_target_field_acknowledgements SET acknowledged_at = ? WHERE account_id = ? AND calendar_url = ? AND event_uid = ? AND field = ?`, previousClock.Format(time.RFC3339Nano), account.ID, canonicalCalendarTargetURL(account.DefaultCalendarURL), event.UID, calendarFieldTitle); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteEdit := event
	remoteEdit.Title = "Later remote title"
	remoteObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, remoteEdit, `"etag-later"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"ctag-later"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	fieldClock := readCalendarFieldClocksForEventTest(t, service, event.UID)[calendarFieldTitle]
	if !fieldClock.After(previousClock) {
		t.Fatalf("field clock=%s previous=%s", fieldClock, previousClock)
	}
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	acknowledgements, errorValue := readCalendarTargetFieldAcknowledgements(ctx, database, account.ID, account.DefaultCalendarURL, event.UID)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !acknowledgements[calendarFieldTitle].Equal(fieldClock) {
		t.Fatalf("title acknowledgement=%s field clock=%s", acknowledgements[calendarFieldTitle], fieldClock)
	}
}

func TestSameSecondRemoteEditBackfillsHistoricalTarget(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("same-second-target-backfill", "Original")
	baselineObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, event, `"etag-a"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"ctag-a"`, objects: []calDAVCalendarObject{baselineObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedB, errorValue := service.saveSelectedCalendar(ctx, account, "calendar-b", "B", "writer", "/calendars/b/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedB, &fakeCalDAVPullClient{ctag: `"ctag-b-initial"`}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedB, _, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	bPath := "/calendars/b/" + event.UID + ".ics"
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, selectedB, &fakeCalDAVPushClient{putETags: map[string]string{bPath: `"etag-b"`}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteEdit := event
	remoteEdit.Title = "Later remote B title"
	remoteObject := calendarRemoteObjectWithModifiedAt(t, "/calendars/b/", remoteEdit, `"etag-b-later"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedB, &fakeCalDAVPullClient{ctag: `"ctag-b-later"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedB, _, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.saveSelectedCalendar(ctx, selectedB, "calendar-a", "A", "writer", account.DefaultCalendarURL, time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	changedFields := []string{}
	for _, row := range rows {
		if row.EventUID == event.UID && row.Operation == calendarOutboxOperationPut && row.TargetCalendarURL == normalizeCalendarOutboxTargetURL(account.DefaultCalendarURL) {
			changedFields = mergeCalendarFieldLists(changedFields, row.ChangedFields)
		}
	}
	if len(changedFields) != 1 || changedFields[0] != calendarFieldTitle {
		t.Fatalf("calendar A changed fields=%v want [%s]", changedFields, calendarFieldTitle)
	}
}
