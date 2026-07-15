package admind

import (
	"context"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestDiffCalendarEventFieldsReturnsAllForNewEvent(t *testing.T) {
	current := calendarEvent{Title: "Hello"}
	got := diffCalendarEventFields(calendarEvent{}, current)
	expected := calendarAllUserEditableFields()
	if !equalUnorderedStringSlices(got, expected) {
		t.Errorf("expected all user-editable fields for new event, got %v", got)
	}
}

func TestDiffCalendarEventFieldsReturnsOnlyChanged(t *testing.T) {
	previous := calendarEvent{
		ID:                "id-1",
		Title:             "Old",
		Description:       "desc",
		StartISO:          "2026-06-15T10:00:00Z",
		EndISO:            "2026-06-15T11:00:00Z",
		TimeZone:          "UTC",
		Color:             "#000000",
		ReminderLeadHours: 24,
	}
	current := previous
	current.Title = "New"
	current.StartISO = "2026-06-15T10:30:00Z"

	got := diffCalendarEventFields(previous, current)
	expected := []string{calendarFieldTitle, calendarFieldStart}
	if !equalUnorderedStringSlices(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
}

func TestMergeCalendarEventChangesOverlaysOnlyChangedFields(t *testing.T) {
	remote := calendarEvent{
		Title:             "Remote Title",
		Description:       "Remote Desc",
		StartISO:          "2026-06-15T11:00:00Z",
		EndISO:            "2026-06-15T12:00:00Z",
		Color:             "#ff0000",
		ReminderLeadHours: 60,
	}
	local := calendarEvent{
		Title:             "Local Title",
		Description:       "Local Desc",
		StartISO:          "2026-06-15T10:00:00Z",
		EndISO:            "2026-06-15T11:00:00Z",
		Color:             "#0000ff",
		ReminderLeadHours: 24,
	}
	merged := mergeCalendarEventChanges(remote, local, []string{calendarFieldTitle, calendarFieldColor})

	if merged.Title != "Local Title" {
		t.Errorf("title: got %q, want Local Title (admind wins)", merged.Title)
	}
	if merged.Color != "#0000ff" {
		t.Errorf("color: got %q, want #0000ff (admind wins)", merged.Color)
	}
	if merged.Description != "Remote Desc" {
		t.Errorf("description: got %q, want Remote Desc (remote preserved)", merged.Description)
	}
	if merged.StartISO != "2026-06-15T11:00:00Z" {
		t.Errorf("startISO: got %q, want 2026-06-15T11:00:00Z (remote preserved)", merged.StartISO)
	}
	if merged.ReminderLeadHours != 60 {
		t.Errorf("reminderLeadHours: got %d, want 60 (remote preserved)", merged.ReminderLeadHours)
	}
}

func TestPreserveCalendarInternalParticipantsKeepsLocalUnlessParticipantsChanged(t *testing.T) {
	local := calendarEvent{
		Participants: []calendarParticipant{
			{PersonID: "person-dongha", Name: "이동하", Email: "dongha@example.com"},
		},
	}
	remote := calendarEvent{Title: "Remote Title"}

	preserved := preserveCalendarInternalParticipants(remote, local, []string{calendarFieldTitle})
	if len(preserved.Participants) != 1 || preserved.Participants[0].PersonID != "person-dongha" {
		t.Fatalf("preserved participants = %+v", preserved.Participants)
	}

	cleared := preserveCalendarInternalParticipants(remote, local, []string{calendarFieldParticipants})
	if len(cleared.Participants) != 0 {
		t.Fatalf("changed participants = %+v", cleared.Participants)
	}
}

func TestIntersectCalendarFieldsReturnsOverlap(t *testing.T) {
	got := intersectCalendarFields([]string{"a", "b", "c"}, []string{"b", "c", "d"})
	expected := []string{"b", "c"}
	if !equalUnorderedStringSlices(got, expected) {
		t.Errorf("got %v, want %v", got, expected)
	}
	if intersectCalendarFields(nil, []string{"a"}) != nil {
		t.Error("nil input should return nil")
	}
}

func TestPendingCalendarLocalChangesTreatLegacyPutAsAllEditableFields(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	event := newLocalTestCalendarEvent("legacy-put", "Legacy Pending")
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID: account.ID,
		EventID:   event.ID,
		EventUID:  event.UID,
		Operation: calendarOutboxOperationPut,
	}); errorValue != nil {
		t.Fatalf("enqueue legacy put: %v", errorValue)
	}

	changes, errorValue := service.listPendingCalendarLocalChanges(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list pending local changes: %v", errorValue)
	}
	change, found := changes[event.UID]
	if !found {
		t.Fatal("legacy put should be treated as pending local change")
	}
	if !equalUnorderedStringSlices(change.ChangedFields, calendarAllUserEditableFields()) {
		t.Errorf("changed fields: got %v, want all editable fields", change.ChangedFields)
	}
}

