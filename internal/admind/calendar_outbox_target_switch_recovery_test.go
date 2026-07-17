package admind

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

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
