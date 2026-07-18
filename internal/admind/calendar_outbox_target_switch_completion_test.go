package admind

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type multiBatchBlockingCalendarPushClient struct {
	mutex        sync.Mutex
	firstStarted chan struct{}
	firstRelease chan struct{}
	firstError   error
	putPaths     []string
	deletePaths  []string
	getPaths     []string
}

func (client *multiBatchBlockingCalendarPushClient) putCalendarObject(_ context.Context, objectPath string, _ []byte, _ string, _ string) (string, error) {
	client.mutex.Lock()
	client.putPaths = append(client.putPaths, objectPath)
	callCount := len(client.putPaths)
	client.mutex.Unlock()
	if callCount == 1 {
		close(client.firstStarted)
		<-client.firstRelease
	}
	return `"etag-a"`, client.firstError
}

func (client *multiBatchBlockingCalendarPushClient) deleteCalendarObject(_ context.Context, objectPath string, _ string) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	client.deletePaths = append(client.deletePaths, objectPath)
	return nil
}

func (client *multiBatchBlockingCalendarPushClient) getCalendarObject(_ context.Context, objectPath string) (calDAVCalendarObject, error) {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	client.getPaths = append(client.getPaths, objectPath)
	return calDAVCalendarObject{Path: objectPath, ETag: `"etag-remote"`, Data: []byte("invalid")}, nil
}

func (client *multiBatchBlockingCalendarPushClient) deleteCallCount() int {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	return len(client.deletePaths)
}

func (client *multiBatchBlockingCalendarPushClient) getCallCount() int {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	return len(client.getPaths)
}

func (client *multiBatchBlockingCalendarPushClient) putCallCount() int {
	client.mutex.Lock()
	defer client.mutex.Unlock()
	return len(client.putPaths)
}

type blockingCalendarPushClient struct {
	started chan struct{}
	release chan struct{}
	etag    string
	error   error
}

func (client *blockingCalendarPushClient) putCalendarObject(context.Context, string, []byte, string, string) (string, error) {
	client.started <- struct{}{}
	<-client.release
	return client.etag, client.error
}

func (client *blockingCalendarPushClient) deleteCalendarObject(context.Context, string, string) error {
	return nil
}

func (client *blockingCalendarPushClient) getCalendarObject(context.Context, string) (calDAVCalendarObject, error) {
	return calDAVCalendarObject{}, errCalDAVObjectNotFound
}

func TestSelectedCalendarSwitchAfterPushAuthFailurePreservesNewTarget(t *testing.T) {
	testCalendarPushCompletionAfterTargetSwitch(t, errors.New("caldav put status 401: Unauthorized"))
}

func TestSelectedCalendarSwitchAfterPushSuccessPreservesNewTarget(t *testing.T) {
	testCalendarPushCompletionAfterTargetSwitch(t, nil)
}

func TestCalendarPushWithStaleAccountSnapshotPreservesNewTargetRows(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("stale-account-row-read", "Before Switch")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	accountRead := make(chan struct{})
	releasePush := make(chan struct{})
	pushResult := make(chan error, 1)
	client := &fakeCalDAVPushClient{}
	go func() {
		staleAccount, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
		if errorValue != nil {
			pushResult <- errorValue
			return
		}
		close(accountRead)
		<-releasePush
		_, errorValue = service.pushCalendarOutboxForAccount(ctx, staleAccount, client)
		pushResult <- errorValue
	}()
	<-accountRead
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	latestEvent := event
	latestEvent.Title = "After Switch"
	if errorValue := service.writeCalendarEvent(ctx, latestEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	close(releasePush)
	if errorValue := <-pushResult; errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.putCalls) != 0 || len(client.deleteCalls) != 0 {
		t.Fatalf("stale push performed remote calls: put=%+v delete=%+v", client.putCalls, client.deleteCalls)
	}
	storedAccount, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedAccount.SelectedCalendarID != selectedAccount.SelectedCalendarID || storedAccount.SelectedCalendarURL != selectedAccount.SelectedCalendarURL {
		t.Fatalf("selected calendar reverted: %+v", storedAccount)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != latestEvent.Title {
		t.Fatalf("event title=%q want %q", storedEvent.Title, latestEvent.Title)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	newTargetRows := 0
	for _, row := range rows {
		if normalizeCalendarOutboxTargetURL(row.TargetCalendarURL) == "/calendars/company/" {
			newTargetRows++
		}
	}
	if newTargetRows != 2 {
		t.Fatalf("new target rows=%d want 2 rows=%+v", newTargetRows, rows)
	}
}

func testCalendarPushCompletionAfterTargetSwitch(t *testing.T, pushError error) {
	t.Helper()
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	service.markRemoteCalendarAccountAuthError(ctx, account, errors.New("existing auth error"))
	account, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := newLocalTestCalendarEvent("concurrent-target-switch", "Before Switch")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &blockingCalendarPushClient{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		etag:    `"etag-a"`,
		error:   pushError,
	}
	pushResult := make(chan error, 1)
	go func() {
		_, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- errorValue
	}()
	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		t.Fatal("calendar A push did not start")
	}
	switchResult := startSelectedCalendarSwitch(ctx, service, account)
	earlySwitchResult, switchedEarly := waitForSelectedCalendarSwitch(switchResult, 100*time.Millisecond)
	if switchedEarly {
		close(client.release)
		if earlySwitchResult.error != nil {
			t.Fatal(earlySwitchResult.error)
		}
		t.Fatal("selected calendar switch committed before the in-flight request finished")
	}
	latestEvent := event
	latestEvent.Title = "After Switch"
	if errorValue := service.writeCalendarEvent(ctx, latestEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	close(client.release)
	select {
	case errorValue := <-pushResult:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("calendar A push did not finish")
	}
	selectedResult, completed := waitForSelectedCalendarSwitch(switchResult, 5*time.Second)
	if !completed {
		t.Fatal("selected calendar switch did not finish")
	}
	if selectedResult.error != nil {
		t.Fatal(selectedResult.error)
	}
	selectedAccount := selectedResult.account
	storedAccount, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedAccount.SelectedCalendarID != selectedAccount.SelectedCalendarID || storedAccount.SelectedCalendarURL != selectedAccount.SelectedCalendarURL {
		t.Fatalf("selected calendar reverted: id=%q url=%q", storedAccount.SelectedCalendarID, storedAccount.SelectedCalendarURL)
	}
	if pushError == nil {
		if storedAccount.LastAuthError != "" || storedAccount.LastAuthErrorAt != "" {
			t.Fatalf("successful push restored stale auth error: %+v", storedAccount)
		}
	} else if storedAccount.LastAuthError != pushError.Error() || storedAccount.LastAuthErrorAt == "" {
		t.Fatalf("failed push auth state=%q at %q want %q", storedAccount.LastAuthError, storedAccount.LastAuthErrorAt, pushError.Error())
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	newTargetRows := []calendarOutboxRow{}
	for _, row := range rows {
		if normalizeCalendarOutboxTargetURL(row.TargetCalendarURL) != "/calendars/company/" {
			continue
		}
		newTargetRows = append(newTargetRows, row)
		if row.RemoteHref != "" || row.IfMatchETag != "" {
			t.Fatalf("new target row contaminated by stale push: %+v", row)
		}
	}
	if batches := aggregateCalendarOutboxRows(newTargetRows); len(batches) != 1 {
		t.Fatalf("new target batches=%d want 1 rows=%+v", len(batches), rows)
	}
}
