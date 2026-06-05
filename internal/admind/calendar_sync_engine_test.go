package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
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

func TestServeCalendarAccountStatusReportsDisconnected(t *testing.T) {
	service := newCalendarTestService(t)
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if body.Connected {
		t.Error("Connected should be false without account")
	}
	if body.NeedsReauth {
		t.Error("NeedsReauth should be false when disconnected")
	}
}

func TestServeCalendarAccountStatusReportsConnectedHealthy(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	_ = ctx
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if !body.Connected {
		t.Error("Connected should be true")
	}
	if body.Provider != remoteCalendarProviderGoogle {
		t.Errorf("Provider: got %q", body.Provider)
	}
	if body.NeedsReauth {
		t.Error("NeedsReauth should be false without auth error")
	}
	if body.LastAuthError != "" {
		t.Errorf("LastAuthError should be empty: %q", body.LastAuthError)
	}
}

func TestServeCalendarAccountStatusFlagsReauthOnAuthError(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	service.markRemoteCalendarAccountAuthError(ctx, account, errors.New("invalid_grant: expired"))
	request := httptest.NewRequest(http.MethodGet, "http://x/calendar/api/account-status", nil)
	recorder := httptest.NewRecorder()
	service.serveCalendarAccountStatus(recorder, request)
	var body calendarAccountStatusResponse
	if errorValue := json.Unmarshal(recorder.Body.Bytes(), &body); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if !body.NeedsReauth {
		t.Error("NeedsReauth should be true when last_auth_error set")
	}
	if !strings.Contains(body.LastAuthError, "invalid_grant") {
		t.Errorf("LastAuthError: got %q", body.LastAuthError)
	}
	if body.LastAuthErrorAt == "" {
		t.Error("LastAuthErrorAt should be populated")
	}
}

func TestMarkRemoteCalendarAccountAuthErrorPersistsMessage(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	service.markRemoteCalendarAccountAuthError(ctx, account, errors.New("invalid_grant: token expired"))
	reloaded, found, _ := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if !found {
		t.Fatal("account missing after mark")
	}
	if reloaded.LastAuthError == "" {
		t.Error("LastAuthError empty")
	}
	if !strings.Contains(reloaded.LastAuthError, "invalid_grant") {
		t.Errorf("LastAuthError content: got %q", reloaded.LastAuthError)
	}
	if reloaded.LastAuthErrorAt == "" {
		t.Error("LastAuthErrorAt empty")
	}
}

func TestClearRemoteCalendarAccountAuthErrorWipesMarker(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.LastAuthError = "previous failure"
	account.LastAuthErrorAt = "2026-05-15T10:00:00Z"
	saved, _ := service.upsertRemoteCalendarAccount(ctx, account)
	service.clearRemoteCalendarAccountAuthError(ctx, saved)
	reloaded, _, _ := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if reloaded.LastAuthError != "" {
		t.Errorf("LastAuthError should be cleared: %q", reloaded.LastAuthError)
	}
	if reloaded.LastAuthErrorAt != "" {
		t.Errorf("LastAuthErrorAt should be cleared: %q", reloaded.LastAuthErrorAt)
	}
}

func TestIsCalendarAuthErrorDetectsOAuthFailures(t *testing.T) {
	cases := []struct {
		message  string
		expected bool
	}{
		{"oauth2: cannot fetch token: 401 Unauthorized", true},
		{"invalid_grant: Token has been expired or revoked.", true},
		{"caldav put status 401: Unauthorized", true},
		{"caldav ctag propfind status 403: ...", false},
		{"network unreachable", false},
		{"", false},
	}
	for _, testCase := range cases {
		var errValue error
		if testCase.message != "" {
			errValue = errors.New(testCase.message)
		}
		got := isCalendarAuthError(errValue)
		if got != testCase.expected {
			t.Errorf("isCalendarAuthError(%q): got %v, want %v", testCase.message, got, testCase.expected)
		}
	}
}

func TestPullGoogleCalendarChangesNoOpWhenAccountMissing(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	if _, errorValue := service.pullGoogleCalendarChanges(ctx); errorValue != nil {
		t.Errorf("expected no error when no account exists, got %v", errorValue)
	}
}

