package admind

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/emersion/go-webdav"
)

type countingCalendarProvider struct {
	name        string
	builds      int
	discoveries int
}

func (provider *countingCalendarProvider) Name() string {
	return provider.name
}

func (provider *countingCalendarProvider) Endpoint(account remoteCalendarAccount) string {
	return "https://calendar.example.com/"
}

func (provider *countingCalendarProvider) BuildHTTPClient(ctx context.Context, service *Service, account remoteCalendarAccount) (webdav.HTTPClient, error) {
	provider.builds++
	return http.DefaultClient, nil
}

func (provider *countingCalendarProvider) Discover(ctx context.Context, service *Service, account remoteCalendarAccount) (remoteCalendarAccount, error) {
	provider.discoveries++
	account.DefaultCalendarURL = "/calendars/default/"
	return service.upsertRemoteCalendarAccount(ctx, account)
}

func TestPullCalendarChangesForProviderSkipsBeforeCalendarSelection(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                 "account-google-1",
		Provider:           "counting",
		AccountEmail:       "user@example.com",
		DefaultCalendarURL: "/calendars/default/",
	}); errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	provider := &countingCalendarProvider{name: "counting"}

	changed, errorValue := service.pullCalendarChangesForProvider(ctx, provider, false, nil)
	if errorValue != nil {
		t.Fatalf("pull: %v", errorValue)
	}
	if changed {
		t.Fatal("pull should report no changes before calendar selection")
	}
	if provider.builds != 0 || provider.discoveries != 0 {
		t.Fatalf("provider should not be touched before calendar selection: builds=%d discoveries=%d", provider.builds, provider.discoveries)
	}
}

func TestPushPendingCalendarOutboxSkipsBeforeCalendarSelection(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account, errorValue := service.upsertRemoteCalendarAccount(ctx, remoteCalendarAccount{
		ID:                 "account-google-1",
		Provider:           "counting",
		AccountEmail:       "user@example.com",
		DefaultCalendarURL: "/calendars/default/",
	})
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	event := newLocalTestCalendarEvent("pending-before-selection", "Pending Before Selection")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write event: %v", errorValue)
	}
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID:     account.ID,
		EventID:       event.ID,
		EventUID:      event.UID,
		Operation:     calendarOutboxOperationPut,
		ChangedFields: calendarAllUserEditableFields(),
	}); errorValue != nil {
		t.Fatalf("enqueue outbox: %v", errorValue)
	}
	provider := &countingCalendarProvider{name: "counting"}

	pushedUIDs, errorValue := service.pushPendingCalendarOutboxForProvider(ctx, provider)
	if errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(pushedUIDs) != 0 {
		t.Fatalf("pushed UIDs before selection: %+v", pushedUIDs)
	}
	if provider.builds != 0 || provider.discoveries != 0 {
		t.Fatalf("provider should not be touched before calendar selection: builds=%d discoveries=%d", provider.builds, provider.discoveries)
	}
}

func TestRunGoogleCalendarPullUsesSelectedCalendarTarget(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	selectedCalendarURL := "/calendars/company/"
	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", selectedCalendarURL, time.Now())
	if errorValue != nil {
		t.Fatalf("select calendar: %v", errorValue)
	}

	client := &fakeCalDAVPullClient{
		ctag: `"selected-ctag"`,
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "evt-selected@google", `"etag-selected"`, "Selected"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, client); errorValue != nil {
		t.Fatalf("pull: %v", errorValue)
	}
	if len(client.ctagPaths) != 1 || client.ctagPaths[0] != selectedCalendarURL {
		t.Fatalf("ctag paths: got %+v, want [%s]", client.ctagPaths, selectedCalendarURL)
	}
	if len(client.queryPaths) != 1 || client.queryPaths[0] != selectedCalendarURL {
		t.Fatalf("query paths: got %+v, want [%s]", client.queryPaths, selectedCalendarURL)
	}
	refreshed, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}
	if refreshed.InitialSyncCompletedAt == "" {
		t.Fatal("InitialSyncCompletedAt should be set after selected calendar pull")
	}
	if refreshed.DefaultCalendarCTag != `"selected-ctag"` {
		t.Errorf("DefaultCalendarCTag: got %q", refreshed.DefaultCalendarCTag)
	}
}

