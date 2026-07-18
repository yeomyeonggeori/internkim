package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPushCalendarDeleteRecoversStaleHrefByUIDBeforeDeleting(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event, deletedAt := seedRecoverableCalendarDelete(t, service, "stale-href-local-delete", account.DefaultCalendarURL+"stale.ics")
	movedObject := recoveredCalendarDeleteObject(t, event, account.DefaultCalendarURL+"moved.ics", `"etag-moved"`, deletedAt.Add(-time.Minute), "Older remote")
	client := &fakeCalDAVUIDQueryPushClient{
		fakeCalDAVPushClient: &fakeCalDAVPushClient{getErrors: map[string]error{event.RemoteHref: errCalDAVObjectNotFound}},
		queryObjects:         []calDAVCalendarObject{movedObject},
	}

	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.queryCalls) != 1 || client.queryCalls[0].EventUID != event.UID {
		t.Fatalf("UID query calls=%+v", client.queryCalls)
	}
	if len(client.deleteCalls) != 1 || client.deleteCalls[0].Path != movedObject.Path || client.deleteCalls[0].IfMatch != movedObject.ETag {
		t.Fatalf("DELETE calls=%+v", client.deleteCalls)
	}
	if _, found, errorValue := service.readCalendarEventByID(ctx, event.ID); errorValue != nil || found {
		t.Fatalf("local newer delete found=%v error=%v", found, errorValue)
	}
}

func TestPushCalendarDeleteRestoresNewerMovedRemoteEvent(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event, deletedAt := seedRecoverableCalendarDelete(t, service, "stale-href-remote-edit", account.DefaultCalendarURL+"stale.ics")
	remoteModifiedAt := deletedAt.Add(time.Minute).Truncate(time.Second)
	movedObject := recoveredCalendarDeleteObject(t, event, account.DefaultCalendarURL+"moved.ics", `"etag-moved"`, remoteModifiedAt, "Newer remote")
	client := &fakeCalDAVUIDQueryPushClient{
		fakeCalDAVPushClient: &fakeCalDAVPushClient{getErrors: map[string]error{event.RemoteHref: errCalDAVObjectNotFound}},
		queryObjects:         []calDAVCalendarObject{movedObject},
	}

	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.queryCalls) != 1 || len(client.deleteCalls) != 0 {
		t.Fatalf("UID queries=%+v DELETE calls=%+v", client.queryCalls, client.deleteCalls)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("restored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != "Newer remote" || storedEvent.RemoteHref != movedObject.Path || storedEvent.RemoteETag != movedObject.ETag {
		t.Fatalf("restored event=%+v", storedEvent)
	}
	fieldClocks := readCalendarFieldClocksForEventTest(t, service, event.UID)
	if !fieldClocks[calendarFieldTitle].Equal(remoteModifiedAt) {
		t.Fatalf("title clock=%s want %s", fieldClocks[calendarFieldTitle], remoteModifiedAt)
	}
	if !fieldClocks[calendarEventDeletionClockField].IsZero() {
		t.Fatalf("deletion clock remained=%s", fieldClocks[calendarEventDeletionClockField])
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	acknowledgements, errorValue := readCalendarTargetFieldAcknowledgements(ctx, database, account.ID, account.DefaultCalendarURL, event.UID)
	database.Close()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !acknowledgements[calendarFieldTitle].Equal(remoteModifiedAt) {
		t.Fatalf("title acknowledgement=%s want %s", acknowledgements[calendarFieldTitle], remoteModifiedAt)
	}
}

func TestPushCalendarDeletePreservesStaleHrefWhenUIDQueryIsUnsupported(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event, _ := seedRecoverableCalendarDelete(t, service, "stale-href-unsupported-query", account.DefaultCalendarURL+"stale.ics")
	client := &fakeCalDAVPushClient{getErrors: map[string]error{event.RemoteHref: errCalDAVObjectNotFound}}

	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.getCalls) != 1 || client.getCalls[0] != event.RemoteHref {
		t.Fatalf("GET calls=%+v", client.getCalls)
	}
	if len(client.deleteCalls) != 0 {
		t.Fatalf("DELETE calls=%+v", client.deleteCalls)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%+v error=%v", rows, errorValue)
	}
	if rows[0].AttemptCount != 1 || !strings.Contains(rows[0].LastError, "UID query") {
		t.Fatalf("retryable row=%+v", rows[0])
	}
}