func TestDecodeRemoteCalendarObjectMarksProvenance(t *testing.T) {
	object := fakeRemoteObject(t, "evt-p@google", `"etag-p"`, "Provenance Check")
	event, errorValue := decodeRemoteCalendarObject(object, "user@example.com")
	if errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}
	if event.RemoteSource != remoteCalendarProviderGoogle {
		t.Errorf("RemoteSource: got %q", event.RemoteSource)
	}
	if event.RemoteETag != `"etag-p"` {
		t.Errorf("RemoteETag: got %q", event.RemoteETag)
	}
	if event.RemoteHref != object.Path {
		t.Errorf("RemoteHref: got %q", event.RemoteHref)
	}
	if event.RawICS == "" {
		t.Error("RawICS empty")
	}
}

func TestSoftDeleteMissingRemoteEventsDeletesAllUntrackedGoogleEvents(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	recentEvent := newLocalTestCalendarEvent("recent", "Recent")
	recentEvent.RemoteSource = remoteCalendarProviderGoogle
	recentEvent.RemoteETag = `"e1"`
	recentEvent.RemoteHref = "/calendars/me/recent.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, recentEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed recent: %v", errorValue)
	}

	farFutureEvent := calendarEvent{
		ID:                "far-future",
		UID:               "far-future@internkim",
		Title:             "Far future",
		StartISO:          "2030-01-01T00:00:00Z",
		EndISO:            "2030-01-01T01:00:00Z",
		TimeZone:          "UTC",
		Color:             "#000000",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
		RemoteSource:      remoteCalendarProviderGoogle,
		RemoteETag:        `"e2"`,
		RemoteHref:        "/calendars/me/far-future.ics",
	}
	if errorValue := service.writeCalendarEventWithSource(ctx, farFutureEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed far future: %v", errorValue)
	}

	allEvents, _ := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	emptyRemoteUIDs := map[string]struct{}{}
	if errorValue := service.softDeleteMissingRemoteEvents(ctx, account.ID, allEvents, emptyRemoteUIDs, nil); errorValue != nil {
		t.Fatalf("softDelete: %v", errorValue)
	}

	_, recentFound, _ := service.readCalendarEventByID(ctx, recentEvent.ID)
	if recentFound {
		t.Error("recent google-source event missing from remote response should be soft-deleted")
	}
	_, farFutureFound, _ := service.readCalendarEventByID(ctx, farFutureEvent.ID)
	if farFutureFound {
		t.Error("far-future google-source event missing from remote response should also be soft-deleted (no time-window protection)")
	}
}

func TestRunGoogleCalendarPullReusesLocalEventIDOnUIDMatch(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	localEvent := newLocalTestCalendarEvent("local-roundtrip", "Local Roundtrip")
	if errorValue := service.writeCalendarEvent(ctx, localEvent); errorValue != nil {
		t.Fatalf("write local: %v", errorValue)
	}

	client := &fakeCalDAVPullClient{
		ctag: "ctag-rt",
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, localEvent.UID, `"server-etag"`, "Local Roundtrip"),
		},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("pull on UID roundtrip must not fail with UNIQUE: %v", errorValue)
	}
	stored, found, errorValue := service.readCalendarEventByID(ctx, localEvent.ID)
	if errorValue != nil {
		t.Fatalf("read: %v", errorValue)
	}
	if !found {
		t.Fatal("original ID not preserved after pull match")
	}
	if stored.RemoteSource != remoteCalendarProviderGoogle {
		t.Errorf("RemoteSource should be marked: got %q", stored.RemoteSource)
	}
	if stored.RemoteETag != `"server-etag"` {
		t.Errorf("RemoteETag: got %q", stored.RemoteETag)
	}
	allEvents, _ := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	matches := 0
	for _, event := range allEvents {
		if event.UID == localEvent.UID {
			matches++
		}
	}
	if matches != 1 {
		t.Errorf("expected 1 row for UID, got %d (UNIQUE violation)", matches)
	}
}

