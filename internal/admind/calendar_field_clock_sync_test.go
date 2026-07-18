package admind

import (
	"context"
	"testing"
	"time"
)

func TestPullSeedsNewRemoteEventFieldClocks(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("remote-field-clock-seed", "Remote")
	remoteObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, event, `"etag-seed"`, remoteModifiedAt)

	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"seed-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	fieldClocks := readCalendarFieldClocksForEventTest(t, service, event.UID)
	for _, field := range calendarAllUserEditableFields() {
		if !fieldClocks[field].Equal(remoteModifiedAt) {
			t.Fatalf("field %q clock=%s want %s", field, fieldClocks[field], remoteModifiedAt)
		}
	}
}

func TestPullSeedsObservedClockWhenRemoteModifiedAtIsMissing(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteObject := fakeRemoteObject(t, "remote-field-clock-fallback@google", `"etag-fallback"`, "Remote")
	remoteObject.Path = account.DefaultCalendarURL + "remote-field-clock-fallback@google.ics"
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"fallback-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	fieldClocks := readCalendarFieldClocksForEventTest(t, service, "remote-field-clock-fallback@google")
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	acknowledgements, errorValue := readCalendarTargetFieldAcknowledgements(ctx, database, account.ID, account.DefaultCalendarURL, "remote-field-clock-fallback@google")
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, field := range calendarAllUserEditableFields() {
		if fieldClocks[field].IsZero() || !acknowledgements[field].Equal(fieldClocks[field]) {
			t.Fatalf("field %q clock=%s acknowledgement=%s", field, fieldClocks[field], acknowledgements[field])
		}
	}
}

func TestPullUpdatesOnlyRemoteWinningFieldClocks(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineModifiedAt := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Second)
	baseline := newLocalTestCalendarEvent("remote-field-clock-winner", "Original")
	baseline.Description = "Original description"
	baselineObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, baseline, `"etag-baseline"`, baselineModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"baseline-ctag"`, objects: []calDAVCalendarObject{baselineObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedBaseline, found, errorValue := service.readCalendarEventByUID(ctx, baseline.UID)
	if errorValue != nil || !found {
		t.Fatalf("baseline found=%v error=%v", found, errorValue)
	}
	localEdit := storedBaseline
	localEdit.Title = "Local title"
	localEdit.Description = "Local description"
	if errorValue := service.writeCalendarEvent(ctx, localEdit); errorValue != nil {
		t.Fatal(errorValue)
	}
	localFieldClocks := readCalendarFieldClocksForEventTest(t, service, baseline.UID)
	remoteModifiedAt := localFieldClocks[calendarFieldTitle].Truncate(time.Second).Add(time.Second)
	remoteEdit := storedBaseline
	remoteEdit.Title = "Remote title"
	remoteObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, remoteEdit, `"etag-remote"`, remoteModifiedAt)

	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"remote-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}

	fieldClocks := readCalendarFieldClocksForEventTest(t, service, baseline.UID)
	if !fieldClocks[calendarFieldTitle].Equal(remoteModifiedAt) {
		t.Fatalf("remote-winning title clock=%s want %s", fieldClocks[calendarFieldTitle], remoteModifiedAt)
	}
	if !fieldClocks[calendarFieldDescription].Equal(localFieldClocks[calendarFieldDescription]) {
		t.Fatalf("local-winning description clock=%s want %s", fieldClocks[calendarFieldDescription], localFieldClocks[calendarFieldDescription])
	}
	storedEvent, found, errorValue := service.readCalendarEventByUID(ctx, baseline.UID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != remoteEdit.Title || storedEvent.Description != localEdit.Description {
		t.Fatalf("merged event title=%q description=%q", storedEvent.Title, storedEvent.Description)
	}
}

func TestLocalEditAdvancesPastFutureRemoteFieldClock(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("future-remote-field-clock", "Remote")
	remoteObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, event, `"etag-future"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"future-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByUID(ctx, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	storedEvent.Title = "Later local edit"
	if errorValue := service.writeCalendarEvent(ctx, storedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	fieldClocks := readCalendarFieldClocksForEventTest(t, service, event.UID)
	if !fieldClocks[calendarFieldTitle].After(remoteModifiedAt) {
		t.Fatalf("local title clock=%s remote clock=%s", fieldClocks[calendarFieldTitle], remoteModifiedAt)
	}
}

