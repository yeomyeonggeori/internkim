package admind

import (
	"context"
	"strings"
	"testing"
)

func TestCalendarPushObservationFenceClearsOnlyAfterCompleteObservedSnapshot(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "ctag-old"
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	uid := "push-fence-observed@internkim"
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, uid)
	observedObject := fakeRemoteObject(t, uid, `"etag-observed"`, "Observed")
	partialClient := &fakeCalDAVPullClient{
		ctag: "ctag-new",
		objects: []calDAVCalendarObject{
			observedObject,
			{Path: account.DefaultCalendarURL + "invalid.ics", Data: []byte("invalid calendar data")},
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, partialClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[uid]; !found {
		t.Fatal("partial snapshot cleared observed fence")
	}
	refreshed, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if refreshed.DefaultCalendarCTag != "ctag-old" {
		t.Fatalf("partial snapshot advanced ctag: %q", refreshed.DefaultCalendarCTag)
	}

	completeClient := &fakeCalDAVPullClient{ctag: "ctag-new", objects: []calDAVCalendarObject{observedObject}}
	if _, errorValue := service.runGoogleCalendarPull(ctx, refreshed, completeClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	fencedUIDs = readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[uid]; found {
		t.Fatal("complete observed snapshot did not clear fence")
	}
	refreshed, _, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if refreshed.DefaultCalendarCTag != "ctag-new" {
		t.Fatalf("complete observed snapshot did not advance ctag: %q", refreshed.DefaultCalendarCTag)
	}
}

func TestCalendarPushObservationFenceSurvivesMissingDeleteFailureAndForcesRetry(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "ctag-known"
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	fencedUID := "fence-retry@internkim"
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, fencedUID)
	missingEvent := newLocalTestCalendarEvent("missing-delete-failure", "Missing Delete Failure")
	missingEvent.RemoteSource = remoteCalendarProviderGoogle
	missingEvent.RemoteHref = account.DefaultCalendarURL + missingEvent.UID + ".ics"
	missingEvent.RemoteETag = `"etag-missing"`
	if errorValue := service.writeCalendarEventWithSource(ctx, missingEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TRIGGER fail_fenced_pull_missing_delete
BEFORE UPDATE OF deleted_at ON calendar_events
WHEN OLD.id = 'missing-delete-failure'
BEGIN
	SELECT RAISE(ABORT, 'forced missing delete failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	observedObject := fakeRemoteObject(t, fencedUID, `"etag-observed"`, "Observed Fence")
	firstClient := &fakeCalDAVPullClient{ctag: "ctag-known", objects: []calDAVCalendarObject{observedObject}}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, firstClient); errorValue == nil || !strings.Contains(errorValue.Error(), "forced missing delete failure") {
		t.Fatalf("missing-delete failure: %v", errorValue)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[fencedUID]; !found {
		t.Fatal("missing-delete failure cleared observation fence")
	}
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, "DROP TRIGGER fail_fenced_pull_missing_delete")
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secondClient := &fakeCalDAVPullClient{ctag: "ctag-known", objects: []calDAVCalendarObject{observedObject, fakeRemoteObject(t, missingEvent.UID, missingEvent.RemoteETag, missingEvent.Title)}}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, secondClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	if secondClient.queryCalls != 1 {
		t.Fatalf("retry pull skipped query after failed reconciliation: %d", secondClient.queryCalls)
	}
}

func TestCalendarPushObservationFenceClearsMultipleObservedFencesOnly(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "ctag-old"
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	firstUID := "bulk-first@internkim"
	secondUID := "bulk-second@internkim"
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, firstUID)
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, secondUID)
	client := &fakeCalDAVPullClient{
		ctag: "ctag-new",
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, firstUID, `"etag-first"`, "First"),
			fakeRemoteObject(t, secondUID, `"etag-second"`, "Second"),
			fakeRemoteObject(t, "not-fenced@internkim", `"etag-other"`, "Other"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if len(fencedUIDs) != 0 {
		t.Fatalf("observed fences remain after bulk clear: %#v", fencedUIDs)
	}
}
