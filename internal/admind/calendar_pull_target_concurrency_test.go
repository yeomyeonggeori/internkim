package admind

import (
	"context"
	"testing"
	"time"
)

type blockingCalendarPullClient struct {
	queryStarted chan struct{}
	queryRelease chan struct{}
	ctag         string
	objects      []calDAVCalendarObject
}

func (client *blockingCalendarPullClient) discoverPrincipalURL(context.Context) (string, error) {
	return "", nil
}

func (client *blockingCalendarPullClient) discoverHomeSetURL(context.Context, string) (string, error) {
	return "", nil
}

func (client *blockingCalendarPullClient) listCalendars(context.Context, string) ([]calDAVCalendarInfo, error) {
	return nil, nil
}

func (client *blockingCalendarPullClient) fetchCalendarCTag(context.Context, string) (string, error) {
	return client.ctag, nil
}

func (client *blockingCalendarPullClient) queryAllCalendarEvents(context.Context, string) ([]calDAVCalendarObject, error) {
	close(client.queryStarted)
	<-client.queryRelease
	return client.objects, nil
}

func TestCalendarPullDiscardsStaleSnapshotAfterTargetSwitch(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("stale-pull-target", "Before Switch")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = "/calendars/me/stale-pull-target.ics"
	event.RemoteETag = `"etag-a"`
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteObject := fakeRemoteObject(t, event.UID, `"etag-a-new"`, "Stale A Remote")
	remoteObject.Path = event.RemoteHref
	client := &blockingCalendarPullClient{
		queryStarted: make(chan struct{}),
		queryRelease: make(chan struct{}),
		ctag:         `"ctag-a"`,
		objects:      []calDAVCalendarObject{remoteObject},
	}
	pullResult := make(chan error, 1)
	go func() {
		_, errorValue := service.runGoogleCalendarPull(ctx, account, client)
		pullResult <- errorValue
	}()
	select {
	case <-client.queryStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("calendar A pull query did not start")
	}
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	latestEvent := event
	latestEvent.Title = "After Switch"
	if errorValue := service.writeCalendarEvent(ctx, latestEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	close(client.queryRelease)
	select {
	case errorValue := <-pullResult:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("calendar A pull did not finish")
	}
	storedAccount, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedAccount.SelectedCalendarID != selectedAccount.SelectedCalendarID || storedAccount.SelectedCalendarURL != selectedAccount.SelectedCalendarURL || storedAccount.DefaultCalendarCTag != "" {
		t.Fatalf("selected account mutated by stale pull: %+v", storedAccount)
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
	if newTargetRows != 2 {
		t.Fatalf("new target rows=%d want 2 rows=%+v", newTargetRows, rows)
	}
	if remoteState, found, errorValue := service.readCalendarRemoteEventState(ctx, account.ID, account.DefaultCalendarURL, event.UID); errorValue != nil || found {
		t.Fatalf("stale remote state found=%v state=%+v error=%v", found, remoteState, errorValue)
	}
}
