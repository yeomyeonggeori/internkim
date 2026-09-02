package admind

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestUpdateCalendarEventSkipsUnchangedPayload(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()

	seeded := calendarEvent{
		ID:                "unchanged-id-1",
		UID:               "unchanged-id-1@internkim",
		Title:             "unchanged",
		StartISO:          "2026-05-27T00:00:00Z",
		EndISO:            "2026-05-27T01:00:00Z",
		TimeZone:          "Asia/Seoul",
		Color:             "#3b82f6",
		RawICS:            "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n",
		ReminderLeadHours: 24,
		CreatedByEmail:    "admin@example.com",
		UpdatedByEmail:    "editor@example.com",
		UpdatedByName:     "Editor",
		UpdatedByAt:       "2026-05-27T00:30:00Z",
	}
	if errorValue := service.writeCalendarEvent(ctx, seeded); errorValue != nil {
		t.Fatalf("seed: %v", errorValue)
	}
	before, found, errorValue := service.readCalendarEventByID(ctx, seeded.ID)
	if errorValue != nil || !found {
		t.Fatalf("event lookup before update: found=%v error=%v", found, errorValue)
	}

	updatePayload := fmt.Sprintf(`{
		"title":"unchanged",
		"startISO":"2026-05-27T00:00:00Z",
		"endISO":"2026-05-27T01:00:00Z",
		"timeZone":"Asia/Seoul",
		"color":"#3b82f6",
		"expectedUpdatedAt":%q
	}`, before.UpdatedAt)
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
}