func TestRecoveredCalendarDeleteResolvesNewerRemoteEventBeforeDelete(t *testing.T) {
	testCases := []struct {
		name        string
		objectPath  func(account remoteCalendarAccount, event calendarEvent) string
		buildClient func(canonicalPath string, remoteObject calDAVCalendarObject) (calDAVPushClient, *fakeCalDAVPushClient)
	}{
		{
			name: "canonical GET",
			objectPath: func(account remoteCalendarAccount, event calendarEvent) string {
				return account.DefaultCalendarURL + event.UID + ".ics"
			},
			buildClient: func(canonicalPath string, remoteObject calDAVCalendarObject) (calDAVPushClient, *fakeCalDAVPushClient) {
				baseClient := &fakeCalDAVPushClient{getObjects: map[string]calDAVCalendarObject{canonicalPath: remoteObject}}
				return baseClient, baseClient
			},
		},
		{
			name: "UID query",
			objectPath: func(account remoteCalendarAccount, event calendarEvent) string {
				return account.DefaultCalendarURL + "server-generated.ics"
			},
			buildClient: func(canonicalPath string, remoteObject calDAVCalendarObject) (calDAVPushClient, *fakeCalDAVPushClient) {
				baseClient := &fakeCalDAVPushClient{getErrors: map[string]error{canonicalPath: errCalDAVObjectNotFound}}
				return &fakeCalDAVUIDQueryPushClient{fakeCalDAVPushClient: baseClient, queryObjects: []calDAVCalendarObject{remoteObject}}, baseClient
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newCalendarTestService(t)
			ctx := context.Background()
			account := seedAccountWithDiscovery(t, service)
			event, deletedAt := seedRecoverableCalendarDelete(t, service, "recovered-current-etag", "")
			canonicalPath := account.DefaultCalendarURL + event.UID + ".ics"
			remoteObject := recoveredCalendarDeleteObject(t, event, testCase.objectPath(account, event), `"etag-current"`, deletedAt.Add(time.Minute), "Newer remote")
			client, callRecorder := testCase.buildClient(canonicalPath, remoteObject)

			if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
				t.Fatal(errorValue)
			}
			if len(callRecorder.deleteCalls) != 0 {
				t.Fatalf("DELETE calls=%+v", callRecorder.deleteCalls)
			}
			storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
			if errorValue != nil || !found {
				t.Fatalf("restored event found=%v error=%v", found, errorValue)
			}
			if storedEvent.Title != "Newer remote" || storedEvent.RemoteHref != remoteObject.Path || storedEvent.RemoteETag != remoteObject.ETag {
				t.Fatalf("restored event=%+v", storedEvent)
			}
		})
	}
}

func TestRecoveredCalendarDeleteRestoresNewerRemoteWithoutProjection(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("recovered-without-projection", "Newer remote")
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID: account.ID,
		EventID:   event.ID,
		EventUID:  event.UID,
		Operation: calendarOutboxOperationDelete,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil || len(rows) != 1 {
		t.Fatalf("outbox rows=%+v error=%v", rows, errorValue)
	}
	canonicalPath := account.DefaultCalendarURL + event.UID + ".ics"
	remoteObject := recoveredCalendarDeleteObject(t, event, canonicalPath, `"etag-current"`, parseCalendarConflictTime(rows[0].CreatedAt).Add(time.Minute), event.Title)
	client := &fakeCalDAVPushClient{getObjects: map[string]calDAVCalendarObject{canonicalPath: remoteObject}}

	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(client.deleteCalls) != 0 {
		t.Fatalf("DELETE calls=%+v", client.deleteCalls)
	}
	storedEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("restored event found=%v error=%v", found, errorValue)
	}
	if storedEvent.Title != event.Title || storedEvent.RemoteHref != remoteObject.Path || storedEvent.RemoteETag != remoteObject.ETag {
		t.Fatalf("restored event=%+v", storedEvent)
	}
}

func seedRecoverableCalendarDelete(t *testing.T, service *Service, suffix string, remoteHref string) (calendarEvent, time.Time) {
	t.Helper()
	event := newLocalTestCalendarEvent(suffix, "Local event")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = remoteHref
	event.RemoteETag = `"etag-before"`
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(context.Background(), event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(context.Background(), event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), event.ID)
	if errorValue != nil || !found || !projection.IsDeleted {
		t.Fatalf("deleted projection found=%v deleted=%v error=%v", found, projection.IsDeleted, errorValue)
	}
	return event, parseCalendarConflictTime(projection.DeletedAt)
}

func recoveredCalendarDeleteObject(t *testing.T, event calendarEvent, path string, etag string, modifiedAt time.Time, title string) calDAVCalendarObject {
	t.Helper()
	remoteEvent := event
	remoteEvent.Title = title
	return calDAVCalendarObject{
		Path: path,
		ETag: etag,
		Data: encodeCalendarTestEventWithLastModified(t, remoteEvent, modifiedAt),
	}
}
