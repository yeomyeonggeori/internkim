package admind

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

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
	if errorValue := service.softDeleteMissingRemoteEvents(ctx, account.ID, activeRemoteCalendarTarget(account), allEvents, emptyRemoteUIDs, nil); errorValue != nil {
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
