package admind

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCalendarDeleteIntentCancelBeforeCreatePersistsAcrossRestart(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-cancel-before-create-restart")
	resolvedAt := time.Now().UTC()
	if errorValue := service.cancelCalendarDeleteIntent(context.Background(), event.ID, "cancel-before-create-restart", "page-a", 3, resolvedAt); errorValue != nil {
		t.Fatal(errorValue)
	}

	restartedService := NewService(service.Configuration)
	placeholder := readCalendarDeleteIntentForTest(t, restartedService, "cancel-before-create-restart")
	if placeholder.EventID != event.ID || placeholder.ClientID != "page-a" || placeholder.Status != calendarDeleteIntentStatusCanceled {
		t.Fatalf("cancel placeholder = %+v", placeholder)
	}
	if placeholder.Sequence != 0 || placeholder.ResolutionSequence != 3 || placeholder.ResolvedAt != resolvedAt.Format(time.RFC3339Nano) {
		t.Fatalf("cancel placeholder ordering = %+v", placeholder)
	}

	if errorValue := restartedService.cancelCalendarDeleteIntent(context.Background(), event.ID, placeholder.OperationID, "page-a", 3, resolvedAt.Add(time.Second)); errorValue != nil {
		t.Fatalf("idempotent cancel error = %v", errorValue)
	}
	if errorValue := restartedService.cancelCalendarDeleteIntent(context.Background(), event.ID, placeholder.OperationID, "page-a", 4, resolvedAt.Add(2*time.Second)); errorValue != nil {
		t.Fatalf("higher cancel error = %v", errorValue)
	}
	updatedPlaceholder := readCalendarDeleteIntentForTest(t, restartedService, placeholder.OperationID)
	if updatedPlaceholder.ResolutionSequence != 4 {
		t.Fatalf("higher cancel resolution sequence = %d", updatedPlaceholder.ResolutionSequence)
	}
	if errorValue := restartedService.cancelCalendarDeleteIntent(context.Background(), event.ID, placeholder.OperationID, "page-a", 3, resolvedAt.Add(3*time.Second)); !errors.Is(errorValue, errCalendarDeleteIntentPayloadMismatch) {
		t.Fatalf("lower cancel error = %v", errorValue)
	}
	if errorValue := restartedService.cancelCalendarDeleteIntent(context.Background(), event.ID, placeholder.OperationID, "page-b", 5, resolvedAt.Add(4*time.Second)); !errors.Is(errorValue, errCalendarDeleteIntentPayloadMismatch) {
		t.Fatalf("different client cancel error = %v", errorValue)
	}

	if errorValue := restartedService.processDueCalendarDeleteIntents(context.Background(), resolvedAt.Add(time.Hour)); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, found, errorValue := restartedService.readCalendarEventByID(context.Background(), event.ID); errorValue != nil || !found {
		t.Fatalf("event after canceled placeholder worker pass found = %v error = %v", found, errorValue)
	}
}

func TestCalendarDeleteIntentCreateAfterEarlierCancelUsesSequenceOrdering(t *testing.T) {
	testCases := []struct {
		name           string
		createSequence int64
		expectedStatus string
		shouldDelete   bool
	}{
		{name: "lower create remains canceled", createSequence: 2, expectedStatus: calendarDeleteIntentStatusCanceled},
		{name: "equal create remains canceled", createSequence: 3, expectedStatus: calendarDeleteIntentStatusCanceled},
		{name: "higher create becomes pending", createSequence: 4, expectedStatus: calendarDeleteIntentStatusPending, shouldDelete: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newCalendarTestService(t)
			event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-create-ordering-"+testCase.name)
			operationID := "create-ordering-" + testCase.name
			if errorValue := service.cancelCalendarDeleteIntent(context.Background(), event.ID, operationID, "page-a", 3, time.Now().UTC()); errorValue != nil {
				t.Fatal(errorValue)
			}
			restartedService := NewService(service.Configuration)
			requestedAt := time.Now().UTC().Add(-calendarDeleteIntentDelay - time.Second)
			request := calendarDeleteIntentCreate{
				ClientID:          "page-a",
				Sequence:          testCase.createSequence,
				ExpectedUpdatedAt: event.UpdatedAt,
			}
			intent, errorValue := restartedService.createCalendarDeleteIntent(context.Background(), event.ID, operationID, request, requestedAt)
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			if intent.Status != testCase.expectedStatus || intent.Sequence != testCase.createSequence || intent.ResolutionSequence != 3 {
				t.Fatalf("created intent = %+v", intent)
			}
			duplicate, errorValue := restartedService.createCalendarDeleteIntent(context.Background(), event.ID, operationID, request, requestedAt.Add(time.Hour))
			if errorValue != nil {
				t.Fatalf("duplicate create error = %v", errorValue)
			}
			if duplicate.RequestedAt != intent.RequestedAt || duplicate.Status != intent.Status {
				t.Fatalf("duplicate intent = %+v, first = %+v", duplicate, intent)
			}

			if errorValue := restartedService.processDueCalendarDeleteIntents(context.Background(), time.Now().UTC()); errorValue != nil {
				t.Fatal(errorValue)
			}
			projection, found, errorValue := restartedService.readCalendarEventProjectionByID(context.Background(), event.ID)
			if errorValue != nil || !found || projection.IsDeleted != testCase.shouldDelete {
				t.Fatalf("projection found = %v deleted = %v want deleted = %v error = %v", found, projection.IsDeleted, testCase.shouldDelete, errorValue)
			}
			expectedFinalStatus := testCase.expectedStatus
			if testCase.shouldDelete {
				expectedFinalStatus = calendarDeleteIntentStatusExecuted
			}
			assertCalendarDeleteIntentStatus(t, restartedService, operationID, expectedFinalStatus)
		})
	}
}

func TestCalendarDeleteIntentCancelPlaceholderDoesNotAffectDifferentOperation(t *testing.T) {
	service := newCalendarTestService(t)
	event := seedCalendarDeleteIntentEvent(t, service, "delete-intent-canceled-placeholder-other-operation")
	requestedAt := time.Now().UTC().Add(-calendarDeleteIntentDelay - time.Second)
	if errorValue := service.cancelCalendarDeleteIntent(context.Background(), event.ID, "canceled-placeholder-operation", "page-a", 3, requestedAt); errorValue != nil {
		t.Fatal(errorValue)
	}
	activeIntent := createCalendarDeleteIntentForTest(t, service, event, "active-different-operation", "page-a", 4, requestedAt)

	if errorValue := service.processDueCalendarDeleteIntents(context.Background(), time.Now().UTC()); errorValue != nil {
		t.Fatal(errorValue)
	}
	projection, found, errorValue := service.readCalendarEventProjectionByID(context.Background(), event.ID)
	if errorValue != nil || !found || !projection.IsDeleted {
		t.Fatalf("projection after different operation found = %v deleted = %v error = %v", found, projection.IsDeleted, errorValue)
	}
	assertCalendarDeleteIntentStatus(t, service, "canceled-placeholder-operation", calendarDeleteIntentStatusCanceled)
	assertCalendarDeleteIntentStatus(t, service, activeIntent.OperationID, calendarDeleteIntentStatusExecuted)
}
