package admind

import (
	"context"
	"database/sql"
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

func TestDeleteCalendarPushObservationFencesUsesOneParameterizedWrite(t *testing.T) {
	runner := &calendarPushObservationFenceCountingRunner{}
	fencedUIDs := map[string]struct{}{
		"bulk-first@internkim":  {},
		"bulk-second@internkim": {},
	}
	observedUIDs := map[string]struct{}{
		"bulk-first@internkim":  {},
		"bulk-second@internkim": {},
		"not-fenced@internkim":  {},
	}
	eventUIDs := observedCalendarPushObservationFenceUIDs(fencedUIDs, observedUIDs)
	if errorValue := deleteCalendarPushObservationFencesWithRunner(context.Background(), runner, "account", "/calendar/", eventUIDs); errorValue != nil {
		t.Fatal(errorValue)
	}
	if runner.executionCalls != 1 {
		t.Fatalf("bulk fence delete writes=%d want 1", runner.executionCalls)
	}
	if !strings.Contains(runner.query, "event_uid IN (?,?)") {
		t.Fatalf("bulk fence delete query=%q", runner.query)
	}
	if len(runner.arguments) != 4 {
		t.Fatalf("bulk fence delete argument count=%d want 4", len(runner.arguments))
	}
	emptyRunner := &calendarPushObservationFenceCountingRunner{}
	if errorValue := deleteCalendarPushObservationFencesWithRunner(context.Background(), emptyRunner, "account", "/calendar/", nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	if emptyRunner.executionCalls != 0 {
		t.Fatalf("empty fence delete writes=%d want 0", emptyRunner.executionCalls)
	}
}

func TestDeleteRemoteCalendarAccountDeletesObservationFences(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, "cleanup@internkim")
	if errorValue := service.deleteRemoteCalendarAccount(ctx, account.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if len(fencedUIDs) != 0 {
		t.Fatalf("account cleanup left fences: %#v", fencedUIDs)
	}
}

func TestCalendarPushObservationFenceUsesCanonicalAbsoluteTargetIdentity(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	absoluteCalendarURL := "https://calendar.example.com/calendars/company/"
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", absoluteCalendarURL, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := newLocalTestCalendarEvent("absolute-fence", "Absolute Fence")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, &fakeCalDAVPullClient{ctag: `"initial-ctag"`}); errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedAccount, _, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	objectPath := "/calendars/company/" + event.UID + ".ics"
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, selectedAccount, &fakeCalDAVPushClient{
		putETags: map[string]string{objectPath: `"etag-company"`},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	fencedUIDs, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, absoluteCalendarURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found := fencedUIDs[event.UID]; !found {
		t.Fatalf("absolute target lookup missed persisted fence: %+v", fencedUIDs)
	}
	remoteObject := fakeRemoteObject(t, event.UID, `"etag-company"`, event.Title)
	remoteObject.Path = objectPath
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, &fakeCalDAVPullClient{
		ctag:    `"observed-ctag"`,
		objects: []calDAVCalendarObject{remoteObject},
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	fencedUIDs, errorValue = service.listCalendarPushObservationFenceUIDs(ctx, account.ID, absoluteCalendarURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(fencedUIDs) != 0 {
		t.Fatalf("observed absolute target fence remains: %+v", fencedUIDs)
	}
}

func TestDeleteRemoteCalendarAccountRollsBackFenceCleanupWhenAccountDeleteFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	fencedUID := "cleanup-rollback@internkim"
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, fencedUID)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TRIGGER fail_remote_account_delete
BEFORE DELETE ON calendar_remote_accounts
WHEN OLD.id = 'google-test'
BEGIN
	SELECT RAISE(ABORT, 'forced account delete failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.deleteRemoteCalendarAccount(ctx, account.ID); errorValue == nil || !strings.Contains(errorValue.Error(), "forced account delete failure") {
		t.Fatalf("account delete failure: %v", errorValue)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[fencedUID]; !found {
		t.Fatal("account delete failure did not roll back fence cleanup")
	}
	_, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil || !found {
		t.Fatalf("account missing after rollback: found=%v error=%v", found, errorValue)
	}
}

func TestCalendarPushObservationFenceWriteFailurePreservesEventAndOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("push-fence-write-failure", "Fence Write Failure")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	rowsBefore, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rowsBefore) != 1 {
		t.Fatalf("outbox before failure: rows=%d error=%v", len(rowsBefore), errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TRIGGER fail_push_fence_insert
BEFORE INSERT ON calendar_push_observation_fences
BEGIN
	SELECT RAISE(ABORT, 'forced fence insert failure');
END`)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &fakeCalDAVPushClient{}
	pushed, errorValue := service.pushCalendarOutboxPut(ctx, account, client, rowsBefore[0])
	if pushed || errorValue == nil || !strings.Contains(errorValue.Error(), "forced fence insert failure") {
		t.Fatalf("push result: pushed=%v error=%v", pushed, errorValue)
	}
	if len(client.putCalls) != 0 {
		t.Fatalf("remote PUT ran before fence persistence: %d calls", len(client.putCalls))
	}
	rowsAfter, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rowsAfter) != 1 {
		t.Fatalf("outbox after failure: rows=%d error=%v", len(rowsAfter), errorValue)
	}
	if rowsAfter[0].ID != rowsBefore[0].ID || rowsAfter[0].AttemptCount != rowsBefore[0].AttemptCount || rowsAfter[0].LastError != rowsBefore[0].LastError {
		t.Fatalf("outbox changed after fence failure: before=%+v after=%+v", rowsBefore[0], rowsAfter[0])
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("event after failure: found=%v error=%v", found, errorValue)
	}
	if storedEvent.RemoteHref != "" || storedEvent.RemoteETag != "" {
		t.Fatalf("event changed after fence failure: href=%q etag=%q", storedEvent.RemoteHref, storedEvent.RemoteETag)
	}
}

func seedCalendarPushObservationFenceForTest(t *testing.T, service *Service, accountID string, calendarURL string, eventUID string) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(context.Background(), `
INSERT INTO calendar_push_observation_fences (account_id, calendar_url, event_uid, created_at)
VALUES (?, ?, ?, ?)`, accountID, calendarURL, eventUID, time.Now().UTC().Format(time.RFC3339Nano))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
}

func readCalendarPushObservationFenceUIDsForTest(t *testing.T, service *Service, accountID string, calendarURL string) map[string]struct{} {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(context.Background(), `
SELECT event_uid
FROM calendar_push_observation_fences
WHERE account_id = ? AND calendar_url = ?`, accountID, calendarURL)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer rows.Close()
	result := map[string]struct{}{}
	for rows.Next() {
		var eventUID string
		if errorValue := rows.Scan(&eventUID); errorValue != nil {
			t.Fatal(errorValue)
		}
		result[eventUID] = struct{}{}
	}
	if errorValue := rows.Err(); errorValue != nil {
		t.Fatal(errorValue)
	}
	return result
}

type calendarPushObservationFenceCountingRunner struct {
	executionCalls int
	query          string
	arguments      []any
}

func (runner *calendarPushObservationFenceCountingRunner) ExecContext(ctx context.Context, query string, arguments ...any) (sql.Result, error) {
	runner.executionCalls++
	runner.query = query
	runner.arguments = append([]any(nil), arguments...)
	return calendarPushObservationFenceTestResult{}, nil
}

func (runner *calendarPushObservationFenceCountingRunner) QueryContext(ctx context.Context, query string, arguments ...any) (*sql.Rows, error) {
	return nil, nil
}

type calendarPushObservationFenceTestResult struct{}

func (calendarPushObservationFenceTestResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (calendarPushObservationFenceTestResult) RowsAffected() (int64, error) {
	return 0, nil
}