func TestPullConflictDifferentFieldsPreservesPendingLocalEdit(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("pull-different", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-different.ics"
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

	remoteUpdated := baselineEvent
	remoteUpdated.Location = "Conference Room A"
	remoteICS, errorValue := encodeEventToICS(remoteUpdated)
	if errorValue != nil {
		t.Fatalf("encode remote update: %v", errorValue)
	}
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{{
		Path: baselineEvent.RemoteHref,
		ETag: `"etag-remote"`,
		Data: remoteICS,
	}}, nil); errorValue != nil {
		t.Fatalf("pull reconcile: %v", errorValue)
	}

	final, _, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil {
		t.Fatalf("read final: %v", errorValue)
	}
	if final.Title != "Local Renamed" {
		t.Errorf("title: got %q, want Local Renamed", final.Title)
	}
	if final.Location != "Conference Room A" {
		t.Errorf("location: got %q, want Conference Room A", final.Location)
	}
	finalRawEvent := decodeCalendarEventFromRawICS(final.RawICS, final.RemoteHref, final.CreatedByEmail)
	if finalRawEvent.Title != "Original Title" {
		t.Errorf("raw title: got %q, want Original Title", finalRawEvent.Title)
	}
	if finalRawEvent.Location != "Conference Room A" {
		t.Errorf("raw location: got %q, want Conference Room A", finalRawEvent.Location)
	}

	conflicts, errorValue := service.listActiveCalendarConflicts(ctx)
	if errorValue != nil {
		t.Fatalf("list conflicts: %v", errorValue)
	}
	if len(conflicts) != 0 {
		t.Fatalf("different-field pull should not record conflicts, got %d", len(conflicts))
	}

	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatalf("list outbox: %v", errorValue)
	}
	if len(rows) != 1 {
		t.Fatalf("pending local edit should remain queued, got %d rows", len(rows))
	}
	if rows[0].IfMatchETag != `"etag-remote"` {
		t.Errorf("outbox if-match: got %q, want etag-remote", rows[0].IfMatchETag)
	}
	if rows[0].RemoteHref != baselineEvent.RemoteHref {
		t.Errorf("outbox remote href: got %q, want %q", rows[0].RemoteHref, baselineEvent.RemoteHref)
	}
}

