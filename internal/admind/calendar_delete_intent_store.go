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

func (service *Service) createCalendarDeleteIntent(ctx context.Context, eventID string, operationID string, request calendarDeleteIntentCreate, requestedAt time.Time) (calendarDeleteIntent, error) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarDeleteIntent{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return calendarDeleteIntent{}, errorValue
	}
	defer transaction.Rollback()

	existing, found, errorValue := readCalendarDeleteIntentWithRunner(ctx, transaction, operationID)
	if errorValue != nil {
		return calendarDeleteIntent{}, errorValue
	}
	if found {
		if !calendarDeleteIntentMatchesCreate(existing, eventID, request) {
			return calendarDeleteIntent{}, errCalendarDeleteIntentPayloadMismatch
		}
		if errorValue := transaction.Commit(); errorValue != nil {
			return calendarDeleteIntent{}, errorValue
		}
		service.signalCalendarDeleteIntentWakeUp()
		return existing, nil
	}

	var currentUpdatedAt string
	errorValue = transaction.QueryRowContext(ctx, `SELECT updated_at FROM calendar_events WHERE id = ? AND deleted_at = ''`, strings.TrimSpace(eventID)).Scan(&currentUpdatedAt)
	if errorValue != nil {
		return calendarDeleteIntent{}, errorValue
	}
	if strings.TrimSpace(currentUpdatedAt) != strings.TrimSpace(request.ExpectedUpdatedAt) {
		intentCandidate := calendarDeleteIntent{ClientID: strings.TrimSpace(request.ClientID), Sequence: request.Sequence, ExpectedUpdatedAt: strings.TrimSpace(request.ExpectedUpdatedAt)}
		supersedesCurrent, errorValue := calendarDeleteIntentSupersedesCurrentEvent(ctx, transaction, intentCandidate, calendarEvent{ID: strings.TrimSpace(eventID), UpdatedAt: currentUpdatedAt})
		if errorValue != nil {
			return calendarDeleteIntent{}, errorValue
		}
		if !supersedesCurrent {
			return calendarDeleteIntent{}, errCalendarEventVersionConflict
		}
	}

	requestedAt = requestedAt.UTC()
	executeAt := requestedAt.Add(calendarDeleteIntentDelay)
	intent := calendarDeleteIntent{
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
	_, errorValue = transaction.ExecContext(ctx, `
INSERT INTO calendar_delete_intents (
	operation_id, event_id, client_id, sequence, expected_updated_at, requested_at, execute_at,
	status, resolved_at, resolution_sequence, next_attempt_at, attempt_count, last_error
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, '', 0, ?, 0, '')`,
		intent.OperationID,
		intent.EventID,
		intent.ClientID,
		intent.Sequence,
		intent.ExpectedUpdatedAt,
		intent.RequestedAt,
		intent.ExecuteAt,
		intent.Status,
		intent.NextAttemptAt,
	)
	if errorValue != nil {
		return calendarDeleteIntent{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return calendarDeleteIntent{}, errorValue
	}
	service.signalCalendarDeleteIntentWakeUp()
	return intent, nil
}

func (service *Service) cancelCalendarDeleteIntent(ctx context.Context, eventID string, operationID string, clientID string, sequence int64, resolvedAt time.Time) error {
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
	if errorValue != nil {
		return errorValue
	}
	if !found || intent.EventID != strings.TrimSpace(eventID) {
		return sql.ErrNoRows
	}
	if intent.ClientID != strings.TrimSpace(clientID) || sequence <= intent.Sequence {
		return errCalendarDeleteIntentPayloadMismatch
	}
	if intent.Status == calendarDeleteIntentStatusCanceled {
		if errorValue := transaction.Commit(); errorValue != nil {
			return errorValue
		}
		service.signalCalendarDeleteIntentWakeUp()
		return nil
	}
	if intent.Status != calendarDeleteIntentStatusPending {
		return errCalendarDeleteIntentNotPending
	}
	_, errorValue = transaction.ExecContext(ctx, `
UPDATE calendar_delete_intents
SET status = ?, resolved_at = ?, resolution_sequence = ?, last_error = ''
WHERE operation_id = ? AND status = ?`,
		calendarDeleteIntentStatusCanceled,
		resolvedAt.UTC().Format(time.RFC3339Nano),
		sequence,
		intent.OperationID,
		calendarDeleteIntentStatusPending,
	)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	service.signalCalendarDeleteIntentWakeUp()
	return nil
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
