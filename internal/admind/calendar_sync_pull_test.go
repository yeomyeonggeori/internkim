package admind

import (
	"context"
	"testing"
	"time"
)

func TestRunGoogleCalendarPullInitialDiscoveryAndUpsert(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedRemoteCalendarAccountForPull(t, service, "user@example.com")

	client := &fakeCalDAVPullClient{
		principalURL: "/calendars/user@example.com/user",
		homeSetURL:   "/calendars/user@example.com/",
		calendars: []calDAVCalendarInfo{{
			Path: "/calendars/user@example.com/events/",
			Name: "Primary",
		}},
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "evt-a@google", `"etag-a"`, "Meeting A"),
			fakeRemoteObject(t, "evt-b@google", `"etag-b"`, "Meeting B"),
			fakeRemoteObject(t, "evt-c@google", `"etag-c"`, "Meeting C"),
		},
	}

	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("pull: %v", errorValue)
	}
	if client.principalCalls != 0 || client.homeSetCalls != 0 || client.listCalls != 0 {
		t.Errorf("google flow should bypass PROPFIND-based discovery: principal=%d, homeSet=%d, list=%d",
			client.principalCalls, client.homeSetCalls, client.listCalls)
	}
	if client.queryCalls != 1 {
		t.Errorf("query calls: got %d", client.queryCalls)
	}
	stored, errorValue := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read stored: %v", errorValue)
	}
	if len(stored) != 3 {
		t.Fatalf("stored events: got %d, want 3", len(stored))
	}
	for _, event := range stored {
		if event.RemoteSource != remoteCalendarProviderGoogle {
			t.Errorf("RemoteSource for %s: got %q", event.UID, event.RemoteSource)
		}
		if event.RemoteETag == "" {
			t.Errorf("RemoteETag empty for %s", event.UID)
		}
	}
	refreshed, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("reload account: %v", errorValue)
	}
	expectedCalendarURL := googleCalDAVBaseURL + "/user@example.com/events/"
	if refreshed.DefaultCalendarURL != expectedCalendarURL {
		t.Errorf("DefaultCalendarURL: got %q, want %q", refreshed.DefaultCalendarURL, expectedCalendarURL)
	}
	if refreshed.PrincipalURL == "" || refreshed.HomeSetURL == "" {
		t.Errorf("principal/home-set not persisted: %+v", refreshed)
	}
}

func TestRunGoogleCalendarPullStoresRemoteObservationState(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	modifiedAt := time.Date(2026, 7, 15, 1, 2, 3, 0, time.UTC)
	event := newLocalTestCalendarEvent("remote-observation", "Observed")
	remoteObject := calDAVCalendarObject{
		Path: activeRemoteCalendarTarget(account).CalendarURL + event.UID + ".ics",
		ETag: `"etag-observed"`,
		Data: encodeCalendarTestEventWithLastModified(t, event, modifiedAt),
	}
	client := &fakeCalDAVPullClient{ctag: "observed-ctag", objects: []calDAVCalendarObject{remoteObject}}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	state, found, errorValue := service.readCalendarRemoteEventState(ctx, account.ID, activeRemoteCalendarTarget(account).CalendarURL, event.UID)
	if errorValue != nil || !found {
		t.Fatalf("remote event state: found=%v error=%v", found, errorValue)
	}
	if state.RemoteModifiedAt != modifiedAt.Format(time.RFC3339Nano) {
		t.Fatalf("remote modified at=%q", state.RemoteModifiedAt)
	}
	if state.LastSeenAt == "" || state.MissingDetectedAt != "" {
		t.Fatalf("observation state=%+v", state)
	}
}