func TestRunGoogleCalendarPullDoesNotSoftDeleteLocalOnlyEvents(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	localOnly := newLocalTestCalendarEvent("local-only", "Local Only")
	if errorValue := service.writeCalendarEvent(ctx, localOnly); errorValue != nil {
		t.Fatalf("write local-only: %v", errorValue)
	}
	client := &fakeCalDAVPullClient{
		ctag:    "ctag-empty",
		objects: []calDAVCalendarObject{},
	}
	if _, errorValue := service.runGoogleCalendarPull(ctx, account, client); errorValue != nil {
		t.Fatalf("pull: %v", errorValue)
	}
	_, found, errorValue := service.readCalendarEventByID(ctx, localOnly.ID)
	if errorValue != nil {
		t.Fatalf("read: %v", errorValue)
	}
	if !found {
		t.Fatal("local-only event must survive pull with empty remote response")
	}
}

func TestRunGoogleCalendarPullRequiresAccountEmailForDiscovery(t *testing.T) {
	service := newCalendarTestService(t)
	account := seedRemoteCalendarAccountForPull(t, service, "")
	client := &fakeCalDAVPullClient{}
	_, errorValue := service.runGoogleCalendarPull(context.Background(), account, client)
	if errorValue == nil {
		t.Fatal("expected discovery error when account email is empty")
	}
}

func seedRemoteCalendarAccountForPull(t *testing.T, service *Service, email string) remoteCalendarAccount {
	t.Helper()
	account := remoteCalendarAccount{
		ID:           "google-test",
		Provider:     remoteCalendarProviderGoogle,
		AccountEmail: email,
	}
	saved, errorValue := service.upsertRemoteCalendarAccount(context.Background(), account)
	if errorValue != nil {
		t.Fatalf("seed account: %v", errorValue)
	}
	return saved
}

func seedAccountWithDiscovery(t *testing.T, service *Service) remoteCalendarAccount {
	t.Helper()
	account := seedRemoteCalendarAccountForPull(t, service, "user@example.com")
	account.PrincipalURL = "/principal"
	account.HomeSetURL = "/home/"
	account.DefaultCalendarURL = "/calendars/me/"
	saved, errorValue := service.upsertRemoteCalendarAccount(context.Background(), account)
	if errorValue != nil {
		t.Fatalf("seed account discovery: %v", errorValue)
	}
	return saved
}

func fakeRemoteObject(t *testing.T, uid string, etag string, title string) calDAVCalendarObject {
	t.Helper()
	start := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)
	event := calendarEvent{
		ID:                "remote-" + uid,
		UID:               uid,
		Title:             title,
		StartISO:          start.Format(time.RFC3339),
		EndISO:            end.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#10b981",
		ReminderLeadHours: 24,
		CreatedByEmail:    "ignored@example.com",
	}
	calendar, errorValue := calendarObjectForEvent(event)
	if errorValue != nil {
		t.Fatalf("calendarObjectForEvent: %v", errorValue)
	}
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(calendar); errorValue != nil {
		t.Fatalf("encode: %v", errorValue)
	}
	return calDAVCalendarObject{
		Path: "/calendars/user@example.com/events/" + uid + ".ics",
		ETag: etag,
		Data: buffer.Bytes(),
	}
}

func TestWriteCalendarEventLocalEnqueuesPutOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("local-new-1", "Local New")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, "")
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("outbox rows: got %d, want 1", len(rows))
	}
	if rows[0].Operation != calendarOutboxOperationPut {
		t.Errorf("operation: got %q", rows[0].Operation)
	}
	if rows[0].EventID != event.ID {
		t.Errorf("event id: got %q", rows[0].EventID)
	}
	if rows[0].RemoteHref != "" {
		t.Errorf("remote href should be empty for new event: %q", rows[0].RemoteHref)
	}
}

func TestWriteCalendarEventFromPullSkipsOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("pull-1", "Pulled")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-1"`
	event.RemoteHref = "/calendars/me/pull-1.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("write pull: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 0 {
		t.Errorf("pull source should not enqueue, got %d rows", len(rows))
	}
}

