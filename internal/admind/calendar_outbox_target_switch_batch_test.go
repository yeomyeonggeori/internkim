package admind

import (
	"context"
	"testing"
	"time"
)

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
