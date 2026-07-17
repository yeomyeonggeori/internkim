package admind

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	calendarDeleteIntentStatusExecuted    = "executed"
	calendarDeleteIntentStatusConflicted  = "conflicted"
	calendarDeleteIntentRetryBaseDelay    = time.Second
	calendarDeleteIntentRetryMaximumDelay = time.Minute
)

func (service *Service) processDueCalendarDeleteIntents(ctx context.Context, now time.Time) error {
	intents, errorValue := service.listPendingCalendarDeleteIntents(ctx)
	if errorValue != nil {
		return errorValue
	}
	var resultError error
	for _, intent := range intents {
		if !calendarDeleteIntentIsDue(intent, now) {
			continue
		}
		errorValue := service.finalizeCalendarDeleteIntent(ctx, intent.OperationID, now)
		if errorValue == nil {
			continue
		}
		if recordError := service.recordCalendarDeleteIntentFailure(ctx, intent.OperationID, errorValue, now); recordError != nil {
			resultError = errors.Join(resultError, errorValue, recordError)
			continue
		}
		resultError = errors.Join(resultError, errorValue)
	}
	return resultError
}

func (service *Service) finalizeCalendarDeleteIntent(ctx context.Context, operationID string, now time.Time) error {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	defer transaction.Rollback()

	intent, found, errorValue := readCalendarDeleteIntentWithRunner(ctx, transaction, operationID)
	if errorValue != nil || !found || intent.Status != calendarDeleteIntentStatusPending || !calendarDeleteIntentIsDue(intent, now) {
		return errorValue
	}
	event, isDeleted, found, errorValue := readCalendarDeleteIntentEvent(ctx, transaction, intent.EventID)
	if errorValue != nil {
		return errorValue
	}
	if !found || isDeleted {
		if errorValue := resolveCalendarDeleteIntent(ctx, transaction, intent.OperationID, calendarDeleteIntentStatusExecuted, now); errorValue != nil {
			return errorValue
		}
		return transaction.Commit()
	}
	shouldDelete, errorValue := calendarDeleteIntentSupersedesCurrentEvent(ctx, transaction, intent, event)
	if errorValue != nil {
		return errorValue
	}
	if !shouldDelete {
		if errorValue := resolveCalendarDeleteIntent(ctx, transaction, intent.OperationID, calendarDeleteIntentStatusConflicted, now); errorValue != nil {
			return errorValue
		}
		return transaction.Commit()
	}
	outboxRow, shouldSignalSync, errorValue := prepareCalendarOutboxForDeleteWithRunner(ctx, transaction, event)
	if errorValue != nil {
		return errorValue
	}
	requestedAt := parseCalendarConflictTime(intent.RequestedAt)
	if requestedAt.IsZero() {
		return fmt.Errorf("calendar delete intent %q requested_at is invalid", intent.OperationID)
	}
	if _, errorValue := service.persistCalendarEventDeletionWithTransaction(ctx, transaction, event, calendarSourceLocal, requestedAt, outboxRow, shouldSignalSync); errorValue != nil {
		return errorValue
	}
	if errorValue := resolveCalendarDeleteIntent(ctx, transaction, intent.OperationID, calendarDeleteIntentStatusExecuted, now); errorValue != nil {
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	service.runCalendarStoreSideEffectUnlocked(func() {
		if errorValue := service.cancelCalendarNotifications(ctx, event.ID); errorValue != nil {
			slog.WarnContext(ctx, "calendar notification cancel failed", "event_id", event.ID, "error", errorValue)
		}
		service.applyCalendarMattermostProjectionByID(ctx, event.ID)
	})
	if shouldSignalSync {
		service.signalCalendarSyncWakeUp()
	}
	return nil
}

func (service *Service) listPendingCalendarDeleteIntents(ctx context.Context) ([]calendarDeleteIntent, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT operation_id, event_id, client_id, sequence, expected_updated_at, requested_at, execute_at,
	status, resolved_at, resolution_sequence, next_attempt_at, attempt_count, last_error
FROM calendar_delete_intents
WHERE status = ?`, calendarDeleteIntentStatusPending)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	intents := []calendarDeleteIntent{}
	for rows.Next() {
		var intent calendarDeleteIntent
		if errorValue := rows.Scan(
			&intent.OperationID,
			&intent.EventID,
			&intent.ClientID,
			&intent.Sequence,
			&intent.ExpectedUpdatedAt,
			&intent.RequestedAt,
			&intent.ExecuteAt,
			&intent.Status,
			&intent.ResolvedAt,
			&intent.ResolutionSequence,
			&intent.NextAttemptAt,
			&intent.AttemptCount,
			&intent.LastError,
		); errorValue != nil {
			return nil, errorValue
		}
		intents = append(intents, intent)
	}
	return intents, rows.Err()
}

func readCalendarDeleteIntentEvent(ctx context.Context, transaction *sql.Tx, eventID string) (calendarEvent, bool, bool, error) {
	var event calendarEvent
	var deletedAt string
	errorValue := transaction.QueryRowContext(ctx, `
SELECT id, uid, start_at, end_at, updated_at, deleted_at, remote_etag, remote_href
FROM calendar_events
WHERE id = ?`, strings.TrimSpace(eventID)).Scan(
		&event.ID,
		&event.UID,
		&event.StartISO,
		&event.EndISO,
		&event.UpdatedAt,
		&deletedAt,
		&event.RemoteETag,
		&event.RemoteHref,
	)
	if errorValue == nil {
		return event, strings.TrimSpace(deletedAt) != "", true, nil
	}
	if errors.Is(errorValue, sql.ErrNoRows) {
		return calendarEvent{}, false, false, nil
	}
	return calendarEvent{}, false, false, errorValue
}

func calendarDeleteIntentSupersedesCurrentEvent(ctx context.Context, transaction *sql.Tx, intent calendarDeleteIntent, event calendarEvent) (bool, error) {
	if strings.TrimSpace(event.UpdatedAt) == strings.TrimSpace(intent.ExpectedUpdatedAt) {
		return true, nil
	}
	var clientID string
	var sequence int64
	errorValue := transaction.QueryRowContext(ctx, `
SELECT client_id, sequence
FROM calendar_event_mutation_origins
WHERE event_id = ? AND resulting_updated_at = ?`, event.ID, event.UpdatedAt).Scan(&clientID, &sequence)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return false, nil
	}
	if errorValue != nil {
		return false, errorValue
	}
	return strings.TrimSpace(clientID) == intent.ClientID && sequence < intent.Sequence, nil
}

func resolveCalendarDeleteIntent(ctx context.Context, transaction *sql.Tx, operationID string, status string, resolvedAt time.Time) error {
	_, errorValue := transaction.ExecContext(ctx, `
UPDATE calendar_delete_intents
SET status = ?, resolved_at = ?, last_error = ''
WHERE operation_id = ? AND status = ?`,
		status,
		resolvedAt.UTC().Format(time.RFC3339Nano),
		strings.TrimSpace(operationID),
		calendarDeleteIntentStatusPending,
	)
	return errorValue
}

func (service *Service) recordCalendarDeleteIntentFailure(ctx context.Context, operationID string, failure error, failedAt time.Time) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	defer transaction.Rollback()
	intent, found, errorValue := readCalendarDeleteIntentWithRunner(ctx, transaction, operationID)
	if errorValue != nil || !found || intent.Status != calendarDeleteIntentStatusPending {
		return errorValue
	}
	attemptCount := intent.AttemptCount + 1
	nextAttemptAt := failedAt.UTC().Add(calendarDeleteIntentRetryDelay(attemptCount)).Format(time.RFC3339Nano)
	_, errorValue = transaction.ExecContext(ctx, `
UPDATE calendar_delete_intents
SET attempt_count = ?, last_error = ?, next_attempt_at = ?
WHERE operation_id = ? AND status = ?`,
		attemptCount,
		failure.Error(),
		nextAttemptAt,
		intent.OperationID,
		calendarDeleteIntentStatusPending,
	)
	if errorValue != nil {
		return errorValue
	}
	return transaction.Commit()
}

func calendarDeleteIntentRetryDelay(attemptCount int) time.Duration {
	if attemptCount <= 1 {
		return calendarDeleteIntentRetryBaseDelay
	}
	delay := calendarDeleteIntentRetryBaseDelay
	for currentAttempt := 1; currentAttempt < attemptCount; currentAttempt++ {
		if delay >= calendarDeleteIntentRetryMaximumDelay/2 {
			return calendarDeleteIntentRetryMaximumDelay
		}
		delay *= 2
	}
	if delay > calendarDeleteIntentRetryMaximumDelay {
		return calendarDeleteIntentRetryMaximumDelay
	}
	return delay
}

func calendarDeleteIntentIsDue(intent calendarDeleteIntent, now time.Time) bool {
	executeAt := parseCalendarConflictTime(intent.ExecuteAt)
	nextAttemptAt := parseCalendarConflictTime(intent.NextAttemptAt)
	return !executeAt.IsZero() && !nextAttemptAt.IsZero() && !executeAt.After(now.UTC()) && !nextAttemptAt.After(now.UTC())
}
