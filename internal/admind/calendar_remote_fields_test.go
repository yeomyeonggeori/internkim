package admind

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCalendarEventRemoteFieldsRoundTrip(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	startTime := time.Now().UTC().Add(2 * time.Hour)
	endTime := startTime.Add(time.Hour)
	event := calendarEvent{
		ID:                "remote-roundtrip-1",
		UID:               "remote-roundtrip-1@google",
		Title:             "Remote event",
		Description:       "Pulled from Google",
		StartISO:          startTime.Format(time.RFC3339),
		EndISO:            endTime.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#10b981",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
		RemoteSource:      remoteCalendarProviderGoogle,
		RemoteETag:        `"abc-123"`,
		RemoteHref:        "/calendars/v1/example@gmail.com/events/abc.ics",
	}
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("writeCalendarEvent: %v", errorValue)
	}
	stored, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatalf("readCalendarEventByID: %v", errorValue)
	}
	if !found {
		t.Fatal("event not found after write")
	}
	if stored.RemoteSource != event.RemoteSource {
		t.Errorf("RemoteSource: got %q, want %q", stored.RemoteSource, event.RemoteSource)
	}
	if stored.RemoteETag != event.RemoteETag {
		t.Errorf("RemoteETag: got %q, want %q", stored.RemoteETag, event.RemoteETag)
	}
	if stored.RemoteHref != event.RemoteHref {
		t.Errorf("RemoteHref: got %q, want %q", stored.RemoteHref, event.RemoteHref)
	}
}

func TestCalendarEventLocalEventHasEmptyRemoteFields(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	startTime := time.Now().UTC().Add(time.Hour)
	endTime := startTime.Add(time.Hour)
	event := calendarEvent{
		ID:                "local-1",
		UID:               "local-1@internkim",
		Title:             "Local event",
		StartISO:          startTime.Format(time.RFC3339),
		EndISO:            endTime.Format(time.RFC3339),
		TimeZone:          "UTC",
		Color:             "#2563eb",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
	}
	if errorValue := service.writeCalendarEvent(ctx, event); errorValue != nil {
		t.Fatalf("writeCalendarEvent: %v", errorValue)
	}
	stored, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		t.Fatalf("readCalendarEventByID: %v", errorValue)
	}
	if !found {
		t.Fatal("event not found after write")
	}
	if stored.RemoteSource != "" || stored.RemoteETag != "" || stored.RemoteHref != "" {
		t.Errorf("expected empty remote fields, got source=%q etag=%q href=%q",
			stored.RemoteSource, stored.RemoteETag, stored.RemoteHref)
	}
}

func TestUpdateCalendarEventPreservesRemoteIdentity(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()

	seeded := calendarEvent{
		ID:                "remote-id-1",
		UID:               "remote-id-1@internkim",
		Title:             "original",
		Description:       "",
		Location:          "",
		StartISO:          "2026-05-27T00:00:00Z",
		EndISO:            "2026-05-27T01:00:00Z",
		TimeZone:          "Asia/Seoul",
		IsAllDay:          false,
		Color:             "#3b82f6",
		RawICS:            "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
		RemoteSource:      remoteCalendarProviderGoogle,
		RemoteETag:        `"etag-original"`,
		RemoteHref:        "/calendars/me/events/remote-id-1.ics",
	}
	if errorValue := service.writeCalendarEventWithSource(ctx, seeded, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed: %v", errorValue)
	}

	updatePayload := `{
		"title":"edited",
		"startISO":"2026-05-27T03:00:00Z",
		"endISO":"2026-05-27T04:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#3b82f6"
	}`
	updateRequest := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+seeded.ID, strings.NewReader(updatePayload))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	updateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}

	stored, found, errorValue := service.readCalendarEventByID(ctx, seeded.ID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup after update: found=%v error=%v", found, errorValue)
	}
	if stored.Title != "edited" {
		t.Errorf("title not updated: got %q", stored.Title)
	}
	if stored.StartISO != "2026-05-27T03:00:00Z" {
		t.Errorf("startISO not updated: got %q", stored.StartISO)
	}
	if stored.RemoteSource != remoteCalendarProviderGoogle {
		t.Errorf("RemoteSource lost: got %q want %q", stored.RemoteSource, remoteCalendarProviderGoogle)
	}
	if stored.RemoteETag != `"etag-original"` {
		t.Errorf("RemoteETag lost: got %q want %q", stored.RemoteETag, `"etag-original"`)
	}
	if stored.RemoteHref != "/calendars/me/events/remote-id-1.ics" {
		t.Errorf("RemoteHref lost: got %q", stored.RemoteHref)
	}
}

func TestUpdateCalendarEventSkipsUnchangedPayload(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()

	seeded := calendarEvent{
		ID:                "unchanged-id-1",
		UID:               "unchanged-id-1@internkim",
		Title:             "unchanged",
		Description:       "",
		Location:          "",
		StartISO:          "2026-05-27T00:00:00Z",
		EndISO:            "2026-05-27T01:00:00Z",
		TimeZone:          "Asia/Seoul",
		IsAllDay:          false,
		Color:             "#3b82f6",
		RawICS:            "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
		UpdatedByEmail:    "editor@example.com",
		UpdatedByName:     "Editor",
		UpdatedByAt:       "2026-05-27T00:30:00Z",
		RemoteSource:      remoteCalendarProviderGoogle,
		RemoteETag:        `"etag-original"`,
		RemoteHref:        "/calendars/me/events/unchanged-id-1.ics",
	}
	if errorValue := service.writeCalendarEventWithSource(ctx, seeded, calendarSourcePull); errorValue != nil {
		t.Fatalf("seed: %v", errorValue)
	}
	before, found, errorValue := service.readCalendarEventByID(ctx, seeded.ID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup before update: found=%v error=%v", found, errorValue)
	}

	updatePayload := `{
		"title":"unchanged",
		"startISO":"2026-05-27T00:00:00Z",
		"endISO":"2026-05-27T01:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#3b82f6"
	}`
	updateRequest := httptest.NewRequest(http.MethodPut, "/calendar/api/events/"+seeded.ID, strings.NewReader(updatePayload))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRequest.Header.Set("CF-Access-Authenticated-User-Email", "admin@example.com")
	updateResponse := httptest.NewRecorder()
	service.router().ServeHTTP(updateResponse, updateRequest)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}

	after, found, errorValue := service.readCalendarEventByID(ctx, seeded.ID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup after update: found=%v error=%v", found, errorValue)
	}
	if after.UpdatedAt != before.UpdatedAt {
		t.Errorf("UpdatedAt changed for unchanged payload: got %q want %q", after.UpdatedAt, before.UpdatedAt)
	}
	if after.UpdatedByEmail != before.UpdatedByEmail || after.UpdatedByName != before.UpdatedByName || after.UpdatedByAt != before.UpdatedByAt {
		t.Errorf("updated actor changed for unchanged payload: got %q/%q at %q want %q/%q at %q", after.UpdatedByEmail, after.UpdatedByName, after.UpdatedByAt, before.UpdatedByEmail, before.UpdatedByName, before.UpdatedByAt)
	}
	if after.RemoteETag != before.RemoteETag || after.RemoteHref != before.RemoteHref || after.RemoteSource != before.RemoteSource {
		t.Errorf("remote identity changed for unchanged payload: got %q/%q/%q want %q/%q/%q", after.RemoteSource, after.RemoteETag, after.RemoteHref, before.RemoteSource, before.RemoteETag, before.RemoteHref)
	}
}
