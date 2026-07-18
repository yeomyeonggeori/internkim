package admind

import (
	"context"
	"testing"
	"time"
)

type selectedCalendarSaveResult struct {
	account remoteCalendarAccount
	error   error
}

type blockingCalendarDeleteClient struct {
	started chan struct{}
	release chan struct{}
	object  calDAVCalendarObject
}

func (client *blockingCalendarDeleteClient) putCalendarObject(context.Context, string, []byte, string, string) (string, error) {
	return "", nil
}

func (client *blockingCalendarDeleteClient) deleteCalendarObject(context.Context, string, string) error {
	close(client.started)
	<-client.release
	return nil
}

func (client *blockingCalendarDeleteClient) getCalendarObject(context.Context, string) (calDAVCalendarObject, error) {
	return client.object, nil
}

type blockingCalendarUIDQueryClient struct {
	*fakeCalDAVPushClient
	started chan struct{}
	release chan struct{}
}

func (client *blockingCalendarUIDQueryClient) queryCalendarObjectsByUID(context.Context, string, string) ([]calDAVCalendarObject, error) {
	close(client.started)
	<-client.release
	return nil, nil
}

func TestSelectedCalendarSwitchWaitsForInFlightPutWithoutBlockingLocalWrites(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("target-switch-put", "Before Switch")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &blockingCalendarPushClient{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
		etag:    `"etag-a"`,
	}
	pushResult := make(chan error, 1)
	go func() {
		_, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- errorValue
	}()
	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		t.Fatal("calendar PUT did not start")
	}
	switchResult := make(chan selectedCalendarSaveResult, 1)
	go func() {
		selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
		switchResult <- selectedCalendarSaveResult{account: selectedAccount, error: errorValue}
	}()
	waitForCalendarSwitchWaiter(t, service)
	var earlySwitchResult *selectedCalendarSaveResult
	select {
	case result := <-switchResult:
		earlySwitchResult = &result
	default:
	}
	localWriteResult := make(chan error, 1)
	go func() {
		localEvent := newLocalTestCalendarEvent("target-switch-local-write", "Local Write")
		localWriteResult <- service.writeCalendarEvent(ctx, localEvent)
	}()
	select {
	case errorValue := <-localWriteResult:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		close(client.release)
		t.Fatal("local calendar write was blocked by remote PUT")
	}
	close(client.release)
	select {
	case errorValue := <-pushResult:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("calendar PUT did not finish")
	}
	if earlySwitchResult != nil {
		if earlySwitchResult.error != nil {
			t.Fatal(earlySwitchResult.error)
		}
		t.Fatal("selected calendar switch committed before the in-flight PUT finished")
	}
	select {
	case result := <-switchResult:
		if result.error != nil {
			t.Fatal(result.error)
		}
		if result.account.SelectedCalendarURL != "/calendars/company/" {
			t.Fatalf("selected calendar URL=%q", result.account.SelectedCalendarURL)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("selected calendar switch did not finish after the PUT")
	}
}

func TestSelectedCalendarSwitchWaitsForInFlightDeleteCleanup(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	eventUID := "target-switch-delete"
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID:         account.ID,
		EventID:           eventUID,
		EventUID:          eventUID,
		Operation:         calendarOutboxOperationDelete,
		RemoteHref:        account.DefaultCalendarURL + eventUID + ".ics",
		IfMatchETag:       `"etag-delete"`,
		TargetCalendarURL: account.DefaultCalendarURL,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, eventUID)
	remoteObject := fakeRemoteObject(t, eventUID, `"etag-delete"`, "Target Switch Delete")
	remoteObject.Path = account.DefaultCalendarURL + eventUID + ".ics"
	client := &blockingCalendarDeleteClient{started: make(chan struct{}), release: make(chan struct{}), object: remoteObject}
	pushResult := make(chan error, 1)
	go func() {
		_, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- errorValue
	}()
	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		t.Fatal("calendar DELETE did not start")
	}
	switchResult := startSelectedCalendarSwitch(ctx, service, account)
	waitForCalendarSwitchWaiter(t, service)
	earlyResult, completedEarly := waitForSelectedCalendarSwitch(switchResult, 0)
	close(client.release)
	waitForCalendarPush(t, pushResult, "calendar DELETE")
	if completedEarly {
		if earlyResult.error != nil {
			t.Fatal(earlyResult.error)
		}
		t.Fatal("selected calendar switch committed before DELETE cleanup finished")
	}
	selectedResult, completed := waitForSelectedCalendarSwitch(switchResult, 5*time.Second)
	if !completed {
		t.Fatal("selected calendar switch did not finish after DELETE cleanup")
	}
	if selectedResult.error != nil {
		t.Fatal(selectedResult.error)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rows) != 0 {
		t.Fatalf("remaining outbox rows=%+v error=%v", rows, errorValue)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[eventUID]; found {
		t.Fatal("completed DELETE left an observation fence")
	}
}

