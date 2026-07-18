package admind

import (
	"context"
	"errors"
	"testing"
)

func TestPushCalendarOutboxRecoversMissingDeleteRemoteStateAfterConcurrentCreate(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("recover-concurrent-create-delete", "Create then delete")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &blockingCalendarPutClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
		etag:    `"etag-created"`,
	}
	pushResult := make(chan error, 1)
	go func() {
		_, pushError := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- pushError
	}()
	<-client.started
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `CREATE TRIGGER fail_concurrent_delete_remote_state BEFORE UPDATE OF remote_href ON calendar_outbox WHEN OLD.operation = 'delete' BEGIN SELECT RAISE(ABORT, 'delete remote state blocked'); END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	close(client.release)
	if errorValue := <-pushResult; errorValue != nil {
		t.Fatal(errorValue)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[event.UID]; !found {
		t.Fatal("concurrent create did not persist observation fence")
	}
	database, errorValue = service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `DROP TRIGGER fail_concurrent_delete_remote_state`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	service.markRemoteCalendarAccountAuthError(ctx, account, errors.New("caldav delete status 401: Unauthorized"))
	account, _, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
	recoveredObject := fakeRemoteObject(t, event.UID, client.etag, event.Title)
	recoveredObject.Path = expectedPath
	recoveryClient := &fakeCalDAVPushClient{getObjects: map[string]calDAVCalendarObject{
		expectedPath: recoveredObject,
	}}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, recoveryClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(recoveryClient.getCalls) != 1 || recoveryClient.getCalls[0] != expectedPath {
		t.Fatalf("recovery GET calls=%v want [%s]", recoveryClient.getCalls, expectedPath)
	}
	if len(recoveryClient.deleteCalls) != 1 {
		t.Fatalf("recovery DELETE calls=%+v want one", recoveryClient.deleteCalls)
	}
	if recoveryClient.deleteCalls[0].Path != expectedPath || recoveryClient.deleteCalls[0].IfMatch != client.etag {
		t.Fatalf("recovery DELETE=%+v", recoveryClient.deleteCalls[0])
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 0 {
		t.Fatalf("remaining rows=%+v error=%v", remainingRows, errorValue)
	}
	fencedUIDs = readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[event.UID]; found {
		t.Fatal("completed DELETE left PUT observation fence")
	}
	account, _, errorValue = service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if account.LastAuthError != "" || account.LastAuthErrorAt != "" {
		t.Fatalf("auth error should clear after recovered DELETE: %+v", account)
	}
}