func TestRunGoogleCalendarPullSkipsDiscoveryAndSameETagWrites(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedRemoteCalendarAccountForPull(t, service, "user@example.com")
	account.PrincipalURL = "/principal"
	account.HomeSetURL = "/home/"
	account.DefaultCalendarURL = "/calendars/me/"
	updated, errorValue := service.upsertRemoteCalendarAccount(ctx, account)
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}

	client := &fakeCalDAVPullClient{
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "evt-x@google", `"etag-x"`, "Workshop X"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, updated, client); errorValue != nil {
		t.Fatalf("first pull: %v", errorValue)
	}
	if client.principalCalls != 0 || client.homeSetCalls != 0 || client.listCalls != 0 {
		t.Errorf("discovery should be skipped: principal=%d, homeSet=%d, list=%d",
			client.principalCalls, client.homeSetCalls, client.listCalls)
	}
	firstEvents, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(firstEvents) != 1 {
		t.Fatalf("after first pull: got %d events", len(firstEvents))
	}
	originalUpdated := firstEvents[0].UpdatedAt

	time.Sleep(10 * time.Millisecond)
	if _, errorValue := service.runGoogleCalendarPull(ctx, updated, client); errorValue != nil {
		t.Fatalf("second pull: %v", errorValue)
	}
	secondEvents, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(secondEvents) != 1 {
		t.Fatalf("after second pull: got %d events", len(secondEvents))
	}
	if secondEvents[0].UpdatedAt != originalUpdated {
		t.Errorf("UpdatedAt changed despite same ETag: %q -> %q", originalUpdated, secondEvents[0].UpdatedAt)
	}
}

func TestRunGoogleCalendarPullInsertsNewRemoteEvent(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	client := &fakeCalDAVPullClient{
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "evt-1@google", `"etag-1"`, "First"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("first pull: %v", errorValue)
	}

	client.objects = append(client.objects,
		fakeRemoteObject(t, "evt-2@google", `"etag-2"`, "Second"))
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("second pull: %v", errorValue)
	}
	stored, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(stored) != 2 {
		t.Fatalf("after add: got %d events, want 2", len(stored))
	}
}

func TestRunGoogleCalendarPullUpdatesEventOnETagChange(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	client := &fakeCalDAVPullClient{
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "evt-mut@google", `"etag-v1"`, "Old Title"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("first pull: %v", errorValue)
	}
	first, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(first) != 1 {
		t.Fatalf("first pull events: %d", len(first))
	}
	originalID := first[0].ID

	client.objects = []calDAVCalendarObject{
		fakeRemoteObject(t, "evt-mut@google", `"etag-v2"`, "New Title"),
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("second pull: %v", errorValue)
	}
	second, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(second) != 1 {
		t.Fatalf("after update: %d events", len(second))
	}
	if second[0].ID != originalID {
		t.Errorf("event ID should be stable across updates: %q -> %q", originalID, second[0].ID)
	}
	if second[0].Title != "New Title" {
		t.Errorf("title not updated: got %q", second[0].Title)
	}
	if second[0].RemoteETag != `"etag-v2"` {
		t.Errorf("ETag not updated: got %q", second[0].RemoteETag)
	}
}

func TestRunGoogleCalendarPullSoftDeletesMissingRemoteEvent(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	client := &fakeCalDAVPullClient{
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "evt-keep@google", `"etag-keep"`, "Keep me"),
			fakeRemoteObject(t, "evt-drop@google", `"etag-drop"`, "Drop me"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("seed pull: %v", errorValue)
	}
	all, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(all) != 2 {
		t.Fatalf("seed pull events: %d", len(all))
	}

	backdateCalendarEventUpdatedAt(t, service, ctx, "evt-drop@google", -10*time.Minute)

	client.objects = []calDAVCalendarObject{
		fakeRemoteObject(t, "evt-keep@google", `"etag-keep"`, "Keep me"),
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("delete pull: %v", errorValue)
	}
	remaining, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(remaining) != 1 {
		t.Fatalf("after delete: got %d active events, want 1", len(remaining))
	}
	if remaining[0].UID != "evt-keep@google" {
		t.Errorf("wrong event survived: %q", remaining[0].UID)
	}
}

