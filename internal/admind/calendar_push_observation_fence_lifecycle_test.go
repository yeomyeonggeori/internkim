package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCalendarPushObservationFenceSurvivesRestartAndForcesQueryWithUnchangedCTag(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "ctag-known"
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := newLocalTestCalendarEvent("push-fence-restart", "Push Fence Restart")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox before push: rows=%d error=%v", len(rows), errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TRIGGER fail_push_event_update
BEFORE UPDATE ON calendar_events
WHEN OLD.id = 'push-fence-restart'
BEGIN
	SELECT RAISE(ABORT, 'forced push state failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &fakeCalDAVPushClient{putETags: map[string]string{
		account.DefaultCalendarURL + event.UID + ".ics": `"etag-pushed"`,
	}}
	pushed, errorValue := service.pushCalendarOutboxPut(ctx, account, client, rows[0])
	if !pushed || errorValue == nil || !strings.Contains(errorValue.Error(), "forced push state failure") {
		t.Fatalf("push result: pushed=%v error=%v", pushed, errorValue)
	}

	restartedService := NewService(service.Configuration)
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, restartedService, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[event.UID]; !found {
		t.Fatalf("persisted fence missing after restart: %#v", fencedUIDs)
	}
	pullClient := &fakeCalDAVPullClient{ctag: "ctag-known"}
	changed, errorValue := restartedService.runGoogleCalendarPull(ctx, account, pullClient)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !changed || pullClient.queryCalls != 1 {
		t.Fatalf("unresolved fence did not force query: changed=%v queryCalls=%d", changed, pullClient.queryCalls)
	}
	rows, errorValue = restartedService.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox changed after failed push persistence: rows=%d error=%v", len(rows), errorValue)
	}
	storedEvent, found, errorValue := restartedService.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("event missing after failed push persistence: found=%v error=%v", found, errorValue)
	}
	if storedEvent.RemoteHref != "" || storedEvent.RemoteETag != "" {
		t.Fatalf("event remote state changed despite transaction failure: href=%q etag=%q", storedEvent.RemoteHref, storedEvent.RemoteETag)
	}
	secondRestartedService := NewService(service.Configuration)
	secondPullClient := &fakeCalDAVPullClient{ctag: "ctag-known"}
	if _, errorValue := secondRestartedService.runGoogleCalendarPull(ctx, account, secondPullClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := secondRestartedService.readCalendarEventByID(ctx, event.ID); errorValue != nil || !found {
		t.Fatalf("active PUT fence did not survive second missing snapshot: found=%v error=%v", found, errorValue)
	}
	rows, errorValue = secondRestartedService.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("active PUT changed after second missing snapshot: rows=%d error=%v", len(rows), errorValue)
	}
	if _, found, errorValue := secondRestartedService.readCalendarRemoteEventState(ctx, account.ID, account.DefaultCalendarURL, event.UID); errorValue != nil || found {
		t.Fatalf("active PUT missing snapshot persisted deletion bound: found=%v error=%v", found, errorValue)
	}
	fencedUIDs = readCalendarPushObservationFenceUIDsForTest(t, secondRestartedService, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[event.UID]; !found {
		t.Fatalf("active PUT missing snapshot cleared fence: %#v", fencedUIDs)
	}
}

