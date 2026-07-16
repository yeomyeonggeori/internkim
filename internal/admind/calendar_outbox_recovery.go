package admind

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const calendarOutboxBlockedRetryDelay = time.Minute

func (service *Service) pushCalendarOutboxDeleteWithRecovery(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow) (bool, error) {
	resolvedRow, found, errorValue := service.recoverCalendarOutboxDeleteRemoteState(ctx, account, client, row)
	if errorValue != nil || !found {
		return false, errorValue
	}
	errorValue = service.guardedCalendarDelete(ctx, client, resolvedRow, resolvedRow.RemoteHref, resolvedRow.IfMatchETag)
	if errorValue != nil && isCalDAVPreconditionFailed(errorValue) {
		return true, service.reconcileCalendarLocalDeletionDuringPush(ctx, account, client, resolvedRow, time.Now().UTC())
	}
	return errorValue == nil, errorValue
}

func (service *Service) recoverCalendarOutboxDeleteRemoteState(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow) (calendarOutboxRow, bool, error) {
	if strings.TrimSpace(row.RemoteHref) != "" {
		return row, true, nil
	}
	calendarURL := normalizeCalendarOutboxTargetURL(row.TargetCalendarURL)
	eventUID := strings.TrimSpace(row.EventUID)
	if calendarURL == "" || eventUID == "" {
		return calendarOutboxRow{}, false, fmt.Errorf("recover calendar delete target: calendar URL and event UID are required")
	}
	canonicalPath := strings.TrimRight(calendarURL, "/") + "/" + eventUID + ".ics"
	remoteObject, errorValue := service.guardedCalendarGet(ctx, client, row, canonicalPath)
	if errorValue != nil {
		if isCalDAVObjectNotFound(errorValue) {
			return row, false, nil
		}
		return calendarOutboxRow{}, false, errorValue
	}
	row.RemoteHref = firstNonEmpty(strings.TrimSpace(remoteObject.Path), canonicalPath)
	row.IfMatchETag = strings.TrimSpace(remoteObject.ETag)
	return row, true, nil
}

func calendarOutboxBatchRowIDs(row calendarOutboxRow) []int64 {
	if len(row.SourceRowIDs) > 0 {
		return row.SourceRowIDs
	}
	if row.ID == 0 {
		return nil
	}
	return []int64{row.ID}
}

func (service *Service) deleteCalendarOutboxBatch(ctx context.Context, row calendarOutboxRow) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := deleteCalendarOutboxBatchWithRunner(ctx, transaction, row); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func deleteCalendarOutboxBatchWithRunner(ctx context.Context, queryRunner calendarSQLRunner, row calendarOutboxRow) error {
	for _, rowID := range calendarOutboxBatchRowIDs(row) {
		if _, errorValue := queryRunner.ExecContext(ctx, `DELETE FROM calendar_outbox WHERE id = ?`, rowID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func canRetryBlockedCalendarOutbox(row calendarOutboxRow, currentTime time.Time) bool {
	if row.Status != calendarOutboxStatusBlocked {
		return true
	}
	lastAttemptedAt := calendarOutboxRetryTime(row.LastAttemptedAt, currentTime)
	failedAt := calendarOutboxRetryTime(row.FailedAt, currentTime)
	mostRecentFailureAt := lastAttemptedAt
	if failedAt.After(mostRecentFailureAt) {
		mostRecentFailureAt = failedAt
	}
	if mostRecentFailureAt.IsZero() {
		return true
	}
	return !currentTime.Before(mostRecentFailureAt.Add(calendarOutboxBlockedRetryDelay))
}

func calendarOutboxRetryTime(rawTime string, currentTime time.Time) time.Time {
	retryTime := parseCalendarConflictTime(rawTime)
	if retryTime.After(currentTime) {
		return time.Time{}
	}
	return retryTime
}

func (service *Service) markCalendarOutboxBatchAttempt(ctx context.Context, row calendarOutboxRow, lastError string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	attemptedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for _, rowID := range calendarOutboxBatchRowIDs(row) {
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE calendar_outbox SET attempt_count = attempt_count + 1, last_error = ?, last_attempted_at = ? WHERE id = ?`, strings.TrimSpace(lastError), attemptedAt, rowID); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	return transaction.Commit()
}

func (service *Service) markCalendarOutboxBatchBlocked(ctx context.Context, row calendarOutboxRow, failedAt string, failure string) error {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	for _, rowID := range calendarOutboxBatchRowIDs(row) {
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE calendar_outbox SET status = ?, failed_at = ?, last_error = ? WHERE id = ?`, calendarOutboxStatusBlocked, strings.TrimSpace(failedAt), strings.TrimSpace(failure), rowID); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	return transaction.Commit()
}
