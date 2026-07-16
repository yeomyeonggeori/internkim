package admind

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
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

func TestSelectedCalendarSwitchStopsRemainingBatchesAfterInFlightRequest(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	firstEvent := newLocalTestCalendarEvent("stale-loop-first", "First Before Switch")
	secondEvent := newLocalTestCalendarEvent("stale-loop-second", "Second Before Switch")
	if errorValue := service.writeCalendarEvent(ctx, firstEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.writeCalendarEvent(ctx, secondEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &multiBatchBlockingCalendarPushClient{
		firstStarted: make(chan struct{}),
		firstRelease: make(chan struct{}),
	}
	pushResult := make(chan error, 1)
	go func() {
		_, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- errorValue
	}()
	select {
	case <-client.firstStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("first calendar A request did not start")
	}
	switchResult := startSelectedCalendarSwitch(ctx, service, account)
	earlySwitchResult, switchedEarly := waitForSelectedCalendarSwitch(switchResult, 100*time.Millisecond)
	if switchedEarly {
		close(client.firstRelease)
		if earlySwitchResult.error != nil {
			t.Fatal(earlySwitchResult.error)
		}
		t.Fatal("selected calendar switch committed before the first request finished")
	}
	latestSecondEvent := secondEvent
	latestSecondEvent.Title = "Second After Switch"
	if errorValue := service.writeCalendarEvent(ctx, latestSecondEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	close(client.firstRelease)
	select {
	case errorValue := <-pushResult:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("calendar A push loop did not finish")
	}
	selectedResult, completed := waitForSelectedCalendarSwitch(switchResult, 5*time.Second)
	if !completed {
		t.Fatal("selected calendar switch did not finish")
	}
	if selectedResult.error != nil {
		t.Fatal(selectedResult.error)
	}
	selectedAccount := selectedResult.account
	if callCount := client.putCallCount(); callCount != 1 {
		t.Fatalf("calendar A PUT calls=%d want only the in-flight request", callCount)
	}
	storedAccount, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedAccount.SelectedCalendarID != selectedAccount.SelectedCalendarID || storedAccount.SelectedCalendarURL != selectedAccount.SelectedCalendarURL {
		t.Fatalf("selected calendar reverted: %+v", storedAccount)
	}
	storedSecondEvent, found, errorValue := service.readCalendarEventByID(ctx, secondEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("second event found=%v error=%v", found, errorValue)
	}
	if storedSecondEvent.Title != latestSecondEvent.Title {
		t.Fatalf("second event title=%q want %q", storedSecondEvent.Title, latestSecondEvent.Title)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	newTargetUIDs := map[string]struct{}{}
	for _, row := range rows {
		if canonicalCalendarTargetURL(row.TargetCalendarURL) == "/calendars/company/" {
			newTargetUIDs[row.EventUID] = struct{}{}
		}
	}
	if _, found := newTargetUIDs[firstEvent.UID]; !found {
		t.Fatalf("first event new-target outbox missing: %+v", rows)
	}
	if _, found := newTargetUIDs[secondEvent.UID]; !found {
		t.Fatalf("second event new-target outbox missing: %+v", rows)
	}
}

func TestSelectedCalendarSwitchWaitsForConflictRecoveryAfterPut(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("stale-conflict-retry", "Before Switch")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &multiBatchBlockingCalendarPushClient{
		firstStarted: make(chan struct{}),
		firstRelease: make(chan struct{}),
		firstError:   errCalDAVPreconditionFailed,
	}
	pushResult := make(chan error, 1)
	go func() {
		_, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- errorValue
	}()
	<-client.firstStarted
	switchResult := startSelectedCalendarSwitch(ctx, service, account)
	earlySwitchResult, switchedEarly := waitForSelectedCalendarSwitch(switchResult, 100*time.Millisecond)
	if switchedEarly {
		close(client.firstRelease)
		if earlySwitchResult.error != nil {
			t.Fatal(earlySwitchResult.error)
		}
		t.Fatal("selected calendar switch committed before conflict recovery finished")
	}
	close(client.firstRelease)
	if errorValue := <-pushResult; errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedResult, completed := waitForSelectedCalendarSwitch(switchResult, 5*time.Second)
	if !completed {
		t.Fatal("selected calendar switch did not finish")
	}
	if selectedResult.error != nil {
		t.Fatal(selectedResult.error)
	}
	if client.putCallCount() != 1 || client.getCallCount() != 1 || client.deleteCallCount() != 0 {
		t.Fatalf("conflict recovery calls put=%d get=%d delete=%d", client.putCallCount(), client.getCallCount(), client.deleteCallCount())
	}
}

func TestSelectedCalendarSwitchSkipsLaterDeleteAfterInFlightPut(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	putEvent := newLocalTestCalendarEvent("stale-put-before-delete", "PUT Before Switch")
	if errorValue := service.writeCalendarEvent(ctx, putEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	deletedEvent := newLocalTestCalendarEvent("stale-delete-after-put", "DELETE Before Switch")
	deletedEvent.RemoteSource = remoteCalendarProviderGoogle
	deletedEvent.RemoteHref = "/calendars/me/stale-delete-after-put.ics"
	deletedEvent.RemoteETag = `"etag-delete"`
	if errorValue := service.writeCalendarEventWithSource(ctx, deletedEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, deletedEvent.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &multiBatchBlockingCalendarPushClient{
		firstStarted: make(chan struct{}),
		firstRelease: make(chan struct{}),
	}
	pushResult := make(chan error, 1)
	go func() {
		_, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- errorValue
	}()
	<-client.firstStarted
	switchResult := startSelectedCalendarSwitch(ctx, service, account)
	earlySwitchResult, switchedEarly := waitForSelectedCalendarSwitch(switchResult, 100*time.Millisecond)
	if switchedEarly {
		close(client.firstRelease)
		if earlySwitchResult.error != nil {
			t.Fatal(earlySwitchResult.error)
		}
		t.Fatal("selected calendar switch committed before the first PUT finished")
	}
	close(client.firstRelease)
	if errorValue := <-pushResult; errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedResult, completed := waitForSelectedCalendarSwitch(switchResult, 5*time.Second)
	if !completed {
		t.Fatal("selected calendar switch did not finish")
	}
	if selectedResult.error != nil {
		t.Fatal(selectedResult.error)
	}
	if client.putCallCount() != 1 || client.deleteCallCount() != 0 || client.getCallCount() != 0 {
		t.Fatalf("stale later calls put=%d get=%d delete=%d", client.putCallCount(), client.getCallCount(), client.deleteCallCount())
	}
}

func TestSelectedCalendarSwitchWaitsForETagRecoveryAfterPut(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("stale-etag-recovery", "Before Switch")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	putStarted := make(chan struct{})
	putRelease := make(chan struct{})
	var getCallCount atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		switch request.Method {
		case http.MethodPut:
			close(putStarted)
			<-putRelease
			responseWriter.WriteHeader(http.StatusNoContent)
		case http.MethodGet:
			getCallCount.Add(1)
			responseWriter.Header().Set("ETag", `"etag-recovered"`)
			_, _ = responseWriter.Write([]byte("calendar"))
		default:
			responseWriter.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()
	client, errorValue := newOutboundCalDAVClient(server.URL, server.Client())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	pushResult := make(chan error, 1)
	go func() {
		_, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- errorValue
	}()
	<-putStarted
	switchResult := startSelectedCalendarSwitch(ctx, service, account)
	earlySwitchResult, switchedEarly := waitForSelectedCalendarSwitch(switchResult, 100*time.Millisecond)
	if switchedEarly {
		close(putRelease)
		if earlySwitchResult.error != nil {
			t.Fatal(earlySwitchResult.error)
		}
		t.Fatal("selected calendar switch committed before ETag recovery finished")
	}
	close(putRelease)
	if errorValue := <-pushResult; errorValue != nil {
		t.Fatal(errorValue)
	}
	selectedResult, completed := waitForSelectedCalendarSwitch(switchResult, 5*time.Second)
	if !completed {
		t.Fatal("selected calendar switch did not finish")
	}
	if selectedResult.error != nil {
		t.Fatal(selectedResult.error)
	}
	if getCallCount.Load() != 1 {
		t.Fatalf("ETag recovery GET calls=%d want 1", getCallCount.Load())
	}
}

func TestCalendarPutTargetChangeBeforeHTTPDoesNotLeaveObservationFence(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("stale-fence-before-http", "Before Switch")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%d error=%v", len(rows), errorValue)
	}
	releasePush := make(chan struct{})
	pushFinished := make(chan error, 1)
	client := &fakeCalDAVPushClient{}
	go func() {
		<-releasePush
		_, errorValue := service.pushCalendarOutboxPut(ctx, account, client, rows[0])
		pushFinished <- errorValue
	}()
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	close(releasePush)
	if errorValue := <-pushFinished; !errors.Is(errorValue, errCalendarPushTargetChanged) {
		t.Fatalf("stale push error=%v", errorValue)
	}
	if len(client.putCalls) != 0 {
		t.Fatalf("stale PUT calls=%+v", client.putCalls)
	}
	fencedUIDs, errorValue := service.listCalendarPushObservationFenceUIDs(ctx, account.ID, account.DefaultCalendarURL)
	if errorValue != nil || len(fencedUIDs) != 0 {
		t.Fatalf("stale target fences=%+v error=%v", fencedUIDs, errorValue)
	}
	reselectedAccount, errorValue := service.saveSelectedCalendar(ctx, selectedAccount, "personal@example.com", "Personal", "writer", account.DefaultCalendarURL, time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	reselectedAccount.DefaultCalendarCTag = `"same-ctag"`
	reselectedAccount.InitialSyncCompletedAt = time.Now().UTC().Format(time.RFC3339Nano)
	reselectedAccount.SelectedCalendarReadinessStatus = calendarReadinessStatusSyncReady
	reselectedAccount, errorValue = service.upsertRemoteCalendarAccount(ctx, reselectedAccount)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	pullClient := &fakeCalDAVPullClient{ctag: reselectedAccount.DefaultCalendarCTag}
	changed, errorValue := service.runGoogleCalendarPull(ctx, reselectedAccount, pullClient)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if changed || pullClient.queryCalls != 0 {
		t.Fatalf("reselected target pull changed=%v query_calls=%d", changed, pullClient.queryCalls)
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
	newTargetRows := 0
	for _, row := range rows {
		if normalizeCalendarOutboxTargetURL(row.TargetCalendarURL) != "/calendars/company/" {
			continue
		}
		newTargetRows++
		if row.RemoteHref != "" || row.IfMatchETag != "" {
			t.Fatalf("new target row contaminated by stale push: %+v", row)
		}
	}
	if newTargetRows != 1 {
		t.Fatalf("new target rows=%d want 1 rows=%+v", newTargetRows, rows)
	}
}
