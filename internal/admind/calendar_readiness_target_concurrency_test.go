package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarReadinessUpdateWithStaleAccountSnapshotPreservesNewTarget(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "personal@example.com", "Personal", "writer", "/calendars/personal/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := newLocalTestCalendarEvent("stale-readiness-target", "Before Switch")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	updateStarted := make(chan struct{})
	updateRelease := make(chan struct{})
	updateFinished := make(chan struct{})
	go func() {
		close(updateStarted)
		<-updateRelease
		service.markSelectedCalendarReadinessStatusIfCurrent(ctx, selectedAccount, selectedAccount.SelectedCalendarID, calendarReadinessStatusCalendarInaccessible)
		close(updateFinished)
	}()
	<-updateStarted
	newSelectedAccount, errorValue := service.saveSelectedCalendar(ctx, selectedAccount, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	latestEvent := event
	latestEvent.Title = "After Switch"
	if errorValue := service.writeCalendarEvent(ctx, latestEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	close(updateRelease)
	select {
	case <-updateFinished:
	case <-time.After(5 * time.Second):
		t.Fatal("stale readiness update did not finish")
	}
	storedAccount, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedAccount.SelectedCalendarID != newSelectedAccount.SelectedCalendarID || storedAccount.SelectedCalendarURL != newSelectedAccount.SelectedCalendarURL {
		t.Fatalf("selected account mutated by stale readiness update: %+v", storedAccount)
	}
	if storedAccount.SelectedCalendarReadinessStatus != calendarReadinessStatusInitialSyncPending {
		t.Fatalf("readiness status=%q want %q", storedAccount.SelectedCalendarReadinessStatus, calendarReadinessStatusInitialSyncPending)
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
		if canonicalCalendarTargetURL(row.TargetCalendarURL) == "/calendars/company/" {
			newTargetRows++
		}
	}
	if newTargetRows == 0 {
		t.Fatalf("new target backfill rows missing: %+v", rows)
	}
}