func TestRunGoogleCalendarPullProtectsCurrentCyclePushedEvent(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	client := &fakeCalDAVPullClient{
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "evt-fresh@google", `"etag-1"`, "Fresh"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("seed pull: %v", errorValue)
	}

	protectedUIDs := map[string]struct{}{"evt-fresh@google": {}}
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, nil, protectedUIDs); errorValue != nil {
		t.Fatalf("missing reconcile: %v", errorValue)
	}
	active, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(active) != 1 {
		t.Fatalf("recently-pushed event was soft-deleted: %d remain", len(active))
	}
	if active[0].UID != "evt-fresh@google" {
		t.Errorf("wrong event survived: %q", active[0].UID)
	}
}

func TestRecentlyPushedCalendarUIDProtectionPersistsUntilObserved(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	now := time.Unix(1000, 0).UTC()

	event := newLocalTestCalendarEvent("recent-push", "Recent Push")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-pushed"`
	event.RemoteHref = "/calendars/me/recent-push.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed: %v", errorValue)
	}

	service.recordRecentlyPushedCalendarUIDs(now, map[string]struct{}{event.UID: {}})
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, nil, service.recentlyPushedCalendarUIDs(now.Add(time.Minute))); errorValue != nil {
		t.Fatalf("missing pull while protected: %v", errorValue)
	}
	if _, found, _ := service.readCalendarEventByID(ctx, event.ID); !found {
		t.Fatal("recently pushed event should survive missing remote LIST")
	}

	observedObject := fakeRemoteObject(t, event.UID, `"etag-observed"`, event.Title)
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{observedObject}, service.recentlyPushedCalendarUIDs(now.Add(2*time.Minute))); errorValue != nil {
		t.Fatalf("observed pull: %v", errorValue)
	}
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, nil, service.recentlyPushedCalendarUIDs(now.Add(3*time.Minute))); errorValue != nil {
		t.Fatalf("missing pull after observed: %v", errorValue)
	}
	if _, found, _ := service.readCalendarEventByID(ctx, event.ID); found {
		t.Fatal("observed then deleted Google event should be removed locally")
	}
}

func TestRunGoogleCalendarPullResurrectsSoftDeletedRowOnUIDMatch(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	client := &fakeCalDAVPullClient{
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, "evt-orphan@google", `"etag-1"`, "Orphan"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("seed pull: %v", errorValue)
	}
	seeded, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(seeded) != 1 {
		t.Fatalf("seed pull events: %d", len(seeded))
	}
	originalID := seeded[0].ID

	if errorValue := service.softDeleteCalendarEventWithSource(ctx, originalID, calendarSourcePull); errorValue != nil {
		t.Fatalf("soft delete: %v", errorValue)
	}

	client.objects = []calDAVCalendarObject{
		fakeRemoteObject(t, "evt-orphan@google", `"etag-2"`, "Resurrected"),
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("resurrect pull: %v", errorValue)
	}
	revived, _ := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if len(revived) != 1 {
		t.Fatalf("after resurrect: got %d active events, want 1", len(revived))
	}
	if revived[0].ID != originalID {
		t.Errorf("event ID changed across resurrect: %q -> %q", originalID, revived[0].ID)
	}
	if revived[0].Title != "Resurrected" {
		t.Errorf("title not updated: %q", revived[0].Title)
	}
}

func backdateCalendarEventUpdatedAt(t *testing.T, service *Service, ctx context.Context, uid string, offset time.Duration) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatalf("open db: %v", errorValue)
	}
	defer database.Close()
	when := time.Now().UTC().Add(offset).Format(time.RFC3339Nano)
	if _, errorValue := database.ExecContext(ctx, "UPDATE calendar_events SET updated_at = ? WHERE uid = ?", when, uid); errorValue != nil {
		t.Fatalf("backdate %s: %v", uid, errorValue)
	}
}