func TestSameTargetReselectionDoesNotBackfillAcknowledgedPush(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("same-target-push-ack", "Original")
	remoteObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, event, `"etag-original"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"initial-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByUID(ctx, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	storedEvent.Title = "Pushed title"
	if errorValue := service.writeCalendarEvent(ctx, storedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &fakeCalDAVPushClient{putETags: map[string]string{remoteObject.Path: `"etag-pushed"`}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.saveSelectedCalendar(ctx, account, "same-target", "Same", "writer", account.DefaultCalendarURL, time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(rows) != 0 {
		t.Fatalf("acknowledged push was backfilled again: %+v", rows)
	}
}

func TestSwitchBackPullPreservesLatestLocalEdit(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("switch-back-latest-edit", "Remote A")
	remoteObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, event, `"etag-a"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"a-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedB, errorValue := service.saveSelectedCalendar(ctx, account, "calendar-b", "B", "writer", "/calendars/b/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByUID(ctx, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	storedEvent.Title = "Latest local edit"
	if errorValue := service.writeCalendarEvent(ctx, storedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedA, errorValue := service.saveSelectedCalendar(ctx, selectedB, "calendar-a", "A", "writer", account.DefaultCalendarURL, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCalendarTargetOutboxOperationTest(t, service, account.ID, account.DefaultCalendarURL, event.UID, calendarOutboxOperationPut)
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedA, &fakeCalDAVPullClient{ctag: `"a-new-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	finalEvent, found, errorValue := service.readCalendarEventByUID(ctx, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != storedEvent.Title {
		t.Fatalf("title=%q want %q", finalEvent.Title, storedEvent.Title)
	}
}

func TestSwitchBackRequeuesLocalWinnerAfterOtherTargetPush(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("switch-back-local-winner", "Remote A")
	event.Description = "Original description"
	baselineObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, event, `"etag-a"`, baselineModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"a-ctag"`, objects: []calDAVCalendarObject{baselineObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByUID(ctx, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	storedEvent.Title = "Local winner"
	if errorValue := service.writeCalendarEvent(ctx, storedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	localTitleClock := readCalendarFieldClocksForEventTest(t, service, event.UID)[calendarFieldTitle]
	remoteDescription := event
	remoteDescription.Description = "Remote description"
	remoteModifiedAt := localTitleClock.Truncate(time.Second).Add(time.Second)
	remoteObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, remoteDescription, `"etag-a-remote"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"a-remote-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedB, errorValue := service.saveSelectedCalendar(ctx, account, "calendar-b", "B", "writer", "/calendars/b/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedB, &fakeCalDAVPullClient{ctag: `"b-ctag"`}); errorValue != nil {
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
		t.Fatalf("switch-back changed fields=%v want [%s]", changedFields, calendarFieldTitle)
	}
}

func TestSwitchBackPullPreservesLatestLocalDelete(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("switch-back-latest-delete", "Remote A")
	remoteObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, event, `"etag-a"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"a-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedB, errorValue := service.saveSelectedCalendar(ctx, account, "calendar-b", "B", "writer", "/calendars/b/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByUID(ctx, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, storedEvent.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedA, errorValue := service.saveSelectedCalendar(ctx, selectedB, "calendar-a", "A", "writer", account.DefaultCalendarURL, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	assertCalendarTargetOutboxOperationTest(t, service, account.ID, account.DefaultCalendarURL, event.UID, calendarOutboxOperationDelete)
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedA, &fakeCalDAVPullClient{ctag: `"a-new-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := service.readCalendarEventByUID(ctx, event.UID); errorValue != nil || found {
		t.Fatalf("deleted event restored: found=%v error=%v", found, errorValue)
	}
}

func TestSameTargetReselectionDoesNotBackfillAcknowledgedDelete(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	remoteModifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	event := newLocalTestCalendarEvent("same-target-delete-ack", "Remote")
	remoteObject := calendarRemoteObjectWithModifiedAt(t, account.DefaultCalendarURL, event, `"etag-delete"`, remoteModifiedAt)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: `"delete-ctag"`, objects: []calDAVCalendarObject{remoteObject}}); errorValue != nil {
		t.Fatal(errorValue)
	}
	storedEvent, found, errorValue := service.readCalendarEventByUID(ctx, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, storedEvent.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	deleteClient := &fakeCalDAVPushClient{getObjects: map[string]calDAVCalendarObject{remoteObject.Path: remoteObject}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, deleteClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.saveSelectedCalendar(ctx, account, "same-target", "Same", "writer", account.DefaultCalendarURL, time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(rows) != 0 {
		t.Fatalf("acknowledged delete was backfilled again: %+v", rows)
	}
}

func TestRemoteDeletionDoesNotCreateLocalDeletionClock(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("remote-delete-no-local-clock", "Remote")
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEventWithSource(ctx, event.ID, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	fieldClocks := readCalendarFieldClocksForEventTest(t, service, event.UID)
	if !fieldClocks[calendarEventDeletionClockField].IsZero() {
		t.Fatalf("remote deletion created local deletion clock=%s", fieldClocks[calendarEventDeletionClockField])
	}
}

func calendarRemoteObjectWithModifiedAt(t *testing.T, calendarURL string, event calendarEvent, etag string, modifiedAt time.Time) calDAVCalendarObject {
	t.Helper()
	return calDAVCalendarObject{
		Path: calendarURL + event.UID + ".ics",
		ETag: etag,
		Data: encodeCalendarTestEventWithLastModified(t, event, modifiedAt),
	}
}

func readCalendarFieldClocksForEventTest(t *testing.T, service *Service, eventUID string) map[string]time.Time {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	fieldClocks, errorValue := readCalendarEventFieldClocks(context.Background(), database)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return fieldClocks[eventUID]
}

func assertCalendarTargetOutboxOperationTest(t *testing.T, service *Service, accountID string, calendarURL string, eventUID string, operation string) {
	t.Helper()
	rows, errorValue := service.listPendingCalendarOutbox(context.Background(), accountID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, row := range rows {
		if row.EventUID == eventUID && row.Operation == operation && row.TargetCalendarURL == normalizeCalendarOutboxTargetURL(calendarURL) {
			return
		}
	}
	t.Fatalf("target operation %q missing for %s: %+v", operation, eventUID, rows)
}
