package admind

import (
	"context"
	"testing"
	"time"
)

func TestPushConflictDifferentFieldsPreservesBothSides(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("c3-different", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/c3-different.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatalf("encode baseline: %v", errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed baseline: %v", errorValue)
	}

	localUpdated := baselineEvent
	localUpdated.Title = "Local Renamed"
	if errorValue := service.writeCalendarEvent(ctx, localUpdated); errorValue != nil {
		t.Fatalf("local title update: %v", errorValue)
	}

	googleSideEvent := baselineEvent
	googleSideEvent.Location = "Conference Room A"
	googleSideICS, errorValue := encodeEventToICS(googleSideEvent)
	if errorValue != nil {
		t.Fatalf("encode google side: %v", errorValue)
	}

	pushClient := &fakeCalDAVPushClient{
		putErrorsQueue: map[string][]error{
			baselineEvent.RemoteHref: {errCalDAVPreconditionFailed},
		},
		putETags: map[string]string{baselineEvent.RemoteHref: `"etag-after-merge"`},
		getObjects: map[string]calDAVCalendarObject{
			baselineEvent.RemoteHref: {
				Path: baselineEvent.RemoteHref,
				ETag: `"etag-google-new"`,
				Data: googleSideICS,
			},
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, pushClient); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}

	if len(pushClient.putCalls) != 2 {
		t.Fatalf("expected 2 PUT calls (initial + retry), got %d", len(pushClient.putCalls))
	}
	if len(pushClient.getCalls) != 1 {
		t.Fatalf("expected 1 GET call after 412, got %d", len(pushClient.getCalls))
	}

	final, _, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil {
		t.Fatalf("read final: %v", errorValue)
	}
	if final.Title != "Local Renamed" {
		t.Errorf("title: got %q, want Local Renamed (admind wins on its change)", final.Title)
	}
	if final.Location != "Conference Room A" {
		t.Errorf("location: got %q, want Conference Room A (remote preserved)", final.Location)
	}
	if final.RemoteETag != `"etag-after-merge"` {
		t.Errorf("remoteETag: got %q, want etag-after-merge", final.RemoteETag)
	}
	finalRawEvent := decodeCalendarEventFromRawICS(final.RawICS, final.RemoteHref, final.CreatedByEmail)
	if finalRawEvent.Title != "Local Renamed" {
		t.Errorf("raw title: got %q, want Local Renamed", finalRawEvent.Title)
	}
	if finalRawEvent.Location != "Conference Room A" {
		t.Errorf("raw location: got %q, want Conference Room A", finalRawEvent.Location)
	}

	conflicts, errorValue := service.listActiveCalendarConflicts(ctx)
	if errorValue != nil {
		t.Fatalf("list conflicts: %v", errorValue)
	}
	if len(conflicts) != 0 {
		t.Errorf("no conflict should be recorded for different-field merge, got %d", len(conflicts))
	}

	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 0 {
		t.Errorf("outbox should be cleared after successful retry, got %d", len(rows))
	}
}

func TestPushConflictSameFieldRecordsConflictAndAdmindWins(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("c5-same", "Meeting")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/c5-same.ics"
	baselineICS, _ := encodeEventToICS(baselineEvent)
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed: %v", errorValue)
	}

	localUpdated := baselineEvent
	localUpdated.Title = "Local Title"
	if errorValue := service.writeCalendarEvent(ctx, localUpdated); errorValue != nil {
		t.Fatalf("local update: %v", errorValue)
	}

	googleSideEvent := baselineEvent
	googleSideEvent.Title = "Remote Title"
	googleSideICS, _ := encodeEventToICS(googleSideEvent)

	pushClient := &fakeCalDAVPushClient{
		putErrorsQueue: map[string][]error{
			baselineEvent.RemoteHref: {errCalDAVPreconditionFailed},
		},
		putETags: map[string]string{baselineEvent.RemoteHref: `"etag-after-merge"`},
		getObjects: map[string]calDAVCalendarObject{
			baselineEvent.RemoteHref: {
				Path: baselineEvent.RemoteHref,
				ETag: `"etag-google-new"`,
				Data: googleSideICS,
			},
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, pushClient); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}

	final, _, _ := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if final.Title != "Local Title" {
		t.Errorf("title after retry: got %q, want Local Title (admind wins)", final.Title)
	}

	conflicts, errorValue := service.listActiveCalendarConflicts(ctx)
	if errorValue != nil {
		t.Fatalf("list conflicts: %v", errorValue)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 conflict recorded, got %d", len(conflicts))
	}
	if conflicts[0].Field != calendarFieldTitle {
		t.Errorf("conflict field: got %q, want title", conflicts[0].Field)
	}
	if conflicts[0].LocalValue != "Local Title" {
		t.Errorf("conflict localValue: got %q", conflicts[0].LocalValue)
	}
	if conflicts[0].RemoteValue != "Remote Title" {
		t.Errorf("conflict remoteValue: got %q", conflicts[0].RemoteValue)
	}
}

