package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCalendarPullSnapshotObservedBatchFailurePreventsFinalization(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "ctag-before-batch-failure"
	account, errorValue := service.upsertRemoteCalendarAccount(contextValue, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	target := activeRemoteCalendarTarget(account)
	firstObject := fakeRemoteObject(t, "snapshot-batch-first@google", `"etag-first"`, "First")
	firstObject.Path = target.CalendarURL + "snapshot-batch-first.ics"
	secondObject := fakeRemoteObject(t, "snapshot-batch-second@google", `"etag-second"`, "Second")
	secondObject.Path = target.CalendarURL + "snapshot-batch-second.ics"
	seedCalendarPushObservationFenceForTest(t, service, account.ID, target.CalendarURL, "snapshot-batch-first@google")
	database, errorValue := service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(contextValue, `
CREATE TRIGGER fail_second_snapshot_observed_state
BEFORE INSERT ON calendar_remote_event_sync_state
WHEN NEW.event_uid = 'snapshot-batch-second@google'
BEGIN
	SELECT RAISE(ABORT, 'forced snapshot observed batch failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	_, errorValue = service.runGoogleCalendarPull(contextValue, account, &fakeCalDAVPullClient{
		ctag:    "ctag-after-batch-failure",
		objects: []calDAVCalendarObject{firstObject, secondObject},
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced snapshot observed batch failure") {
		t.Fatalf("pull error=%v", errorValue)
	}
	for _, eventUID := range []string{"snapshot-batch-first@google", "snapshot-batch-second@google"} {
		_, found, readError := service.readCalendarEventByUID(contextValue, eventUID)
		if readError != nil {
			t.Fatal(readError)
		}
		if found {
			t.Fatalf("calendar event persisted for %s", eventUID)
		}
		_, found, readError = service.readCalendarRemoteEventState(contextValue, account.ID, target.CalendarURL, eventUID)
		if readError != nil {
			t.Fatal(readError)
		}
		if found {
			t.Fatalf("remote event state persisted for %s", eventUID)
		}
	}
	database, errorValue = service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var logicalClockCount int
	if errorValue := database.QueryRowContext(contextValue, `SELECT COUNT(*) FROM calendar_event_logical_clocks WHERE event_uid IN (?, ?)`, "snapshot-batch-first@google", "snapshot-batch-second@google").Scan(&logicalClockCount); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if logicalClockCount != 0 {
		t.Fatalf("logical clock count=%d want 0", logicalClockCount)
	}
	refreshedAccount, found, errorValue := service.readRemoteCalendarAccountByProvider(contextValue, remoteCalendarProviderGoogle)
	if errorValue != nil || !found {
		t.Fatalf("account found=%v error=%v", found, errorValue)
	}
	if refreshedAccount.DefaultCalendarCTag != account.DefaultCalendarCTag {
		t.Fatalf("ctag=%q want %q", refreshedAccount.DefaultCalendarCTag, account.DefaultCalendarCTag)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, target.CalendarURL)
	if _, found := fencedUIDs["snapshot-batch-first@google"]; !found {
		t.Fatal("observation fence was finalized after failed observed batch")
	}
}

func TestCalendarPullMixedValidInvalidSnapshotAppliesValidEventsWithoutFinalization(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "ctag-before-incomplete-snapshot"
	account, errorValue := service.upsertRemoteCalendarAccount(contextValue, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	target := activeRemoteCalendarTarget(account)
	missingEvent := newLocalTestCalendarEvent("mixed-snapshot-missing", "Must Remain")
	missingEvent.RemoteSource = remoteCalendarProviderGoogle
	missingEvent.RemoteETag = `"etag-missing"`
	missingEvent.RemoteHref = target.CalendarURL + "mixed-snapshot-missing.ics"
	if errorValue := service.writeCalendarEventWithSource(contextValue, missingEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	validUID := "mixed-snapshot-valid@google"
	validObject := fakeRemoteObject(t, validUID, `"etag-valid"`, "Valid Remote")
	validObject.Path = target.CalendarURL + "mixed-snapshot-valid.ics"
	invalidObject := calDAVCalendarObject{
		Path: target.CalendarURL + "mixed-snapshot-invalid.ics",
		ETag: `"etag-invalid"`,
		Data: []byte("invalid calendar object"),
	}
	decodedEvents, decodeResult := decodeCalendarPullSnapshot([]calDAVCalendarObject{validObject, invalidObject}, account.AccountEmail)
	if decodeResult.IsComplete {
		t.Fatal("mixed snapshot was reported as complete")
	}
	if len(decodedEvents) != 1 || decodedEvents[0].UID != validUID {
		t.Fatalf("decoded events=%+v", decodedEvents)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, target.CalendarURL, validUID)

	changed, errorValue := service.runGoogleCalendarPull(contextValue, account, &fakeCalDAVPullClient{
		ctag:    "ctag-after-incomplete-snapshot",
		objects: []calDAVCalendarObject{validObject, invalidObject},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !changed {
		t.Fatal("incomplete snapshot did not report a processed remote change")
	}
	validEvent, found, errorValue := service.readCalendarEventByUID(contextValue, validUID)
	if errorValue != nil || !found {
		t.Fatalf("valid event found=%v error=%v", found, errorValue)
	}
	if validEvent.Title != "Valid Remote" {
		t.Fatalf("valid event title=%q", validEvent.Title)
	}
	if _, found, errorValue := service.readCalendarEventByUID(contextValue, missingEvent.UID); errorValue != nil || !found {
		t.Fatalf("missing event found=%v error=%v", found, errorValue)
	}
	refreshedAccount, found, errorValue := service.readRemoteCalendarAccountByProvider(contextValue, remoteCalendarProviderGoogle)
	if errorValue != nil || !found {
		t.Fatalf("account found=%v error=%v", found, errorValue)
	}
	if refreshedAccount.DefaultCalendarCTag != account.DefaultCalendarCTag {
		t.Fatalf("ctag=%q want %q", refreshedAccount.DefaultCalendarCTag, account.DefaultCalendarCTag)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, target.CalendarURL)
	if _, found := fencedUIDs[validUID]; !found {
		t.Fatal("observed fence was finalized for incomplete snapshot")
	}
}

func TestUnchangedCalendarPullObservedBatchFailureRollsBackAllStatesAndLogicalClocks(t *testing.T) {
	service := newCalendarTestService(t)
	contextValue := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "unchanged-batch-ctag"
	account, errorValue := service.upsertRemoteCalendarAccount(contextValue, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	target := activeRemoteCalendarTarget(account)
	firstEvent := newLocalTestCalendarEvent("unchanged-batch-first", "First")
	firstEvent.RemoteSource = remoteCalendarProviderGoogle
	firstEvent.RemoteETag = `"etag-first"`
	firstEvent.RemoteHref = target.CalendarURL + "unchanged-batch-first.ics"
	secondEvent := newLocalTestCalendarEvent("unchanged-batch-second", "Second")
	secondEvent.RemoteSource = remoteCalendarProviderGoogle
	secondEvent.RemoteETag = `"etag-second"`
	secondEvent.RemoteHref = target.CalendarURL + "unchanged-batch-second.ics"
	for _, event := range []calendarEvent{firstEvent, secondEvent} {
		if errorValue := service.writeCalendarEventWithSource(contextValue, event, calendarSourcePull); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	previousLastSeenAt := time.Now().UTC().Add(-2 * time.Hour).Format(time.RFC3339Nano)
	previousMissingAt := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	baselineStates := map[string]calendarRemoteEventState{}
	for _, event := range []calendarEvent{firstEvent, secondEvent} {
		state := calendarRemoteEventState{
			AccountID:         account.ID,
			CalendarURL:       target.CalendarURL,
			EventUID:          event.UID,
			RemoteModifiedAt:  "2036-07-16T12:00:00Z",
			LastSeenAt:        previousLastSeenAt,
			MissingDetectedAt: previousMissingAt,
		}
		if errorValue := service.upsertCalendarRemoteEventState(contextValue, state); errorValue != nil {
			t.Fatal(errorValue)
		}
		baselineStates[event.UID] = state
	}
	database, errorValue := service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(contextValue, `
CREATE TRIGGER fail_second_unchanged_observed_state
BEFORE UPDATE ON calendar_remote_event_sync_state
WHEN NEW.event_uid = 'unchanged-batch-second@internkim'
BEGIN
	SELECT RAISE(ABORT, 'forced unchanged observed batch failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	client := &fakeCalDAVPullClient{ctag: account.DefaultCalendarCTag}
	_, errorValue = service.runGoogleCalendarPull(contextValue, account, client)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "forced unchanged observed batch failure") {
		t.Fatalf("pull error=%v", errorValue)
	}
	if client.queryCalls != 0 {
		t.Fatalf("query calls=%d want 0", client.queryCalls)
	}
	for _, event := range []calendarEvent{firstEvent, secondEvent} {
		state, found, readError := service.readCalendarRemoteEventState(contextValue, account.ID, target.CalendarURL, event.UID)
		if readError != nil || !found {
			t.Fatalf("state for %s found=%v error=%v", event.UID, found, readError)
		}
		if state != baselineStates[event.UID] {
			t.Fatalf("state for %s=%+v want %+v", event.UID, state, baselineStates[event.UID])
		}
	}
	database, errorValue = service.openCalendarDatabase(contextValue)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	var logicalClockCount int
	if errorValue := database.QueryRowContext(contextValue, `SELECT COUNT(*) FROM calendar_event_logical_clocks WHERE event_uid IN (?, ?)`, firstEvent.UID, secondEvent.UID).Scan(&logicalClockCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	if logicalClockCount != 0 {
		t.Fatalf("logical clock count=%d want 0", logicalClockCount)
	}
}
