package admind

import (
	"context"
	"database/sql"
	"strings"
)

type calendarDeletedEventMigrationCandidate struct {
	eventUID   string
	remoteHref string
	deletedAt  string
}

func seedCompletedLocalDeletionClocks(ctx context.Context, transaction *sql.Tx) error {
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT uid, remote_href, deleted_at
FROM calendar_events
WHERE deleted_at <> '' AND remote_source = ? AND remote_href <> ''`, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return errorValue
	}
	candidates := []calendarDeletedEventMigrationCandidate{}
	for rows.Next() {
		var candidate calendarDeletedEventMigrationCandidate
		if errorValue := rows.Scan(&candidate.eventUID, &candidate.remoteHref, &candidate.deletedAt); errorValue != nil {
			rows.Close()
			return errorValue
		}
		candidates = append(candidates, candidate)
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	for _, candidate := range candidates {
		isLocalDeletion, errorValue := calendarDeletedEventHasLocalTombstoneEvidence(ctx, transaction, candidate.eventUID, candidate.remoteHref)
		if errorValue != nil {
			return errorValue
		}
		if !isLocalDeletion {
			continue
		}
		if errorValue := persistCalendarEventFieldClocks(ctx, transaction, candidate.eventUID, []string{calendarEventDeletionClockField}, candidate.deletedAt); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func calendarDeletedEventHasLocalTombstoneEvidence(ctx context.Context, queryRunner calendarSQLRunner, eventUID string, remoteHref string) (bool, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, `
SELECT calendar_url, missing_detected_at, updated_at
FROM calendar_remote_event_sync_state
WHERE event_uid = ?`, strings.TrimSpace(eventUID))
	if errorValue != nil {
		return false, errorValue
	}
	defer rows.Close()
	foundRelevantState := false
	latestUpdatedAt := ""
	isLocalDeletion := false
	for rows.Next() {
		var calendarURL string
		var missingDetectedAt string
		var updatedAt string
		if errorValue := rows.Scan(&calendarURL, &missingDetectedAt, &updatedAt); errorValue != nil {
			return false, errorValue
		}
		if !calendarRemoteHrefBelongsToTargetURL(remoteHref, calendarURL) {
			continue
		}
		if !foundRelevantState || calendarIdentityFirstIsNewer(updatedAt, latestUpdatedAt) {
			foundRelevantState = true
			latestUpdatedAt = updatedAt
			isLocalDeletion = strings.TrimSpace(missingDetectedAt) == ""
			continue
		}
		if !calendarIdentityFirstIsNewer(latestUpdatedAt, updatedAt) && strings.TrimSpace(missingDetectedAt) != "" {
			isLocalDeletion = false
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return false, errorValue
	}
	return foundRelevantState && isLocalDeletion, nil
}
