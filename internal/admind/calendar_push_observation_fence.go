package admind

import (
	"context"
	"sort"
	"strings"
	"time"
)

func persistCalendarPushObservationFenceWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string, eventUID string, createdAt time.Time) error {
	trimmedAccountID := strings.TrimSpace(accountID)
	canonicalCalendarURL := canonicalCalendarTargetURL(calendarURL)
	trimmedEventUID := strings.TrimSpace(eventUID)
	createdAtValue := createdAt.UTC().Format(time.RFC3339Nano)
	if _, errorValue := queryRunner.ExecContext(ctx, `
INSERT INTO calendar_push_observation_fences (account_id, calendar_url, event_uid, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(account_id, calendar_url, event_uid) DO UPDATE SET created_at = excluded.created_at`,
		trimmedAccountID,
		canonicalCalendarURL,
		trimmedEventUID,
		createdAtValue,
	); errorValue != nil {
		return errorValue
	}
	_, errorValue := queryRunner.ExecContext(ctx, `
UPDATE calendar_remote_event_sync_state
SET missing_detected_at = '', updated_at = ?
WHERE account_id = ? AND calendar_url = ? AND event_uid = ?`, createdAtValue, trimmedAccountID, canonicalCalendarURL, trimmedEventUID)
	return errorValue
}

func (service *Service) listCalendarPushObservationFenceUIDs(ctx context.Context, accountID string, calendarURL string) (map[string]struct{}, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT event_uid
FROM calendar_push_observation_fences
WHERE account_id = ? AND calendar_url = ?`, strings.TrimSpace(accountID), canonicalCalendarTargetURL(calendarURL))
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	result := map[string]struct{}{}
	for rows.Next() {
		var eventUID string
		if errorValue := rows.Scan(&eventUID); errorValue != nil {
			return nil, errorValue
		}
		result[eventUID] = struct{}{}
	}
	return result, rows.Err()
}

func (service *Service) clearObservedCalendarPushObservationFences(ctx context.Context, accountID string, calendarURL string, fencedUIDs map[string]struct{}, observedUIDs map[string]struct{}) error {
	eventUIDsToClear := observedCalendarPushObservationFenceUIDs(fencedUIDs, observedUIDs)
	if len(eventUIDsToClear) == 0 {
		return nil
	}
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := deleteCalendarPushObservationFencesWithRunner(ctx, transaction, accountID, calendarURL, eventUIDsToClear); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func observedCalendarPushObservationFenceUIDs(fencedUIDs map[string]struct{}, observedUIDs map[string]struct{}) []string {
	result := []string{}
	for eventUID := range fencedUIDs {
		if _, observed := observedUIDs[eventUID]; observed {
			result = append(result, eventUID)
		}
	}
	sort.Strings(result)
	return result
}

func deleteCalendarPushObservationFencesWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string, eventUIDs []string) error {
	if len(eventUIDs) == 0 {
		return nil
	}
	arguments := []any{strings.TrimSpace(accountID), canonicalCalendarTargetURL(calendarURL)}
	for _, eventUID := range eventUIDs {
		arguments = append(arguments, strings.TrimSpace(eventUID))
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(eventUIDs)), ",")
	_, errorValue := queryRunner.ExecContext(ctx, `
DELETE FROM calendar_push_observation_fences
WHERE account_id = ? AND calendar_url = ? AND event_uid IN (`+placeholders+")", arguments...)
	return errorValue
}

func deleteCalendarPushObservationFenceWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string, eventUID string) error {
	return deleteCalendarPushObservationFencesWithRunner(ctx, queryRunner, accountID, calendarURL, []string{eventUID})
}

func (service *Service) protectMissingCalendarPushObservationFences(ctx context.Context, accountID string, calendarURL string, fencedUIDs map[string]struct{}, observedUIDs map[string]struct{}, pendingLocalChanges map[string]pendingCalendarLocalChange, detectedAt time.Time) (map[string]struct{}, error) {
	protectedUIDs := map[string]struct{}{}
	for _, eventUID := range missingCalendarPushObservationFenceUIDs(fencedUIDs, observedUIDs) {
		_, hasActivePut := pendingLocalChanges[eventUID]
		shouldProtect, errorValue := service.shouldProtectMissingCalendarPushObservationFence(ctx, accountID, calendarURL, eventUID, hasActivePut, detectedAt)
		if errorValue != nil {
			return nil, errorValue
		}
		if shouldProtect {
			protectedUIDs[eventUID] = struct{}{}
		}
	}
	return protectedUIDs, nil
}

func (service *Service) shouldProtectMissingCalendarPushObservationFence(ctx context.Context, accountID string, calendarURL string, eventUID string, hasActivePut bool, detectedAt time.Time) (bool, error) {
	if hasActivePut {
		return true, nil
	}
	state, found, errorValue := service.readCalendarRemoteEventState(ctx, accountID, calendarURL, eventUID)
	if errorValue != nil {
		return false, errorValue
	}
	if found && strings.TrimSpace(state.MissingDetectedAt) != "" {
		return false, nil
	}
	_, errorValue = service.markCalendarRemoteEventMissingAtReservedBoundary(ctx, accountID, calendarURL, eventUID, detectedAt)
	return true, errorValue
}

func missingCalendarPushObservationFenceUIDs(fencedUIDs map[string]struct{}, observedUIDs map[string]struct{}) []string {
	result := []string{}
	for eventUID := range fencedUIDs {
		if _, observed := observedUIDs[eventUID]; !observed {
			result = append(result, eventUID)
		}
	}
	sort.Strings(result)
	return result
}
