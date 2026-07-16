package admind

import (
	"context"
	"sort"
	"strings"
	"time"
)

func (service *Service) persistCalendarPushObservationFence(ctx context.Context, accountID string, calendarURL string, eventUID string, createdAt time.Time) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return persistCalendarPushObservationFenceWithRunner(ctx, database, accountID, calendarURL, eventUID, createdAt)
}

func persistCalendarPushObservationFenceWithRunner(ctx context.Context, queryRunner calendarSQLRunner, accountID string, calendarURL string, eventUID string, createdAt time.Time) error {
	_, errorValue := queryRunner.ExecContext(ctx, `
INSERT INTO calendar_push_observation_fences (account_id, calendar_url, event_uid, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(account_id, calendar_url, event_uid) DO UPDATE SET created_at = excluded.created_at`,
		strings.TrimSpace(accountID),
		canonicalCalendarTargetURL(calendarURL),
		strings.TrimSpace(eventUID),
		createdAt.UTC().Format(time.RFC3339Nano),
	)
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

func mergeCalendarUIDSets(first map[string]struct{}, second map[string]struct{}) map[string]struct{} {
	result := map[string]struct{}{}
	for eventUID := range first {
		result[eventUID] = struct{}{}
	}
	for eventUID := range second {
		result[eventUID] = struct{}{}
	}
	return result
}

func hasMissingCalendarPushObservationFence(fencedUIDs map[string]struct{}, observedUIDs map[string]struct{}) bool {
	for eventUID := range fencedUIDs {
		if _, observed := observedUIDs[eventUID]; !observed {
			return true
		}
	}
	return false
}
