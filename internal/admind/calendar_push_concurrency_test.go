package admind

import (
	"context"
	"testing"
	"time"
)

type blockingCalendarPutClient struct {
	started chan struct{}
	release chan struct{}
	etag    string
}

type blockingCalendarDeleteConflictClient struct {
	started      chan struct{}
	release      chan struct{}
	remoteObject calDAVCalendarObject
	deleteCalls  []fakeDeleteCall
}

func (client *blockingCalendarDeleteConflictClient) putCalendarObject(ctx context.Context, objectPath string, ics []byte, ifMatch string, ifNoneMatch string) (string, error) {
	return "", nil
}

func (client *blockingCalendarDeleteConflictClient) deleteCalendarObject(ctx context.Context, objectPath string, ifMatch string) error {
	client.deleteCalls = append(client.deleteCalls, fakeDeleteCall{Path: objectPath, IfMatch: ifMatch})
	if len(client.deleteCalls) == 1 {
		return errCalDAVPreconditionFailed
	}
	return nil
}

func (client *blockingCalendarDeleteConflictClient) getCalendarObject(ctx context.Context, objectPath string) (calDAVCalendarObject, error) {
	close(client.started)
	select {
	case <-client.release:
		return client.remoteObject, nil
	case <-ctx.Done():
		return calDAVCalendarObject{}, ctx.Err()
	}
}