func TestWriteCalendarEventLocalSkipsOutboxWhenAccountMissing(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("orphan-1", "No account")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 0 {
		t.Errorf("should not enqueue without account, got %d rows", len(rows))
	}
}

func TestSoftDeleteCalendarEventLocalEnqueuesDeleteOutbox(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("local-del", "Local Delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-del"`
	event.RemoteHref = "/calendars/me/local-del.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed write: %v", errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatalf("soft delete: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 1 {
		t.Fatalf("outbox rows: got %d, want 1", len(rows))
	}
	if rows[0].Operation != calendarOutboxOperationDelete {
		t.Errorf("operation: got %q", rows[0].Operation)
	}
	if rows[0].RemoteHref != event.RemoteHref {
		t.Errorf("remote href: got %q", rows[0].RemoteHref)
	}
	if rows[0].IfMatchETag != event.RemoteETag {
		t.Errorf("if-match etag: got %q", rows[0].IfMatchETag)
	}
}

func TestPushCalendarOutboxCreatesNewObject(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-new", "Push New")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	client := &fakeCalDAVPushClient{
		putETags: map[string]string{
			account.DefaultCalendarURL + event.UID + ".ics": `"etag-server-1"`,
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("put calls: got %d", len(client.putCalls))
	}
	call := client.putCalls[0]
	if call.IfNoneMatch != caldavWildcardETag {
		t.Errorf("If-None-Match: got %q, want %q", call.IfNoneMatch, caldavWildcardETag)
	}
	if call.IfMatch != "" {
		t.Errorf("If-Match should be empty for create: got %q", call.IfMatch)
	}
	remaining, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(remaining) != 0 {
		t.Errorf("outbox should be empty after success: %d", len(remaining))
	}
	stored, _, _ := service.readCalendarEventByID(ctx, event.ID)
	if stored.RemoteSource != remoteCalendarProviderGoogle {
		t.Errorf("RemoteSource: got %q", stored.RemoteSource)
	}
	if stored.RemoteETag != `"etag-server-1"` {
		t.Errorf("RemoteETag: got %q", stored.RemoteETag)
	}
	if stored.RemoteHref != call.Path {
		t.Errorf("RemoteHref: got %q, want %q", stored.RemoteHref, call.Path)
	}
}

func TestPushCalendarOutboxUpdatesObjectWithIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-upd", "Push Update")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-old"`
	event.RemoteHref = "/calendars/me/push-upd.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed write: %v", errorValue)
	}
	event.Title = "Push Update modified"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("local update: %v", errorValue)
	}
	client := &fakeCalDAVPushClient{
		putETags: map[string]string{event.RemoteHref: `"etag-new"`},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.putCalls) != 1 {
		t.Fatalf("put calls: got %d", len(client.putCalls))
	}
	if client.putCalls[0].IfMatch != `"etag-old"` {
		t.Errorf("If-Match: got %q", client.putCalls[0].IfMatch)
	}
	if client.putCalls[0].IfNoneMatch != "" {
		t.Errorf("If-None-Match should be empty on update: got %q", client.putCalls[0].IfNoneMatch)
	}
	stored, _, _ := service.readCalendarEventByID(ctx, event.ID)
	if stored.RemoteETag != `"etag-new"` {
		t.Errorf("RemoteETag: got %q", stored.RemoteETag)
	}
}

func TestPushCalendarOutboxDeletesObjectWithIfMatch(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-del", "Push Delete")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-del"`
	event.RemoteHref = "/calendars/me/push-del.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed write: %v", errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatalf("soft delete: %v", errorValue)
	}
	client := &fakeCalDAVPushClient{}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.deleteCalls) != 1 {
		t.Fatalf("delete calls: got %d", len(client.deleteCalls))
	}
	if client.deleteCalls[0].Path != event.RemoteHref {
		t.Errorf("delete path: got %q", client.deleteCalls[0].Path)
	}
	if client.deleteCalls[0].IfMatch != event.RemoteETag {
		t.Errorf("If-Match: got %q", client.deleteCalls[0].IfMatch)
	}
}

func TestPushCalendarOutboxClearsAuthErrorAfterDeleteSuccess(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	service.markRemoteCalendarAccountAuthError(ctx, account, errors.New("caldav delete status 401: Unauthorized"))
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}

	event := newLocalTestCalendarEvent("push-del-auth-clear", "Push Delete Auth Clear")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-del-auth-clear"`
	event.RemoteHref = "/calendars/me/push-del-auth-clear.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed write: %v", errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatalf("soft delete: %v", errorValue)
	}

	client := &fakeCalDAVPushClient{}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, reloaded, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	refreshed, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read refreshed account: %v", errorValue)
	}
	if refreshed.LastAuthError != "" || refreshed.LastAuthErrorAt != "" {
		t.Fatalf("auth error should clear after delete success: %+v", refreshed)
	}
}

