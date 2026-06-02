package admind

import (
	"context"
	"testing"
	"time"
)

func TestCalendarSyncEndToEndRoundTrip(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	localEvent := newLocalTestCalendarEvent("e2e-roundtrip", "End to End")
	if errorValue := service.writeCalendarEvent(ctx, localEvent); errorValue != nil {
		t.Fatalf("local write: %v", errorValue)
	}

	const issuedETag = `"e2e-server-etag"`
	expectedPath := account.DefaultCalendarURL + localEvent.UID + ".ics"
	pushClient := &fakeCalDAVPushClient{
		putETags: map[string]string{expectedPath: issuedETag},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, pushClient); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}
	if len(pushClient.putCalls) != 1 {
		t.Fatalf("expected exactly 1 PUT, got %d", len(pushClient.putCalls))
	}
	if pushClient.putCalls[0].IfNoneMatch != caldavWildcardETag {
		t.Errorf("first push should be create with If-None-Match=*: got %q", pushClient.putCalls[0].IfNoneMatch)
	}
	afterPush, _, _ := service.readCalendarEventByID(ctx, localEvent.ID)
	if afterPush.RemoteSource != remoteCalendarProviderGoogle {
		t.Errorf("after push RemoteSource: got %q", afterPush.RemoteSource)
	}
	if afterPush.RemoteETag != issuedETag {
		t.Errorf("after push RemoteETag: got %q", afterPush.RemoteETag)
	}

	pullClient := &fakeCalDAVPullClient{
		ctag: "ctag-after-push",
		objects: []calDAVCalendarObject{
			fakeRemoteObject(t, localEvent.UID, issuedETag, "End to End"),
		},
	}
	reloaded, _, _ := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if _, errorValue := service.runGoogleCalendarPull(ctx, reloaded, pullClient); errorValue != nil {
		t.Fatalf("pull after push must not fail with UNIQUE: %v", errorValue)
	}
	stored, found, _ := service.readCalendarEventByID(ctx, localEvent.ID)
	if !found {
		t.Fatal("original ID not preserved after pull reconcile")
	}
	if stored.RemoteETag != issuedETag {
		t.Errorf("RemoteETag after roundtrip: got %q", stored.RemoteETag)
	}

	allEvents, _ := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	matchCount := 0
	for _, event := range allEvents {
		if event.UID == localEvent.UID {
			matchCount++
		}
	}
	if matchCount != 1 {
		t.Errorf("expected exactly 1 row for UID after roundtrip, got %d", matchCount)
	}
}
