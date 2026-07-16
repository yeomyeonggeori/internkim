package admind

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type calendarRemoteEventState struct {
	AccountID         string
	CalendarURL       string
	EventUID          string
	RemoteModifiedAt  string
	LastSeenAt        string
	MissingDetectedAt string
}

type calendarRemoteEventStateQueryRunner interface {
	calendarSQLRunner
	QueryRowContext(ctx context.Context, query string, arguments ...any) *sql.Row
}

func upsertCalendarRemoteEventStateWithRunner(ctx context.Context, queryRunner calendarSQLRunner, state calendarRemoteEventState, updatedAt string) error {
	_, errorValue := queryRunner.ExecContext(ctx, `
INSERT INTO calendar_remote_event_sync_state(account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id, calendar_url, event_uid) DO UPDATE SET
	remote_modified_at = excluded.remote_modified_at,
	last_seen_at = excluded.last_seen_at,
	missing_detected_at = excluded.missing_detected_at,
	updated_at = excluded.updated_at`,
		strings.TrimSpace(state.AccountID),
		canonicalCalendarTargetURL(state.CalendarURL),
		strings.TrimSpace(state.EventUID),
		strings.TrimSpace(state.RemoteModifiedAt),
		strings.TrimSpace(state.LastSeenAt),
		strings.TrimSpace(state.MissingDetectedAt),
		strings.TrimSpace(updatedAt),
	)
	return errorValue
}

func (service *Service) readCalendarRemoteEventState(ctx context.Context, accountID string, calendarURL string, eventUID string) (calendarRemoteEventState, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarRemoteEventState{}, false, errorValue
	}
	defer database.Close()
	return readCalendarRemoteEventStateWithRunner(ctx, database, accountID, calendarURL, eventUID)
}

func readCalendarRemoteEventStateWithRunner(ctx context.Context, queryRunner calendarRemoteEventStateQueryRunner, accountID string, calendarURL string, eventUID string) (calendarRemoteEventState, bool, error) {
	state := calendarRemoteEventState{}
	errorValue := queryRunner.QueryRowContext(ctx, `
SELECT account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at
FROM calendar_remote_event_sync_state
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`,
		strings.TrimSpace(accountID), canonicalCalendarTargetURL(calendarURL), strings.TrimSpace(eventUID),
	).Scan(
		&state.AccountID,
		&state.CalendarURL,
		&state.EventUID,
		&state.RemoteModifiedAt,
		&state.LastSeenAt,
		&state.MissingDetectedAt,
	)
	if errors.Is(errorValue, sql.ErrNoRows) {
		return calendarRemoteEventState{}, false, nil
	}
	if errorValue != nil {
		return calendarRemoteEventState{}, false, errorValue
	}
	state.CalendarURL = canonicalCalendarTargetURL(state.CalendarURL)
	return state, true, nil
}

func (service *Service) markCalendarRemoteEventObserved(ctx context.Context, accountID string, calendarURL string, event calendarEvent, observedAt time.Time) error {
	reservedObservedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, observedAt)
	if errorValue != nil {
		return errorValue
	}
	return service.markCalendarRemoteEventObservedAtReservedBoundary(ctx, accountID, calendarURL, event, reservedObservedAt)
}

