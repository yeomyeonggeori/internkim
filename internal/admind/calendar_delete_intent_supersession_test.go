package admind

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestCalendarDeleteIntentSupersedesSameClientEarlierMutation(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-earlier-mutation")
	actionAt := time.Now().UTC().Add(-calendarDeleteIntentDelay - time.Second)
	intent := createCalendarDeleteIntentForTest(t, service, event, "earlier-mutation-operation", "page-a", 2, actionAt)

	updateResponse := sendCalendarMutationUpdate(t, service, event, "Late PUT", event.UpdatedAt, "page-a", 1, true, true)
	if updateResponse.Code != 200 {
		t.Fatalf("late PUT status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	var latePutEvent calendarEvent
	if errorValue := json.Unmarshal(updateResponse.Body.Bytes(), &latePutEvent); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.processDueCalendarDeleteIntents(context.Background(), time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}

	projection, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), event.ID)
	if errorValue != nil || !found || !projection.IsDeleted {
		t.Fatalf("projection found = %v deleted = %v error = %v", found, projection.IsDeleted, errorValue)
	}
	if projection.DeletedAt != intent.RequestedAt {
		t.Errorf("deletedAt = %q, request clock = %q", projection.DeletedAt, intent.RequestedAt)
	}
	if !parseCalendarConflictTime(projection.Event.UpdatedAt).After(parseCalendarConflictTime(latePutEvent.UpdatedAt)) {
		t.Errorf("storage revision = %q must beat the late PUT revision = %q", projection.Event.UpdatedAt, latePutEvent.UpdatedAt)
	}
	assertCalendarDeleteIntentStatus(t, service, intent.OperationID, calendarDeleteIntentStatusExecuted)
}

func TestCalendarDeleteIntentAlreadyDeletedEventResolvesWithoutDeletingTwice(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-already-deleted")
	actionAt := time.Now().UTC().Add(-calendarDeleteIntentDelay - time.Second)
	intent := createCalendarDeleteIntentForTest(t, service, event, "already-deleted-operation", "page-a", 2, actionAt)
	if errorValue := service.softDeleteCalendarEvent(context.Background(), event.ID); errorValue != nil {
		t.Fatal(errorValue)
	}
	deletedBefore, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), event.ID)
	if errorValue != nil || !found || !deletedBefore.IsDeleted {
		t.Fatalf("projection before finalizer found = %v deleted = %v error = %v", found, deletedBefore.IsDeleted, errorValue)
	}

	if errorValue := service.processDueCalendarDeleteIntents(context.Background(), time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}

	deletedAfter, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), event.ID)
	if errorValue != nil || !found {
		t.Fatalf("projection after finalizer found = %v error = %v", found, errorValue)
	}
	if deletedAfter.DeletedAt != deletedBefore.DeletedAt {
		t.Errorf("the finalizer deleted an already deleted event again: deletedAt = %q, was %q", deletedAfter.DeletedAt, deletedBefore.DeletedAt)
	}
	if deletedAfter.Event.UpdatedAt != deletedBefore.Event.UpdatedAt {
		t.Errorf("the finalizer advanced the revision of an already deleted event: %q, was %q", deletedAfter.Event.UpdatedAt, deletedBefore.Event.UpdatedAt)
	}
	assertCalendarDeleteIntentStatus(t, service, intent.OperationID, calendarDeleteIntentStatusExecuted)
}
