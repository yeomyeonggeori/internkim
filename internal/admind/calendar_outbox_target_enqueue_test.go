package admind

import (
	"context"
	"strings"
	"testing"
)

func TestCalendarOutboxEnqueueRejectsMissingAccountTarget(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	for _, operation := range []string{calendarOutboxOperationPut, calendarOutboxOperationDelete} {
		event := newLocalTestCalendarEvent("missing-target-account-"+operation, "Missing Account")
		errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
			AccountID: "missing-account",
			EventID:   event.ID,
			EventUID:  event.UID,
			Operation: operation,
		})
		if errorValue == nil || !strings.Contains(errorValue.Error(), "missing-account") || !strings.Contains(errorValue.Error(), "reconnect") {
			t.Fatalf("operation=%q enqueue error=%v", operation, errorValue)
		}
	}
	rows, listError := service.listCalendarOutbox(ctx, "missing-account", true)
	if listError != nil {
		t.Fatal(listError)
	}
	if len(rows) != 0 {
		t.Fatalf("targetless rows=%+v", rows)
	}
}

func TestCalendarLocalWriteRollsBackForPartialSelectedTarget(t *testing.T) {
	service := newCalendarTestService(t)
	ctx := context.Background()
	account := seedAccountWithDiscovery(t, service)
	account.SelectedCalendarID = "partial@example.com"
	account.SelectedCalendarURL = ""
	if _, errorValue := service.upsertRemoteCalendarAccount(ctx, account); errorValue != nil {
		t.Fatal(errorValue)
	}
	event := newLocalTestCalendarEvent("partial-selected-target", "Partial Selected")
	errorValue := service.writeCalendarEvent(ctx, event)
	if errorValue == nil || !strings.Contains(errorValue.Error(), account.ID) || !strings.Contains(errorValue.Error(), "select a complete writable calendar") {
		t.Fatalf("write error=%v", errorValue)
	}
	if _, found, readError := service.readCalendarEventByID(ctx, event.ID); readError != nil || found {
		t.Fatalf("event found=%v error=%v", found, readError)
	}
	rows, listError := service.listCalendarOutbox(ctx, account.ID, true)
	if listError != nil {
		t.Fatal(listError)
	}
	if len(rows) != 0 {
		t.Fatalf("targetless rows=%+v", rows)
	}
}