func TestPullConflictSameFieldRecordsConflictAndKeepsLocalEvent(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("pull-same", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-same.ics"
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
		t.Fatalf("local title update: %v", errorValue)
	}

	remoteUpdated := baselineEvent
	remoteUpdated.Title = "Remote Title"
	remoteICS, errorValue := encodeEventToICS(remoteUpdated)
	if errorValue != nil {
		t.Fatalf("encode remote update: %v", errorValue)
	}
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{{
		Path: baselineEvent.RemoteHref,
		ETag: `"etag-remote"`,
		Data: remoteICS,
	}}, nil); errorValue != nil {
		t.Fatalf("pull reconcile: %v", errorValue)
	}

	final, _, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil {
		t.Fatalf("read final: %v", errorValue)
	}
	if final.Title != "Local Title" {
		t.Errorf("title: got %q, want Local Title", final.Title)
	}
	finalRawEvent := decodeCalendarEventFromRawICS(final.RawICS, final.RemoteHref, final.CreatedByEmail)
	if finalRawEvent.Title != "Remote Title" {
		t.Errorf("raw title: got %q, want Remote Title", finalRawEvent.Title)
	}

	conflicts, errorValue := service.listActiveCalendarConflicts(ctx)
	if errorValue != nil {
		t.Fatalf("list conflicts: %v", errorValue)
	}
	if len(conflicts) != 1 {
		t.Fatalf("same-field pull should record one conflict, got %d", len(conflicts))
	}
	if conflicts[0].Field != calendarFieldTitle {
		t.Errorf("conflict field: got %q, want title", conflicts[0].Field)
	}
	if conflicts[0].LocalValue != "Local Title" {
		t.Errorf("conflict local value: got %q", conflicts[0].LocalValue)
	}
	if conflicts[0].RemoteValue != "Remote Title" {
		t.Errorf("conflict remote value: got %q", conflicts[0].RemoteValue)
	}
	if conflicts[0].DetectedAt == "" {
		t.Error("conflict detected time should be set")
	}
}

func TestPullConflictSameFieldRemoteNewerWins(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	baselineEvent := newLocalTestCalendarEvent("pull-same-remote-newer", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-same-remote-newer.ics"
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
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{{
		Path: baselineEvent.RemoteHref,
		ETag: `"etag-remote"`,
		Data: remoteICS,
	}}, nil); errorValue != nil {
		t.Fatal(errorValue)
	}
	finalEvent, found, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil || !found {
		t.Fatalf("final event: found=%v error=%v", found, errorValue)
	}
	if finalEvent.Title != "Remote Title" {
		t.Fatalf("title=%q want Remote Title", finalEvent.Title)
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(rows) != 0 {
		t.Fatalf("remote winner should clear pending field, rows=%+v", rows)
	}
}

func TestPullConflictReturnsErrorWhenConflictRecordFails(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("pull-record-fails", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-record-fails.ics"
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
		t.Fatalf("local title update: %v", errorValue)
	}
	blockCalendarConflictInserts(t, service, ctx)

	remoteUpdated := baselineEvent
	remoteUpdated.Title = "Remote Title"
	remoteICS, errorValue := encodeEventToICS(remoteUpdated)
	if errorValue != nil {
		t.Fatalf("encode remote update: %v", errorValue)
	}
	errorValue = service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{{
		Path: baselineEvent.RemoteHref,
		ETag: `"etag-remote"`,
		Data: remoteICS,
	}}, nil)
	if errorValue == nil {
		t.Fatal("expected pull conflict to return conflict record error")
	}

	final, _, errorValue := service.readCalendarEventByID(ctx, baselineEvent.ID)
	if errorValue != nil {
		t.Fatalf("read final: %v", errorValue)
	}
	if final.Title != "Local Title" {
		t.Errorf("title: got %q, want Local Title", final.Title)
	}
	if final.RawICS != baselineEvent.RawICS {
		t.Error("raw ICS should remain at the last persisted remote baseline")
	}
}

