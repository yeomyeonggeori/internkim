package admind

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

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
		if calendarDeleteIntentMatchesCreate(existing, eventID, request) {
			if errorValue := transaction.Commit(); errorValue != nil {
				return calendarDeleteIntent{}, errorValue
			}
			service.signalCalendarDeleteIntentWakeUp()
			return existing, nil
		}
		if !calendarDeleteIntentIsCancelPlaceholder(existing, eventID, request.ClientID) {
			return calendarDeleteIntent{}, errCalendarDeleteIntentPayloadMismatch
		}
		if errorValue := service.validateCalendarDeleteIntentCreateEvent(ctx, transaction, eventID, request); errorValue != nil {
			return calendarDeleteIntent{}, errorValue
		}
		intent := newCalendarDeleteIntent(eventID, operationID, request, requestedAt)
		intent.ResolutionSequence = existing.ResolutionSequence
		if request.Sequence <= existing.ResolutionSequence {
			intent.Status = calendarDeleteIntentStatusCanceled
			intent.ResolvedAt = existing.ResolvedAt
		}
		if errorValue := replaceCalendarDeleteIntentCancelPlaceholder(ctx, transaction, intent); errorValue != nil {
			return calendarDeleteIntent{}, errorValue
		}
		if errorValue := transaction.Commit(); errorValue != nil {
			return calendarDeleteIntent{}, errorValue
		}
		service.signalCalendarDeleteIntentWakeUp()
		return intent, nil
	}

	if errorValue := service.validateCalendarDeleteIntentCreateEvent(ctx, transaction, eventID, request); errorValue != nil {
		return calendarDeleteIntent{}, errorValue
	}
	intent := newCalendarDeleteIntent(eventID, operationID, request, requestedAt)
	if errorValue := insertCalendarDeleteIntent(ctx, transaction, intent); errorValue != nil {
		return calendarDeleteIntent{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return calendarDeleteIntent{}, errorValue
	}
	service.signalCalendarDeleteIntentWakeUp()
	return intent, nil
}

func (service *Service) cancelCalendarDeleteIntent(ctx context.Context, eventID string, operationID string, clientID string, sequence int64, resolvedAt time.Time) error {
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
	if errorValue != nil {
		return errorValue
	}
	if !found {
		if errorValue := requireActiveCalendarDeleteIntentEvent(ctx, transaction, eventID); errorValue != nil {
			return errorValue
		}
		placeholder := calendarDeleteIntent{
			OperationID:        strings.TrimSpace(operationID),
			EventID:            strings.TrimSpace(eventID),
			ClientID:           strings.TrimSpace(clientID),
			Status:             calendarDeleteIntentStatusCanceled,
			ResolvedAt:         resolvedAt.UTC().Format(time.RFC3339Nano),
			ResolutionSequence: sequence,
		}
		if errorValue := insertCalendarDeleteIntent(ctx, transaction, placeholder); errorValue != nil {
			return errorValue
		}
		if errorValue := transaction.Commit(); errorValue != nil {
			return errorValue
		}
		service.signalCalendarDeleteIntentWakeUp()
		return nil
	}
	if intent.EventID != strings.TrimSpace(eventID) {
		return sql.ErrNoRows
	}
	if intent.ClientID != strings.TrimSpace(clientID) {
		return errCalendarDeleteIntentPayloadMismatch
	}
	if intent.Status == calendarDeleteIntentStatusCanceled {
		if sequence < intent.ResolutionSequence {
			return errCalendarDeleteIntentPayloadMismatch
		}
		if sequence > intent.ResolutionSequence {
			if errorValue := updateCalendarDeleteIntentCancellation(ctx, transaction, intent.OperationID, sequence, resolvedAt); errorValue != nil {
				return errorValue
			}
		}
		if errorValue := transaction.Commit(); errorValue != nil {
			return errorValue
		}
		service.signalCalendarDeleteIntentWakeUp()
		return nil
	}
	if sequence <= intent.Sequence {
		return errCalendarDeleteIntentPayloadMismatch
	}
	if intent.Status != calendarDeleteIntentStatusPending {
		return errCalendarDeleteIntentNotPending
	}
	if errorValue := updateCalendarDeleteIntentCancellation(ctx, transaction, intent.OperationID, sequence, resolvedAt); errorValue != nil {
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return errorValue
	}
	service.signalCalendarDeleteIntentWakeUp()
	return nil
}