func TestSelectedCalendarSwitchWaitsForInFlightUIDQueryCleanup(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	eventUID := "target-switch-uid-query"
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID:         account.ID,
		EventID:           eventUID,
		EventUID:          eventUID,
		Operation:         calendarOutboxOperationDelete,
		TargetCalendarURL: account.DefaultCalendarURL,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	seedCalendarPushObservationFenceForTest(t, service, account.ID, account.DefaultCalendarURL, eventUID)
	canonicalPath := account.DefaultCalendarURL + eventUID + ".ics"
	client := &blockingCalendarUIDQueryClient{
		fakeCalDAVPushClient: &fakeCalDAVPushClient{getErrors: map[string]error{canonicalPath: errCalDAVObjectNotFound}},
		started:              make(chan struct{}),
		release:              make(chan struct{}),
	}
	pushResult := make(chan error, 1)
	go func() {
		_, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- errorValue
	}()
	select {
	case <-client.started:
	case <-time.After(5 * time.Second):
		t.Fatal("calendar UID query did not start")
	}
	switchResult := startSelectedCalendarSwitch(ctx, service, account)
	waitForCalendarSwitchWaiter(t, service)
	earlyResult, completedEarly := waitForSelectedCalendarSwitch(switchResult, 0)
	close(client.release)
	waitForCalendarPush(t, pushResult, "calendar UID query")
	if completedEarly {
		if earlyResult.error != nil {
			t.Fatal(earlyResult.error)
		}
		t.Fatal("selected calendar switch committed before UID query cleanup finished")
	}
	selectedResult, completed := waitForSelectedCalendarSwitch(switchResult, 5*time.Second)
	if !completed {
		t.Fatal("selected calendar switch did not finish after UID query cleanup")
	}
	if selectedResult.error != nil {
		t.Fatal(selectedResult.error)
	}
	if len(client.getCalls) != 1 || client.getCalls[0] != canonicalPath {
		t.Fatalf("calendar object GET calls=%v, want [%s]", client.getCalls, canonicalPath)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rows) != 0 {
		t.Fatalf("remaining outbox rows=%+v error=%v", rows, errorValue)
	}
	fencedUIDs := readCalendarPushObservationFenceUIDsForTest(t, service, account.ID, account.DefaultCalendarURL)
	if _, found := fencedUIDs[eventUID]; found {
		t.Fatal("authoritative UID absence left an observation fence")
	}
}

func TestRemoteMutationDoesNotStartWhileTargetSwitchIsWaiting(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("waiting-target-switch", "Waiting Switch")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%+v error=%v", rows, errorValue)
	}
	client := &fakeCalDAVPushClient{}
	service.calendarSwitchWaiters.Add(1)
	defer service.calendarSwitchWaiters.Add(-1)
	result, errorValue := service.executeCalendarOutboxRemoteMutation(ctx, account, client, rows[0])
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !result.shouldStop {
		t.Fatal("remote mutation did not yield to the waiting target switch")
	}
	if len(client.putCalls) != 0 || len(client.getCalls) != 0 || len(client.deleteCalls) != 0 {
		t.Fatalf("remote calls started while target switch waited: put=%+v get=%+v delete=%+v", client.putCalls, client.getCalls, client.deleteCalls)
	}
}

func startSelectedCalendarSwitch(ctx context.Context, service *Service, account remoteCalendarAccount) <-chan selectedCalendarSaveResult {
	result := make(chan selectedCalendarSaveResult, 1)
	go func() {
		selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
		result <- selectedCalendarSaveResult{account: selectedAccount, error: errorValue}
	}()
	return result
}

func waitForSelectedCalendarSwitch(result <-chan selectedCalendarSaveResult, timeout time.Duration) (selectedCalendarSaveResult, bool) {
	if timeout == 0 {
		select {
		case selectedResult := <-result:
			return selectedResult, true
		default:
			return selectedCalendarSaveResult{}, false
		}
	}
	select {
	case selectedResult := <-result:
		return selectedResult, true
	case <-time.After(timeout):
		return selectedCalendarSaveResult{}, false
	}
}

func waitForCalendarSwitchWaiter(t *testing.T, service *Service) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for service.calendarSwitchWaiters.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("selected calendar switch did not wait for the remote mutation")
		}
		time.Sleep(time.Millisecond)
	}
}

func waitForCalendarPush(t *testing.T, result <-chan error, operationName string) {
	t.Helper()
	select {
	case errorValue := <-result:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not finish", operationName)
	}
}