func TestCalendarPushObservationFenceClearsAcceptedRemoteDeletionAndKeepsAmbiguousFailure(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	acceptedEvent := newLocalTestCalendarEvent("accepted-remote-delete-fence", "Accepted")
	acceptedEvent.RemoteSource = remoteCalendarProviderGoogle
	acceptedEvent.RemoteETag = `"etag-accepted"`
	acceptedEvent.RemoteHref = account.DefaultCalendarURL + acceptedEvent.UID + ".ics"
	acceptedICS, errorValue := encodeEventToICS(acceptedEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	acceptedEvent.RawICS = string(acceptedICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, acceptedEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: account.DefaultCalendarURL,
		EventUID:    acceptedEvent.UID,
		LastSeenAt:  time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	acceptedEvent.Title = "Accepted Local Edit"
	if errorValue := service.writeCalendarEvent(ctx, acceptedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	ambiguousEvent := newLocalTestCalendarEvent("ambiguous-remote-delete-fence", "Ambiguous")
	ambiguousEvent.RemoteSource = remoteCalendarProviderGoogle
	ambiguousEvent.RemoteETag = `"etag-ambiguous"`
	ambiguousEvent.RemoteHref = account.DefaultCalendarURL + ambiguousEvent.UID + ".ics"
	ambiguousICS, errorValue := encodeEventToICS(ambiguousEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	ambiguousEvent.RawICS = string(ambiguousICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, ambiguousEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	ambiguousEvent.Title = "Ambiguous Local Edit"
	if errorValue := service.writeCalendarEvent(ctx, ambiguousEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 2 {
		t.Fatalf("pending rows=%d error=%v", len(rows), errorValue)
	}
	rowsByUID := map[string]calendarOutboxRow{}
	for _, row := range rows {
		rowsByUID[row.EventUID] = row
	}
	client := &fakeCalDAVPushClient{
		putErrors: map[string]error{
			acceptedEvent.RemoteHref:  errCalDAVPreconditionFailed,
			ambiguousEvent.RemoteHref: errCalDAVPreconditionFailed,
		},
		getErrors: map[string]error{
			acceptedEvent.RemoteHref:  errCalDAVObjectNotFound,
			ambiguousEvent.RemoteHref: errCalDAVObjectNotFound,
		},
	}
	if pushed, errorValue := service.pushCalendarOutboxPut(ctx, account, client, rowsByUID[acceptedEvent.UID]); pushed || errorValue != nil {
		t.Fatalf("accepted deletion pushed=%v error=%v", pushed, errorValue)
	}
	if pushed, errorValue := service.pushCalendarOutboxPut(ctx, account, client, rowsByUID[ambiguousEvent.UID]); pushed || !isCalDAVObjectNotFound(errorValue) {
		t.Fatalf("ambiguous deletion pushed=%v error=%v", pushed, errorValue)
	}

	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[acceptedEvent.UID]; found {
		t.Fatal("accepted remote deletion left observation fence")
	}
	if _, found := fencedUIDs[ambiguousEvent.UID]; !found {
		t.Fatal("ambiguous remote deletion cleared observation fence")
	}
}

func TestCalendarPushObservationFenceReleasesOnSecondMissingSnapshotAfterRestart(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "ctag-old"
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := newLocalTestCalendarEvent("push-fence-expiry", "Push Fence Expiry")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-old"`
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, event.UID)
	firstClient := &fakeCalDAVPullClient{ctag: "ctag-new"}
	changed, errorValue := service.runGoogleCalendarPull(ctx, account, firstClient)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !changed || firstClient.queryCalls != 1 {
		t.Fatalf("first missing snapshot result: changed=%v queryCalls=%d", changed, firstClient.queryCalls)
	}
	if _, found, errorValue := service.readCalendarEventByID(ctx, event.ID); errorValue != nil || !found {
		t.Fatalf("first missing snapshot deleted fenced event: found=%v error=%v", found, errorValue)
	}
	remoteState, found, errorValue := service.readCalendarRemoteEventState(ctx, account.ID, account.DefaultCalendarURL, event.UID)
	if errorValue != nil || !found || remoteState.MissingDetectedAt == "" {
		t.Fatalf("first missing snapshot state: found=%v state=%+v error=%v", found, remoteState, errorValue)
	}
	refreshed, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil || !found {
		t.Fatalf("account after first pull: found=%v error=%v", found, errorValue)
	}
	if refreshed.DefaultCalendarCTag != "ctag-old" {
		t.Fatalf("first missing snapshot advanced ctag: %q", refreshed.DefaultCalendarCTag)
	}

	restartedService := NewService(service.Configuration)
	secondClient := &fakeCalDAVPullClient{ctag: "ctag-new"}
	changed, errorValue = restartedService.runGoogleCalendarPull(ctx, refreshed, secondClient)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !changed || secondClient.queryCalls != 1 {
		t.Fatalf("second missing snapshot result: changed=%v queryCalls=%d", changed, secondClient.queryCalls)
	}
	if _, found, errorValue := restartedService.readCalendarEventByID(ctx, event.ID); errorValue != nil || found {
		t.Fatalf("second missing snapshot event: found=%v error=%v", found, errorValue)
	}
	if fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, restartedService, account.ID, account.DefaultCalendarURL); len(fencedUIDs) != 0 {
		t.Fatalf("second missing snapshot left fence: %#v", fencedUIDs)
	}
	refreshed, found, errorValue = restartedService.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil || !found {
		t.Fatalf("account after second pull: found=%v error=%v", found, errorValue)
	}
	if refreshed.DefaultCalendarCTag != "ctag-new" {
		t.Fatalf("second missing snapshot did not advance ctag: %q", refreshed.DefaultCalendarCTag)
	}
}

func TestCalendarPushObservationFenceNewPutRestartsMissingSnapshotSequence(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.DefaultCalendarCTag = "ctag-old"
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := newLocalTestCalendarEvent("push-fence-new-put", "Push Fence New PUT")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = account.DefaultCalendarURL + event.UID + ".ics"
	event.RemoteETag = `"etag-old"`
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, event.UID)
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: "ctag-new"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	firstState, found, errorValue := service.readCalendarRemoteEventState(ctx, account.ID, account.DefaultCalendarURL, event.UID)
	if errorValue != nil || !found || firstState.MissingDetectedAt == "" {
		t.Fatalf("first missing snapshot state: found=%v state=%+v error=%v", found, firstState, errorValue)
	}

	updatedEvent := event
	updatedEvent.Title = "Repushed Local Event"
	if errorValue := service.writeCalendarEvent(ctx, updatedEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	pushClient := &fakeCalDAVPushClient{putETags: map[string]string{event.RemoteHref: `"etag-repushed"`}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, pushClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	stateAfterPush, found, errorValue := service.readCalendarRemoteEventState(ctx, account.ID, account.DefaultCalendarURL, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("state after push: found=%v error=%v", found, errorValue)
	}
	if stateAfterPush.MissingDetectedAt != "" {
		t.Fatalf("new PUT kept prior missing boundary: %q", stateAfterPush.MissingDetectedAt)
	}

	restartedService := NewService(service.Configuration)
	if _, errorValue := restartedService.runGoogleCalendarPull(ctx, account, &fakeCalDAVPullClient{ctag: "ctag-new"}); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := restartedService.readCalendarEventByID(ctx, event.ID); errorValue != nil || !found {
		t.Fatalf("first missing snapshot after new PUT deleted event: found=%v error=%v", found, errorValue)
	}
	secondFirstState, found, errorValue := restartedService.readCalendarRemoteEventState(ctx, account.ID, account.DefaultCalendarURL, event.UID)
	if errorValue != nil || !found || secondFirstState.MissingDetectedAt == "" {
		t.Fatalf("first missing snapshot after new PUT state: found=%v state=%+v error=%v", found, secondFirstState, errorValue)
	}
}