func TestRunGoogleCalendarPullSkipsMissingRemoteDeleteBeforeInitialSyncCompletes(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	selectedCalendarEvent := newLocalTestCalendarEvent("selected-initial-missing", "Selected Initial Missing")
	selectedCalendarEvent.RemoteSource = remoteCalendarProviderGoogle
	selectedCalendarEvent.RemoteETag = `"etag-selected-initial-missing"`
	selectedCalendarEvent.RemoteHref = "/calendars/company/selected-initial-missing.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, selectedCalendarEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed selected calendar event: %v", errorValue)
	}

	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatalf("select calendar: %v", errorValue)
	}
	client := &fakeCalDAVPullClient{}
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, client); errorValue != nil {
		t.Fatalf("pull: %v", errorValue)
	}

	if _, found, errorValue := service.readCalendarEventByID(ctx, selectedCalendarEvent.ID); errorValue != nil {
		t.Fatalf("read selected calendar event: %v", errorValue)
	} else if !found {
		t.Fatal("missing event should remain before initial sync completes")
	}
	refreshed, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}
	if refreshed.InitialSyncCompletedAt == "" {
		t.Fatal("InitialSyncCompletedAt should be set after protected initial sync")
	}
}

func TestRunGoogleCalendarPullPreservesEventsFromOtherCalendarTargets(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	otherCalendarEvent := newLocalTestCalendarEvent("other-calendar", "Other Calendar")
	otherCalendarEvent.RemoteSource = remoteCalendarProviderGoogle
	otherCalendarEvent.RemoteETag = `"etag-other"`
	otherCalendarEvent.RemoteHref = "/calendars/me/other-calendar.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, otherCalendarEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed other calendar event: %v", errorValue)
	}

	selectedCalendarEvent := newLocalTestCalendarEvent("selected-missing", "Selected Missing")
	selectedCalendarEvent.RemoteSource = remoteCalendarProviderGoogle
	selectedCalendarEvent.RemoteETag = `"etag-selected-missing"`
	selectedCalendarEvent.RemoteHref = "/calendars/company/selected-missing.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, selectedCalendarEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed selected calendar event: %v", errorValue)
	}

	selectedAccount, errorValue := service.saveSelectedCalendar(ctx, account, "company@example.com", "Company", "writer", "/calendars/company/", time.Now())
	if errorValue != nil {
		t.Fatalf("select calendar: %v", errorValue)
	}
	selectedAccount, errorValue = service.saveCalendarPullState(ctx, selectedAccount, selectedAccount.DefaultCalendarCTag, time.Now(), true)
	if errorValue != nil {
		t.Fatalf("mark initial sync completed: %v", errorValue)
	}
	client := &fakeCalDAVPullClient{}
	if _, errorValue := service.runGoogleCalendarPull(ctx, selectedAccount, client); errorValue != nil {
		t.Fatalf("pull: %v", errorValue)
	}

	if _, found, errorValue := service.readCalendarEventByID(ctx, otherCalendarEvent.ID); errorValue != nil {
		t.Fatalf("read other calendar event: %v", errorValue)
	} else if !found {
		t.Fatal("event from another calendar target should remain")
	}
	if _, found, errorValue := service.readCalendarEventByID(ctx, selectedCalendarEvent.ID); errorValue != nil {
		t.Fatalf("read selected calendar event: %v", errorValue)
	} else if found {
		t.Fatal("missing event from selected calendar target should be soft-deleted")
	}
}

func TestRemoteCalendarEventBelongsToTargetNormalizesURLForms(t *testing.T) {
	event := calendarEvent{
		RemoteHref: "/caldav/v2/user@example.com/company/events/event-1.ics",
	}
	target := remoteCalendarTarget{
		CalendarURL:        "https://apidata.googleusercontent.com/caldav/v2/user@example.com/company/events/",
		IsSelectedCalendar: true,
	}
	if !remoteCalendarEventBelongsToTarget(event, target) {
		t.Fatal("path-only remote href should match absolute selected calendar URL")
	}

	event.RemoteHref = "https://apidata.googleusercontent.com/caldav/v2/user@example.com/company/events/event-2.ics"
	target.CalendarURL = "/caldav/v2/user@example.com/company/events/"
	if !remoteCalendarEventBelongsToTarget(event, target) {
		t.Fatal("absolute remote href should match path-only selected calendar URL")
	}
}

func TestRemoteCalendarEventBelongsToTargetPreservesEscapedSelectedCalendar(t *testing.T) {
	event := calendarEvent{
		RemoteHref: "/caldav/v2/company%2Fschedule%23shared@example.com/events/event-1.ics",
	}
	target := remoteCalendarTarget{
		CalendarURL:        "https://apidata.googleusercontent.com/caldav/v2/company%2Fschedule%23shared@example.com/events/",
		IsSelectedCalendar: true,
	}
	if !remoteCalendarEventBelongsToTarget(event, target) {
		t.Fatal("escaped path-only remote href should match escaped selected calendar URL")
	}
}
