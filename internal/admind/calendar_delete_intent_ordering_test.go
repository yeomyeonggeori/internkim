package admind

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestCalendarDeleteIntentDeletesExpectedVersionUsingRequestClock(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-expected-version")
	actionAt := parseCalendarConflictTime(event.UpdatedAt).Add(time.Second)
	intent := createCalendarDeleteIntentForTest(t, service, event, "expected-operation", "page-a", 2, actionAt)

	if errorValue := service.processDueCalendarDeleteIntents(context.Background(), actionAt.Add(calendarDeleteIntentDelay)); errorValue != nil {
		t.Fatal(errorValue)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), event.ID)
	if errorValue != nil || !found || !projection.IsDeleted {
		t.Fatalf("projection found = %v deleted = %v error = %v", found, projection.IsDeleted, errorValue)
	}
	if projection.DeletedAt != intent.RequestedAt {
		t.Fatalf("deletedAt = %q, request clock = %q", projection.DeletedAt, intent.RequestedAt)
	}
	assertCalendarDeleteIntentStatus(t, service, intent.OperationID, calendarDeleteIntentStatusExecuted)
}

func TestCalendarDeleteIntentConflictsWithDifferentClientMutation(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-different-client")
	actionAt := time.Now().UTC().Add(-calendarDeleteIntentDelay - time.Second)
	intent := createCalendarDeleteIntentForTest(t, service, event, "different-client-operation", "page-a", 2, actionAt)

	updateResponse := sendCalendarMutationUpdate(t, service, event, "Other page", event.UpdatedAt, "page-b", 1, true, true)
	if updateResponse.Code != 200 {
		t.Fatalf("other client PUT status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	if errorValue := service.processDueCalendarDeleteIntents(context.Background(), time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID); errorValue != nil || !found {
		t.Fatalf("event after conflict found = %v error = %v", found, errorValue)
	}
	assertCalendarDeleteIntentStatus(t, service, intent.OperationID, calendarDeleteIntentStatusConflicted)
}

func TestCalendarDeleteIntentConflictsWithSameClientLaterMutation(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-later-mutation")
	actionAt := time.Now().UTC().Add(-calendarDeleteIntentDelay - time.Second)
	intent := createCalendarDeleteIntentForTest(t, service, event, "later-mutation-operation", "page-a", 2, actionAt)

	updateResponse := sendCalendarMutationUpdate(t, service, event, "Later PUT", event.UpdatedAt, "page-a", 3, true, true)
	if updateResponse.Code != 200 {
		t.Fatalf("later PUT status = %d body = %s", updateResponse.Code, updateResponse.Body.String())
	}
	if errorValue := service.processDueCalendarDeleteIntents(context.Background(), time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID); errorValue != nil || !found {
		t.Fatalf("event after later mutation found = %v error = %v", found, errorValue)
	}
	assertCalendarDeleteIntentStatus(t, service, intent.OperationID, calendarDeleteIntentStatusConflicted)
}

func TestCalendarDeleteIntentCancellationPreventsFinalization(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-canceled-order")
	actionAt := time.Now().UTC().Add(-calendarDeleteIntentDelay - time.Second)
	intent := createCalendarDeleteIntentForTest(t, service, event, "canceled-order-operation", "page-a", 2, actionAt)
	if errorValue := service.cancelCalendarDeleteIntent(context.Background(), event.ID, intent.OperationID, "page-a", 3, time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := service.processDueCalendarDeleteIntents(context.Background(), time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID); errorValue != nil || !found {
		t.Fatalf("event after canceled intent found = %v error = %v", found, errorValue)
	}
	assertCalendarDeleteIntentStatus(t, service, intent.OperationID, calendarDeleteIntentStatusCanceled)
}

func TestCalendarDeleteIntentFinalizationRollsBackAndRecordsRetry(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-rollback")
	actionAt := time.Now().UTC().Add(-calendarDeleteIntentDelay - time.Second)
	intent := createCalendarDeleteIntentForTest(t, service, event, "rollback-operation", "page-a", 2, actionAt)
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(), `CREATE TRIGGER fail_delete_intent_event_update BEFORE UPDATE OF deleted_at ON calendar_events BEGIN SELECT RAISE(ABORT, 'delete intent persistence blocked'); END`); errorValue != nil {
		database.Close()
		t.Fatal(errorValue)
	}
	database.Close()

	processAt := time.Now().UTC()
	if errorValue := service.processDueCalendarDeleteIntents(context.Background(), processAt); errorValue == nil || !strings.Contains(errorValue.Error(), "delete intent persistence blocked") {
		t.Fatalf("process error = %v", errorValue)
	}
	if _, found, errorValue := service.readCalendarEventByID(context.Background(), event.ID); errorValue != nil || !found {
		t.Fatalf("event after rollback found = %v error = %v", found, errorValue)
	}
	storedIntent := readCalendarDeleteIntentForTest(t, service, intent.OperationID)
	if storedIntent.Status != calendarDeleteIntentStatusPending || storedIntent.AttemptCount != 1 || !strings.Contains(storedIntent.LastError, "delete intent persistence blocked") {
		t.Fatalf("intent after failure = %+v", storedIntent)
	}
	if !parseCalendarConflictTime(storedIntent.NextAttemptAt).After(processAt) {
		t.Fatalf("nextAttemptAt = %q processAt = %s", storedIntent.NextAttemptAt, processAt)
	}
}

func TestCalendarDeleteIntentRetryDelayIsCapped(t *testing.T) {
	if delay := calendarDeleteIntentRetryDelay(20); delay != calendarDeleteIntentRetryMaximumDelay {
		t.Fatalf("retry delay = %s, want %s", delay, calendarDeleteIntentRetryMaximumDelay)
	}
}

func createCalendarDeleteIntentForTest(t *testing.T, service *Service, event calendarEvent, operationID string, clientID string, sequence int64, requestedAt time.Time) calendarDeleteIntent {
	t.Helper()
	intent, errorValue := service.createCalendarDeleteIntent(context.Background(), event.ID, operationID, calendarDeleteIntentCreate{
		ClientID:          clientID,
		Sequence:          sequence,
		ExpectedUpdatedAt: event.UpdatedAt,
	}, requestedAt)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return intent
}

func readCalendarDeleteIntentForTest(t *testing.T, service *Service, operationID string) calendarDeleteIntent {
	t.Helper()
	database, errorValue := service.openCalendarDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	intent, found, errorValue := readCalendarDeleteIntentWithRunner(context.Background(), database, operationID)
	if errorValue != nil || !found {
		t.Fatalf("delete intent found = %v error = %v", found, errorValue)
	}
	return intent
}

func assertCalendarDeleteIntentStatus(t *testing.T, service *Service, operationID string, expectedStatus string) {
	t.Helper()
	intent := readCalendarDeleteIntentForTest(t, service, operationID)
	if intent.Status != expectedStatus {
		t.Fatalf("delete intent %q status = %q, want %q", operationID, intent.Status, expectedStatus)
	}
}