func (client *blockingCalendarPutClient) putCalendarObject(ctx context.Context, objectPath string, ics []byte, ifMatch string, ifNoneMatch string) (string, error) {
	close(client.started)
	select {
	case <-client.release:
		return client.etag, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (client *blockingCalendarPutClient) deleteCalendarObject(ctx context.Context, objectPath string, ifMatch string) error {
	return nil
}

func (client *blockingCalendarPutClient) getCalendarObject(ctx context.Context, objectPath string) (calDAVCalendarObject, error) {
	return calDAVCalendarObject{}, errCalDAVObjectNotFound
}

func TestPushCalendarOutboxPreservesLocalEditCreatedDuringRemotePut(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("push-concurrent-edit", "Original")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-original"`
	baselineEvent.RemoteHref = "/calendars/me/push-concurrent-edit.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}

	firstLocalEvent := baselineEvent
	firstLocalEvent.Title = "First Local"
	if errorValue := service.writeCalendarEvent(ctx, firstLocalEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &blockingCalendarPutClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
		etag:    `"etag-first-local"`,
	}
	pushResult := make(chan error, 1)
	go func() {
		_, pushError := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- pushError
	}()
	<-client.started

	latestLocalEvent := firstLocalEvent
	latestLocalEvent.Title = "Latest Local"
	if errorValue := service.writeCalendarEvent(ctx, latestLocalEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	close(client.release)
	if errorValue := <-pushResult; errorValue != nil {
		t.Fatal(errorValue)
	}

	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != "Latest Local" {
		t.Fatalf("title=%q want Latest Local", storedEvent.Title)
	}
	if storedEvent.RemoteETag != client.etag {
		t.Fatalf("remote etag=%q want %q", storedEvent.RemoteETag, client.etag)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("remaining rows=%d error=%v", len(remainingRows), errorValue)
	}
	if remainingRows[0].IfMatchETag != client.etag {
		t.Fatalf("remaining if-match=%q want %q", remainingRows[0].IfMatchETag, client.etag)
	}
}

func TestPushCalendarOutboxPreservesLocalDeleteCreatedDuringRemotePut(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("push-concurrent-delete", "Original")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-original"`
	baselineEvent.RemoteHref = "/calendars/me/push-concurrent-delete.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	localEvent := baselineEvent
	localEvent.Title = "Local Update"
	if errorValue := service.writeCalendarEvent(ctx, localEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &blockingCalendarPutClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
		etag:    `"etag-local-update"`,
	}
	pushResult := make(chan error, 1)
	go func() {
		_, pushError := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- pushError
	}()
	<-client.started

	if errorValue := service.softDeleteCalendarEvent(ctx, baselineEvent.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	close(client.release)
	if errorValue := <-pushResult; errorValue != nil {
		t.Fatal(errorValue)
	}

	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("projection found=%v error=%v", found, errorValue)
	}
	if !projection.IsDeleted {
		t.Fatal("event should remain deleted")
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("remaining rows=%d error=%v", len(remainingRows), errorValue)
	}
	if remainingRows[0].Operation != calendarOutboxOperationDelete {
		t.Fatalf("remaining operation=%q", remainingRows[0].Operation)
	}
	if remainingRows[0].IfMatchETag != client.etag {
		t.Fatalf("remaining if-match=%q want %q", remainingRows[0].IfMatchETag, client.etag)
	}

	remoteEvent := localEvent
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, parseCalendarConflictTime(projection.DeletedAt).Add(time.Minute))
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{{
		Path: baselineEvent.RemoteHref,
		ETag: client.etag,
		Data: remoteICS,
	}}, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	projection, found, errorValue = service.readCalendarEventProjectionByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("projection after pull found=%v error=%v", found, errorValue)
	}
	if !projection.IsDeleted {
		t.Fatal("own PUT observed by pull should not restore the locally deleted event")
	}
	remainingRows, errorValue = service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 || remainingRows[0].Operation != calendarOutboxOperationDelete {
		t.Fatalf("remaining rows after pull=%+v error=%v", remainingRows, errorValue)
	}
}

func TestPushCalendarOutboxPreservesLocalEditCreatedDuringDeleteConflictFetch(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("delete-conflict-concurrent-edit", "Original")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-original"`
	baselineEvent.RemoteHref = "/calendars/me/delete-conflict-concurrent-edit.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, baselineEvent.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	deletedAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_events SET deleted_at = ? WHERE id = ?`, deletedAt.Format(time.RFC3339Nano), baselineEvent.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteEvent := baselineEvent
	remoteEvent.Title = "Remote Edit"
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, deletedAt.Add(time.Minute))
	client := &blockingCalendarDeleteConflictClient{
		started: make(chan struct{}),
		release: make(chan struct{}),
		remoteObject: calDAVCalendarObject{
			Path: baselineEvent.RemoteHref,
			ETag: `"etag-remote-edit"`,
			Data: remoteICS,
		},
	}
	pushResult := make(chan error, 1)
	go func() {
		_, pushError := service.pushCalendarOutboxForAccount(ctx, account, client)
		pushResult <- pushError
	}()
	<-client.started

	latestLocalEvent := baselineEvent
	latestLocalEvent.Title = "Latest Local"
	if errorValue := service.writeCalendarEvent(ctx, latestLocalEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	close(client.release)
	if errorValue := <-pushResult; errorValue != nil {
		t.Fatal(errorValue)
	}

	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != "Latest Local" {
		t.Fatalf("title=%q want Latest Local", storedEvent.Title)
	}
	if storedEvent.RemoteETag != client.remoteObject.ETag {
		t.Fatalf("remote etag=%q want %q", storedEvent.RemoteETag, client.remoteObject.ETag)
	}
	remainingRows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(remainingRows) != 1 {
		t.Fatalf("remaining rows=%d error=%v", len(remainingRows), errorValue)
	}
	if remainingRows[0].Operation != calendarOutboxOperationPut {
		t.Fatalf("remaining operation=%q", remainingRows[0].Operation)
	}
	if remainingRows[0].IfMatchETag != client.remoteObject.ETag {
		t.Fatalf("remaining if-match=%q want %q", remainingRows[0].IfMatchETag, client.remoteObject.ETag)
	}
}

func TestRemoteDeleteReconciliationDoesNotDeleteNewerLocalRevision(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("remote-delete-newer-local", "Original")
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	snapshot, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("snapshot found=%v error=%v", found, errorValue)
	}
	latestEvent := snapshot
	latestEvent.Title = "Latest Local"
	if errorValue := service.writeCalendarEventWithSource(ctx, latestEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}

	deleted, errorValue := service.softDeleteCalendarEventIfRevisionMatches(ctx, snapshot, calendarSourcePull)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if deleted {
		t.Fatal("stale remote deletion should not delete a newer local revision")
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != "Latest Local" {
		t.Fatalf("title=%q want Latest Local", storedEvent.Title)
	}
}

func TestCalendarEventPersistenceDoesNotOverwriteMattermostProjection(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	event := newLocalTestCalendarEvent("mattermost-projection-race", "Original")
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	staleEvent := event
	if errorValue := service.updateCalendarEventMattermostPostID(ctx, event.ID, "new-post-id"); errorValue != nil {
		t.Fatal(errorValue)
	}
	staleEvent.Title = "Remote Update"
	if errorValue := service.writeCalendarEventWithSource(ctx, staleEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}

	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("stored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.MattermostPostID != "new-post-id" {
		t.Fatalf("Mattermost post ID=%q want new-post-id", storedEvent.MattermostPostID)
	}
	if storedEvent.Title != "Remote Update" {
		t.Fatalf("title=%q want Remote Update", storedEvent.Title)
	}
}