func TestPullConflictMissingRemoteAmbiguousOrderPrefersDeletion(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)

	baselineEvent := newLocalTestCalendarEvent("pull-missing", "Original Title")
	baselineEvent.RemoteSource = remoteCalendarProviderGoogle
	baselineEvent.RemoteETag = `"etag-base"`
	baselineEvent.RemoteHref = "/calendars/me/pull-missing.ics"
	baselineICS, errorValue := encodeEventToICS(baselineEvent)
	if errorValue != nil {
		t.Fatalf("encode baseline: %v", errorValue)
	}
	baselineEvent.RawICS = string(baselineICS)
	if errorValue := service.writeCalendarEventWithSource(ctx, baselineEvent, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed baseline: %v", errorValue)
	}
	lastSeenAt := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	if errorValue := service.upsertCalendarRemoteEventState(ctx, calendarRemoteEventState{
		AccountID:   account.ID,
		CalendarURL: activeRemoteCalendarTarget(account).CalendarURL,
		EventUID:    baselineEvent.UID,
		LastSeenAt:  lastSeenAt,
	}); errorValue != nil {
		t.Fatal(errorValue)
	}

	localUpdated := baselineEvent
	localUpdated.Title = "Local Pending"
	if errorValue := service.writeCalendarEvent(ctx, localUpdated); errorValue != nil {
		t.Fatalf("local title update: %v", errorValue)
	}

	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, nil, nil); errorValue != nil {
		t.Fatalf("pull reconcile: %v", errorValue)
	}

	projection, found, errorValue := service.readCalendarEventProjectionByID(ctx, baselineEvent.ID)
	if errorValue != nil {
		t.Fatalf("read projection: %v", errorValue)
	}
	if !found {
		t.Fatal("calendar event projection not found")
	}
	if !projection.IsDeleted {
		t.Fatal("ambiguous remote deletion should win over pending local edit")
	}
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(rows) != 0 {
		t.Fatalf("resolved pending rows remain: %+v", rows)
	}
	state, stateFound, errorValue := service.readCalendarRemoteEventState(ctx, account.ID, activeRemoteCalendarTarget(account).CalendarURL, baselineEvent.UID)
	if errorValue != nil || !stateFound {
		t.Fatalf("remote event state: found=%v error=%v", stateFound, errorValue)
	}
	if state.MissingDetectedAt == "" {
		t.Fatal("missing detection time was not stored")
	}
}

func TestPullConflictRemoteEditAfterLocalDeleteRestoresRemoteEvent(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	event := newLocalTestCalendarEvent("pull-edit-after-delete", "Original")
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = `"etag-before-edit"`
	event.RemoteHref = "/calendars/me/pull-edit-after-delete.ics"
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
	deletedAt := time.Date(2026, 7, 15, 3, 0, 0, 0, time.UTC)
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
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, []calDAVCalendarObject{{
		Path: event.RemoteHref,
		ETag: `"etag-after-edit"`,
		Data: remoteICS,
	}}, nil); errorValue != nil {
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
		t.Fatalf("resolved local delete should clear outbox, rows=%d", len(rows))
	}
}

func TestCalendarConflictRecordAndDismiss(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()

	if errorValue := service.recordCalendarConflict(ctx, "evt-1", "evt-1@example", "title", "local-val", "remote-val"); errorValue != nil {
		t.Fatalf("record: %v", errorValue)
	}
	conflicts, errorValue := service.listActiveCalendarConflicts(ctx)
	if errorValue != nil {
		t.Fatalf("list: %v", errorValue)
	}
	if len(conflicts) != 1 {
		t.Fatalf("expected 1 active conflict, got %d", len(conflicts))
	}
	conflictID := conflicts[0].ID

	if errorValue := service.dismissCalendarConflict(ctx, conflictID); errorValue != nil {
		t.Fatalf("dismiss: %v", errorValue)
	}
	after, _ := service.listActiveCalendarConflicts(ctx)
	if len(after) != 0 {
		t.Errorf("expected 0 active conflicts after dismiss, got %d", len(after))
	}
}

func equalUnorderedStringSlices(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a := append([]string(nil), left...)
	b := append([]string(nil), right...)
	sort.Strings(a)
	sort.Strings(b)
	return reflect.DeepEqual(a, b)
}

func blockCalendarConflictInserts(t *testing.T, service *Service, ctx context.Context) {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		t.Fatalf("open calendar database: %v", errorValue)
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
CREATE TRIGGER block_calendar_conflict_inserts
BEFORE INSERT ON calendar_conflicts
BEGIN
	SELECT RAISE(ABORT, 'blocked conflict insert');
END`)
	if errorValue != nil {
		t.Fatalf("create conflict insert blocker: %v", errorValue)
	}
}