func TestPushCalendarOutboxKeepsAuthErrorAfterNoopRow(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	service.markRemoteCalendarAccountAuthError(ctx, account, errors.New("caldav put status 401: Unauthorized"))
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID: account.ID,
		EventID:   "missing-event",
		EventUID:  "missing-event@internkim",
		Operation: calendarOutboxOperationPut,
	}); errorValue != nil {
		t.Fatalf("enqueue outbox: %v", errorValue)
	}

	client := &fakeCalDAVPushClient{}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, reloaded, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	refreshed, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read refreshed account: %v", errorValue)
	}
	if refreshed.LastAuthError == "" || refreshed.LastAuthErrorAt == "" {
		t.Fatalf("auth error should remain after noop row: %+v", refreshed)
	}
	if len(client.putCalls) != 0 || len(client.deleteCalls) != 0 {
		t.Fatalf("noop row should not call remote client: put=%d delete=%d", len(client.putCalls), len(client.deleteCalls))
	}
}

func TestPushCalendarOutboxDropsRowAfterMaxAttempts(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-cap", "Hit attempt cap")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, "")
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("seed rows: %d", len(rows))
	}
	for index := 0; index < calendarOutboxMaxAttempts; index++ {
		if errorValue := service.markCalendarOutboxAttempt(ctx, rows[0].ID, "simulated failure"); errorValue != nil {
			t.Fatalf("mark attempt %d: %v", index, errorValue)
		}
	}
	client := &fakeCalDAVPushClient{}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(client.putCalls) != 0 {
		t.Errorf("client should not be called when attempt cap reached, got %d put calls", len(client.putCalls))
	}
	remaining, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(remaining) != 0 {
		t.Errorf("outbox row should be dropped after cap, got %d remaining", len(remaining))
	}
}

func TestPushCalendarOutboxKeepsRowOnTransientError(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-transient", "Transient")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
	client := &fakeCalDAVPushClient{
		putErrors: map[string]error{expectedPath: errors.New("network unreachable")},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 1 {
		t.Fatalf("transient error should keep outbox row, got %d", len(rows))
	}
	if rows[0].AttemptCount != 1 {
		t.Errorf("attempt count: got %d", rows[0].AttemptCount)
	}
	if !strings.Contains(rows[0].LastError, "network unreachable") {
		t.Errorf("last error not captured: %q", rows[0].LastError)
	}
}

func TestPushCalendarOutboxMarksAccountAuthErrorOnCalDAVUnauthorized(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("push-auth", "Auth Failure")
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("write: %v", errorValue)
	}
	expectedPath := account.DefaultCalendarURL + event.UID + ".ics"
	client := &fakeCalDAVPushClient{
		putErrors: map[string]error{expectedPath: errors.New("caldav put status 401: Unauthorized")},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	reloaded, _, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		t.Fatalf("read account: %v", errorValue)
	}
	if !strings.Contains(reloaded.LastAuthError, "401") {
		t.Fatalf("last auth error: got %q", reloaded.LastAuthError)
	}
	if reloaded.LastAuthErrorAt == "" {
		t.Fatal("last auth error timestamp should be set")
	}
}

func newLocalTestCalendarEvent(idSuffix string, title string) calendarEvent {
	start := time.Now().UTC().Add(3 * time.Hour).Truncate(time.Second)
	end := start.Add(time.Hour)
	return calendarEvent{
		ID:                idSuffix,
		UID:               idSuffix + "@internkim",
		Title:             title,
		StartISO:          start.Format(time.RFC3339),
		EndISO:            end.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
	}
}