func (service *Service) markCalendarRemoteEventObservedAtReservedBoundary(ctx context.Context, accountID string, calendarURL string, event calendarEvent, reservedObservedAt time.Time) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := service.persistObservedCalendarRemoteEventStateWithTransaction(ctx, transaction, accountID, calendarURL, event, reservedObservedAt); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) persistObservedCalendarRemoteEventStateWithTransaction(ctx context.Context, transaction *sql.Tx, accountID string, calendarURL string, event calendarEvent, reservedObservedAt time.Time) (calendarRemoteEventState, error) {
	_, errorValue := service.allocateCalendarConflictTime(ctx, transaction, event.UID, reservedObservedAt)
	if errorValue != nil {
		return calendarRemoteEventState{}, errorValue
	}
	state, found, errorValue := readCalendarRemoteEventStateWithRunner(ctx, transaction, accountID, calendarURL, event.UID)
	if errorValue != nil {
		return calendarRemoteEventState{}, errorValue
	}
	if !found {
		state = calendarRemoteEventState{AccountID: accountID, CalendarURL: canonicalCalendarTargetURL(calendarURL), EventUID: event.UID}
	}
	if strings.TrimSpace(event.RemoteModifiedAt) != "" {
		state.RemoteModifiedAt = event.RemoteModifiedAt
	}
	state.LastSeenAt = latestCalendarRemoteStateBoundary(state, reservedObservedAt).Format(time.RFC3339Nano)
	state.MissingDetectedAt = ""
	if errorValue := upsertCalendarRemoteEventStateWithRunner(ctx, transaction, state, state.LastSeenAt); errorValue != nil {
		return calendarRemoteEventState{}, errorValue
	}
	return state, nil
}

func (service *Service) markCalendarRemoteEventMissing(ctx context.Context, accountID string, calendarURL string, eventUID string, detectedAt time.Time) (calendarRemoteEventState, error) {
	reservedDetectedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, detectedAt)
	if errorValue != nil {
		return calendarRemoteEventState{}, errorValue
	}
	return service.markCalendarRemoteEventMissingAtReservedBoundary(ctx, accountID, calendarURL, eventUID, reservedDetectedAt)
}

func (service *Service) markCalendarRemoteEventMissingAtReservedBoundary(ctx context.Context, accountID string, calendarURL string, eventUID string, reservedDetectedAt time.Time) (calendarRemoteEventState, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarRemoteEventState{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return calendarRemoteEventState{}, errorValue
	}
	_, errorValue = service.allocateCalendarConflictTime(ctx, transaction, eventUID, reservedDetectedAt)
	if errorValue != nil {
		_ = transaction.Rollback()
		return calendarRemoteEventState{}, errorValue
	}
	state, found, errorValue := readCalendarRemoteEventStateWithRunner(ctx, transaction, accountID, calendarURL, eventUID)
	if errorValue != nil {
		_ = transaction.Rollback()
		return calendarRemoteEventState{}, errorValue
	}
	if !found {
		state = calendarRemoteEventState{AccountID: accountID, CalendarURL: canonicalCalendarTargetURL(calendarURL), EventUID: eventUID}
	}
	if strings.TrimSpace(state.MissingDetectedAt) != "" {
		_ = transaction.Rollback()
		return state, nil
	}
	state.MissingDetectedAt = latestCalendarRemoteStateBoundary(state, reservedDetectedAt).Format(time.RFC3339Nano)
	if errorValue := upsertCalendarRemoteEventStateWithRunner(ctx, transaction, state, state.MissingDetectedAt); errorValue != nil {
		_ = transaction.Rollback()
		return calendarRemoteEventState{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return calendarRemoteEventState{}, errorValue
	}
	return state, nil
}

func latestCalendarRemoteStateBoundary(state calendarRemoteEventState, candidate time.Time) time.Time {
	latest := candidate.UTC()
	for _, rawTime := range []string{state.LastSeenAt, state.MissingDetectedAt} {
		parsedTime := parseCalendarConflictTime(rawTime)
		if parsedTime.After(latest) {
			latest = parsedTime
		}
	}
	return latest
}

func (service *Service) markCalendarRemoteTargetLastSeen(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, observedAt time.Time) error {
	reservedObservedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, observedAt)
	if errorValue != nil {
		return errorValue
	}
	events, errorValue := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return errorValue
	}
	observedEvents := make([]calendarEvent, 0, len(events))
	for _, event := range events {
		if !remoteCalendarEventBelongsToTarget(event, target) {
			continue
		}
		observedEvents = append(observedEvents, event)
	}
	return service.persistObservedCalendarRemoteEventStateBatch(ctx, account.ID, target.CalendarURL, observedEvents, reservedObservedAt)
}
