package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	calendarDeleteIntentDelay          = 5 * time.Second
	calendarDeleteIntentStatusPending  = "pending"
	calendarDeleteIntentStatusCanceled = "canceled"
)

var errCalendarDeleteIntentPayloadMismatch = errors.New("calendar delete intent operation payload mismatch")
var errCalendarDeleteIntentNotPending = errors.New("calendar delete intent is not pending")

type calendarDeleteIntent struct {
	OperationID        string
	EventID            string
	ClientID           string
	Sequence           int64
	ExpectedUpdatedAt  string
	RequestedAt        string
	ExecuteAt          string
	Status             string
	ResolvedAt         string
	ResolutionSequence int64
	NextAttemptAt      string
	AttemptCount       int
	LastError          string
}

type calendarDeleteIntentCreate struct {
	ClientID          string
	Sequence          int64
	ExpectedUpdatedAt string
}

func (service *Service) validateCalendarDeleteIntentCreateEvent(ctx context.Context, transaction *sql.Tx, eventID string, request calendarDeleteIntentCreate) error {
	var currentUpdatedAt string
	errorValue := transaction.QueryRowContext(ctx, `SELECT updated_at FROM calendar_events WHERE id = ? AND deleted_at = ''`, strings.TrimSpace(eventID)).Scan(&currentUpdatedAt)
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(currentUpdatedAt) == strings.TrimSpace(request.ExpectedUpdatedAt) {
		return nil
	}
	intentCandidate := calendarDeleteIntent{ClientID: strings.TrimSpace(request.ClientID), Sequence: request.Sequence, ExpectedUpdatedAt: strings.TrimSpace(request.ExpectedUpdatedAt)}
	supersedesCurrent, errorValue := calendarDeleteIntentSupersedesCurrentEvent(ctx, transaction, intentCandidate, calendarEvent{ID: strings.TrimSpace(eventID), UpdatedAt: currentUpdatedAt})
	if errorValue != nil {
		return errorValue
	}
	if !supersedesCurrent {
		return errCalendarEventVersionConflict
	}
	return nil
}

func newCalendarDeleteIntent(eventID string, operationID string, request calendarDeleteIntentCreate, requestedAt time.Time) calendarDeleteIntent {
	requestedAt = requestedAt.UTC()
	executeAt := requestedAt.Add(calendarDeleteIntentDelay)
	return calendarDeleteIntent{
		OperationID:       strings.TrimSpace(operationID),
		EventID:           strings.TrimSpace(eventID),
		ClientID:          strings.TrimSpace(request.ClientID),
		Sequence:          request.Sequence,
		ExpectedUpdatedAt: strings.TrimSpace(request.ExpectedUpdatedAt),
		RequestedAt:       requestedAt.Format(time.RFC3339Nano),
		ExecuteAt:         executeAt.Format(time.RFC3339Nano),
		Status:            calendarDeleteIntentStatusPending,
		NextAttemptAt:     executeAt.Format(time.RFC3339Nano),
	}
}

func insertCalendarDeleteIntent(ctx context.Context, transaction *sql.Tx, intent calendarDeleteIntent) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO calendar_delete_intents (
	operation_id, event_id, client_id, sequence, expected_updated_at, requested_at, execute_at,
	status, resolved_at, resolution_sequence, next_attempt_at, attempt_count, last_error
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		intent.OperationID,
		intent.EventID,
		intent.ClientID,
		intent.Sequence,
		intent.ExpectedUpdatedAt,
		intent.RequestedAt,
		intent.ExecuteAt,
		intent.Status,
		intent.ResolvedAt,
		intent.ResolutionSequence,
		intent.NextAttemptAt,
		intent.AttemptCount,
		intent.LastError,
	)
	return errorValue
}

func calendarDeleteIntentIsCancelPlaceholder(intent calendarDeleteIntent, eventID string, clientID string) bool {
	return intent.EventID == strings.TrimSpace(eventID) &&
		intent.ClientID == strings.TrimSpace(clientID) &&
		intent.Status == calendarDeleteIntentStatusCanceled &&
		intent.Sequence == 0 &&
		intent.ExpectedUpdatedAt == "" &&
		intent.RequestedAt == "" &&
		intent.ExecuteAt == ""
}

func replaceCalendarDeleteIntentCancelPlaceholder(ctx context.Context, transaction *sql.Tx, intent calendarDeleteIntent) error {
	result, errorValue := transaction.ExecContext(ctx, `
UPDATE calendar_delete_intents
SET sequence = ?, expected_updated_at = ?, requested_at = ?, execute_at = ?, status = ?,
	resolved_at = ?, resolution_sequence = ?, next_attempt_at = ?, attempt_count = 0, last_error = ''
WHERE operation_id = ? AND status = ? AND sequence = 0`,
		intent.Sequence,
		intent.ExpectedUpdatedAt,
		intent.RequestedAt,
		intent.ExecuteAt,
		intent.Status,
		intent.ResolvedAt,
		intent.ResolutionSequence,
		intent.NextAttemptAt,
		intent.OperationID,
		calendarDeleteIntentStatusCanceled,
	)
	if errorValue != nil {
		return errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		return errorValue
	}
	if affectedRows != 1 {
		return errCalendarDeleteIntentPayloadMismatch
	}
	return nil
}

func requireActiveCalendarDeleteIntentEvent(ctx context.Context, transaction *sql.Tx, eventID string) error {
	var storedEventID string
	return transaction.QueryRowContext(ctx, `SELECT id FROM calendar_events WHERE id = ? AND deleted_at = ''`, strings.TrimSpace(eventID)).Scan(&storedEventID)
}

func updateCalendarDeleteIntentCancellation(ctx context.Context, transaction *sql.Tx, operationID string, sequence int64, resolvedAt time.Time) error {
	_, errorValue := transaction.ExecContext(ctx, `
UPDATE calendar_delete_intents
SET status = ?, resolved_at = ?, resolution_sequence = ?, last_error = ''
WHERE operation_id = ? AND status IN (?, ?)`,
		calendarDeleteIntentStatusCanceled,
		resolvedAt.UTC().Format(time.RFC3339Nano),
		sequence,
		strings.TrimSpace(operationID),
		calendarDeleteIntentStatusPending,
		calendarDeleteIntentStatusCanceled,
	)
	return errorValue
}

func readCalendarDeleteIntentWithRunner(ctx context.Context, queryRunner interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, operationID string) (calendarDeleteIntent, bool, error) {
	var intent calendarDeleteIntent
	errorValue := queryRunner.QueryRowContext(ctx, `
SELECT operation_id, event_id, client_id, sequence, expected_updated_at, requested_at, execute_at,
	status, resolved_at, resolution_sequence, next_attempt_at, attempt_count, last_error
FROM calendar_delete_intents
WHERE operation_id = ?`, strings.TrimSpace(operationID)).Scan(
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
	)
	if errorValue == nil {
		return intent, true, nil
	}
	if errors.Is(errorValue, sql.ErrNoRows) {
		return calendarDeleteIntent{}, false, nil
	}
	return calendarDeleteIntent{}, false, errorValue
}

func calendarDeleteIntentMatchesCreate(intent calendarDeleteIntent, eventID string, request calendarDeleteIntentCreate) bool {
	return intent.EventID == strings.TrimSpace(eventID) &&
		intent.ClientID == strings.TrimSpace(request.ClientID) &&
		intent.Sequence == request.Sequence &&
		intent.ExpectedUpdatedAt == strings.TrimSpace(request.ExpectedUpdatedAt)
}
