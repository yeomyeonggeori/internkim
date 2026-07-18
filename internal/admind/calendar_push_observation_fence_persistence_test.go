package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

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
