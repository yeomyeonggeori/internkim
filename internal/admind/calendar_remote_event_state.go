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

func (service *Service) upsertCalendarRemoteEventState(ctx context.Context, state calendarRemoteEventState) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO calendar_remote_event_sync_state(account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(account_id, calendar_url, event_uid) DO UPDATE SET
	remote_modified_at = excluded.remote_modified_at,
	last_seen_at = excluded.last_seen_at,
	missing_detected_at = excluded.missing_detected_at,
	updated_at = excluded.updated_at`,
		strings.TrimSpace(state.AccountID),
		strings.TrimSpace(state.CalendarURL),
		strings.TrimSpace(state.EventUID),
		strings.TrimSpace(state.RemoteModifiedAt),
		strings.TrimSpace(state.LastSeenAt),
		strings.TrimSpace(state.MissingDetectedAt),
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return errorValue
}

func (service *Service) readCalendarRemoteEventState(ctx context.Context, accountID string, calendarURL string, eventUID string) (calendarRemoteEventState, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarRemoteEventState{}, false, errorValue
	}
	defer database.Close()
	state := calendarRemoteEventState{}
	errorValue = database.QueryRowContext(ctx, `
SELECT account_id, calendar_url, event_uid, remote_modified_at, last_seen_at, missing_detected_at
FROM calendar_remote_event_sync_state
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`,
		strings.TrimSpace(accountID), strings.TrimSpace(calendarURL), strings.TrimSpace(eventUID),
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
	return state, true, nil
}

func (service *Service) markCalendarRemoteEventObserved(ctx context.Context, accountID string, calendarURL string, event calendarEvent, observedAt time.Time) error {
	state, found, errorValue := service.readCalendarRemoteEventState(ctx, accountID, calendarURL, event.UID)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		state = calendarRemoteEventState{AccountID: accountID, CalendarURL: calendarURL, EventUID: event.UID}
	}
	if strings.TrimSpace(event.RemoteModifiedAt) != "" {
		state.RemoteModifiedAt = event.RemoteModifiedAt
	}
	state.LastSeenAt = observedAt.UTC().Format(time.RFC3339Nano)
	state.MissingDetectedAt = ""
	return service.upsertCalendarRemoteEventState(ctx, state)
}

func (service *Service) markCalendarRemoteEventMissing(ctx context.Context, accountID string, calendarURL string, eventUID string, detectedAt time.Time) (calendarRemoteEventState, error) {
	state, found, errorValue := service.readCalendarRemoteEventState(ctx, accountID, calendarURL, eventUID)
	if errorValue != nil {
		return calendarRemoteEventState{}, errorValue
	}
	if !found {
		state = calendarRemoteEventState{AccountID: accountID, CalendarURL: calendarURL, EventUID: eventUID}
	}
	if strings.TrimSpace(state.MissingDetectedAt) == "" {
		state.MissingDetectedAt = detectedAt.UTC().Format(time.RFC3339Nano)
	}
	if errorValue := service.upsertCalendarRemoteEventState(ctx, state); errorValue != nil {
		return calendarRemoteEventState{}, errorValue
	}
	return state, nil
}

func (service *Service) markCalendarRemoteTargetLastSeen(ctx context.Context, account remoteCalendarAccount, target remoteCalendarTarget, observedAt time.Time) error {
	events, errorValue := service.readRemoteCalendarEventsByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return errorValue
	}
	for _, event := range events {
		if !remoteCalendarEventBelongsToTarget(event, target) {
			continue
		}
		if errorValue := service.markCalendarRemoteEventObserved(ctx, account.ID, target.CalendarURL, event, observedAt); errorValue != nil {
			return errorValue
		}
	}
	return nil
}