func TestPushConflictSameFieldRemoteNewerWins(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("same-field-remote-newer", "Meeting")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/same-field-remote-newer.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	localEvent := baselineEvent
	localEvent.Title = "Local Title"
	if errorValue := service.writeCalendarEvent(ctx, localEvent); errorValue != nil {
		t.Fatal(errorValue)
	}
	localChangedAt := time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_outbox SET created_at = ? WHERE event_uid = ?`, localChangedAt.Format(time.RFC3339Nano), baselineEvent.UID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteEvent := baselineEvent
	remoteEvent.Title = "Remote Title"
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, localChangedAt.Add(time.Minute))
	pushClient := &fakeCalDAVPushClient{
		putErrorsQueue: map[string][]error{baselineEvent.RemoteHref: {errCalDAVPreconditionFailed}},
		getObjects: map[string]calDAVCalendarObject{
			baselineEvent.RemoteHref: {Path: baselineEvent.RemoteHref, ETag: `"etag-remote-newer"`, Data: remoteICS},
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, pushClient); errorValue != nil {
		t.Fatal(errorValue)
	}
	finalEvent, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("final event: found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Remote Title" {
		t.Fatalf("title=%q want Remote Title", finalEvent.Title)
	}
	if len(pushClient.putCalls) != 1 {
		t.Fatalf("remote winner should not be overwritten, put calls=%d", len(pushClient.putCalls))
	}
}

func TestPushConflictRemoteDeletionDoesNotLeaveLocalEditDiverged(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("remote-delete", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-before-delete"`
	event.RemoteHref = "/calendars/me/remote-delete.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: activeRemoteCalendarTarget(account).CalendarURL,
		EventUID:    event.UID,
		LastSeenAt:  time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano),
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	event.Title = "Local Edit"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatal(errorValue)
	}
	client := &fakeCalDAVPushClient{
		putErrors: map[string]error{event.RemoteHref: errCalDAVPreconditionFailed},
		getErrors: map[string]error{event.RemoteHref: errCalDAVObjectNotFound},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := service.readCalendarEventByID(ctx, event.ID); errorValue != nil || found {
		t.Fatalf("event should follow remote deletion: found=%v error=%v", found, errorValue)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(rows) != 0 {
		t.Fatalf("resolved deletion should clear outbox, rows=%d", len(rows))
	}
}

func TestPushConflictRemoteEditAfterLocalDeleteRestoresRemoteEvent(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("remote-edit-after-delete", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-before-edit"`
	event.RemoteHref = "/calendars/me/remote-edit-after-delete.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	deletedAt := time.Date(2026, 7, 15, 1, 0, 0, 0, time.UTC)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_events SET deleted_at = ? WHERE id = ?`, deletedAt.Format(time.RFC3339Nano), event.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteEvent := event
	remoteEvent.Title = "Remote Edit"
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, deletedAt.Add(time.Minute))
	client := &fakeCalDAVPushClient{
		deleteErrors: map[string]error{event.RemoteHref: errCalDAVPreconditionFailed},
		getObjects: map[string]calDAVCalendarObject{
			event.RemoteHref: {Path: event.RemoteHref, ETag: `"etag-after-edit"`, Data: remoteICS},
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	finalEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil || !found {
		t.Fatalf("remote newer event should be restored: found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Remote Edit" {
		t.Fatalf("title=%q want Remote Edit", finalEvent.Title)
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(rows) != 0 {
		t.Fatalf("resolved delete conflict should clear outbox, rows=%d", len(rows))
	}
}

func TestPushConflictLocalDeleteAfterRemoteEditDeletesLatestRemoteVersion(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("local-delete-after-edit", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-before-edit"`
	event.RemoteHref = "/calendars/me/local-delete-after-edit.ics"
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event.RawICS = string(encoded)
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := service.softDeleteCalendarEvent(ctx, event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	deletedAt := time.Date(2026, 7, 15, 2, 0, 0, 0, time.UTC)
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(ctx, `UPDATE calendar_events SET deleted_at = ? WHERE id = ?`, deletedAt.Format(time.RFC3339Nano), event.ID); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	if errorValue := database.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	remoteEvent := event
	remoteEvent.Title = "Earlier Remote Edit"
	remoteICS := encodeCalendarTestEventWithLastModified(t, remoteEvent, deletedAt.Add(-time.Minute))
	client := &fakeCalDAVPushClient{
		deleteErrorsQueue: map[string][]error{event.RemoteHref: {errCalDAVPreconditionFailed, nil}},
		getObjects: map[string]calDAVCalendarObject{
			event.RemoteHref: {Path: event.RemoteHref, ETag: `"etag-after-edit"`, Data: remoteICS},
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, client); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := service.readCalendarEventByID(ctx, event.ID); errorValue != nil || found {
		t.Fatalf("local newer deletion should remain: found=%v error=%v", found, errorValue)
	}
	if len(client.deleteCalls) != 2 {
		t.Fatalf("delete calls=%d want 2", len(client.deleteCalls))
	}
	if client.deleteCalls[1].IfMatch != `"etag-after-edit"` {
		t.Fatalf("retry If-Match=%q", client.deleteCalls[1].IfMatch)
	}
}

func encodeCalendarTestEventWithLastModified(t *testing.T, event calendarEvent, modifiedAt time.Time) []byte {
	t.Helper()
	encoded, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	calendar, errorValue := decodeCalendarObject(string(encoded))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	calendar.Events()[0].Props.SetDateTime("LAST-MODIFIED", modifiedAt.UTC())
	result, errorValue := encodeExistingCalendar(calendar)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return []byte(result)
}

func TestPushConflictKeepsOutboxRowWhenConflictRecordFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("push-record-fails", "Meeting")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/push-record-fails.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatalf("encode baseline: %v", errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed baseline: %v", errorValue)
	}

	localUpdated := baselineEvent
	localUpdated.Title = "Local Title"
	if errorValue := service.writeCalendarEvent(ctx, localUpdated); errorValue != nil {
		t.Fatalf("local update: %v", errorValue)
	}
	blockCalendarConflictInserts(t, service, ctx)

	googleSideEvent := baselineEvent
	googleSideEvent.Title = "Remote Title"
	googleSideICS, errorValue := encodeEventToICS(googleSideEvent)
	if errorValue != nil {
		t.Fatalf("encode google side: %v", errorValue)
	}
	pushClient := &fakeCalDAVPushClient{
		putErrorsQueue: map[string][]error{
			baselineEvent.RemoteHref: {errCalDAVPreconditionFailed},
		},
		putETags: map[string]string{baselineEvent.RemoteHref: `"etag-after-merge"`},
		getObjects: map[string]calDAVCalendarObject{
			baselineEvent.RemoteHref: {
				Path: baselineEvent.RemoteHref,
				ETag: `"etag-google-new"`,
				Data: googleSideICS,
			},
		},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, pushClient); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}

	if len(pushClient.putCalls) != 1 {
		t.Fatalf("expected no retry PUT after conflict record failure, got %d PUT calls", len(pushClient.putCalls))
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("outbox should be preserved when conflict record fails, got %d rows", len(rows))
	}
	if rows[0].AttemptCount != 1 {
		t.Errorf("attempt_count: got %d, want 1", rows[0].AttemptCount)
	}
}

func TestPushConflictKeepsOutboxRowWhenGetFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("get-fails", "Stale ETag")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-stale"`
	event.RemoteHref = "/calendars/me/get-fails.ics"
	if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed: %v", errorValue)
	}
	event.Title = "Local Edit"
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("local update: %v", errorValue)
	}

	pushClient := &fakeCalDAVPushClient{
		putErrors: map[string]error{event.RemoteHref: errCalDAVPreconditionFailed},
	}
	if _, errorValue := service.pushCalendarOutboxForAccount(ctx, account, pushClient); errorValue != nil {
		t.Fatalf("push: %v", errorValue)
	}

	rows, _ := service.listPendingCalendarOutbox(ctx, "")
	if len(rows) != 1 {
		t.Fatalf("outbox should be preserved when retry fetch fails, got %d", len(rows))
	}
	if rows[0].AttemptCount < 1 {
		t.Errorf("attempt_count should be incremented, got %d", rows[0].AttemptCount)
	}
}
