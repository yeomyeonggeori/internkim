package admind

import (
	"context"
	"reflect"
	"sort"
	"testing"
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