type fakeCalDAVPushClient struct {
	putETags       map[string]string
	putErrors      map[string]error
	putErrorsQueue map[string][]error
	deleteErrors   map[string]error
	getObjects     map[string]calDAVCalendarObject
	getErrors      map[string]error

	putCalls    []fakePutCall
	deleteCalls []fakeDeleteCall
	getCalls    []string
}

type fakePutCall struct {
	Path        string
	IfMatch     string
	IfNoneMatch string
	Data        []byte
}

type fakeDeleteCall struct {
	Path    string
	IfMatch string
}

func (client *fakeCalDAVPushClient) putCalendarObject(ctx context.Context, objectPath string, ics []byte, ifMatch string, ifNoneMatch string) (string, error) {
	client.putCalls = append(client.putCalls, fakePutCall{
		Path:        objectPath,
		IfMatch:     ifMatch,
		IfNoneMatch: ifNoneMatch,
		Data:        append([]byte(nil), ics...),
	})
	if client.putErrorsQueue != nil {
		if queue, present := client.putErrorsQueue[objectPath]; present && len(queue) > 0 {
			next := queue[0]
			client.putErrorsQueue[objectPath] = queue[1:]
			if next != nil {
				return "", next
			}
		}
	}
	if client.putErrors != nil {
		if errorValue, present := client.putErrors[objectPath]; present {
			return "", errorValue
		}
	}
	if client.putETags != nil {
		if etag, present := client.putETags[objectPath]; present {
			return etag, nil
		}
	}
	return "", nil
}

func (client *fakeCalDAVPushClient) deleteCalendarObject(ctx context.Context, objectPath string, ifMatch string) error {
	client.deleteCalls = append(client.deleteCalls, fakeDeleteCall{Path: objectPath, IfMatch: ifMatch})
	if client.deleteErrors != nil {
		if errorValue, present := client.deleteErrors[objectPath]; present {
			return errorValue
		}
	}
	return nil
}

func (client *fakeCalDAVPushClient) getCalendarObject(ctx context.Context, objectPath string) (calDAVCalendarObject, error) {
	client.getCalls = append(client.getCalls, objectPath)
	if client.getErrors != nil {
		if errorValue, present := client.getErrors[objectPath]; present {
			return calDAVCalendarObject{}, errorValue
		}
	}
	if client.getObjects != nil {
		if object, present := client.getObjects[objectPath]; present {
			return object, nil
		}
	}
	return calDAVCalendarObject{}, errCalDAVPreconditionFailed
}

type fakeCalDAVPullClient struct {
	principalURL           string
	homeSetURL             string
	calendars              []calDAVCalendarInfo
	objects                []calDAVCalendarObject
	ctag                   string
	discoverPrincipalError error

	principalCalls int
	homeSetCalls   int
	listCalls      int
	ctagCalls      int
	queryCalls     int
}

func (client *fakeCalDAVPullClient) fetchCalendarCTag(ctx context.Context, calendarPath string) (string, error) {
	client.ctagCalls++
	return client.ctag, nil
}

func (client *fakeCalDAVPullClient) discoverPrincipalURL(ctx context.Context) (string, error) {
	client.principalCalls++
	if client.discoverPrincipalError != nil {
		return "", client.discoverPrincipalError
	}
	return client.principalURL, nil
}

func (client *fakeCalDAVPullClient) discoverHomeSetURL(ctx context.Context, principalURL string) (string, error) {
	client.homeSetCalls++
	return client.homeSetURL, nil
}

func (client *fakeCalDAVPullClient) listCalendars(ctx context.Context, homeSetURL string) ([]calDAVCalendarInfo, error) {
	client.listCalls++
	return client.calendars, nil
}

func (client *fakeCalDAVPullClient) queryAllCalendarEvents(ctx context.Context, calendarPath string) ([]calDAVCalendarObject, error) {
	client.queryCalls++
	return client.objects, nil
}
