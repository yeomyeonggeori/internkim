package admind

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type calendarOutboxRemoteMutationResult struct {
	targetChanged             bool
	pushed                    bool
	successfulRemoteOperation bool
	authenticationError       bool
	operationError            error
}

func (service *Service) calendarTargetSwitchIsWaiting() bool {
	return service.calendarSwitchWaiters.Load() > 0
}

func (service *Service) executeCalendarOutboxRemoteMutation(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow) (calendarOutboxRemoteMutationResult, error) {
	service.calendarRemoteMutex.Lock()
	defer service.calendarRemoteMutex.Unlock()
	result := calendarOutboxRemoteMutationResult{}
	isCurrentTarget, errorValue := service.calendarPushTargetIsCurrent(ctx, row)
	if errorValue != nil {
		return result, errorValue
	}
	if !isCurrentTarget {
		result.targetChanged = true
		return result, nil
	}
	if row.Status != calendarOutboxStatusBlocked && row.AttemptCount >= calendarOutboxMaxAttempts {
		slog.WarnContext(ctx, "calendar outbox batch blocked", "row_id", row.ID, "event_uid", row.EventUID, "attempt_count", row.AttemptCount, "last_error", row.LastError)
		failedAt := time.Now().UTC().Format(time.RFC3339Nano)
		if errorValue := service.markCalendarOutboxBatchBlocked(ctx, row, failedAt, row.LastError); errorValue != nil {
			return result, fmt.Errorf("mark calendar outbox batch %d blocked: %w", row.ID, errorValue)
		}
		return result, nil
	}
	if !canRetryBlockedCalendarOutbox(row, time.Now().UTC()) {
		return result, nil
	}
	pushed, errorValue := service.processCalendarOutboxRow(ctx, account, client, row)
	if errorValue != nil {
		if errors.Is(errorValue, errCalendarPushTargetChanged) {
			result.targetChanged = true
			return result, nil
		}
		result.operationError = errorValue
		result.authenticationError = isCalendarAuthError(errorValue)
		if result.authenticationError {
			service.markRemoteCalendarAccountAuthError(ctx, account, errorValue)
		}
		if outboxError := service.markCalendarOutboxBatchAttempt(ctx, row, errorValue.Error()); outboxError != nil {
			return result, fmt.Errorf("mark calendar outbox batch %d attempt: %w", row.ID, outboxError)
		}
		return result, nil
	}
	result.pushed = pushed
	result.successfulRemoteOperation = pushed || (row.Operation == calendarOutboxOperationDelete && strings.TrimSpace(row.RemoteHref) != "")
	if row.Operation == calendarOutboxOperationDelete {
		if errorValue := service.deleteCompletedCalendarDeleteOutboxBatch(ctx, row); errorValue != nil {
			return result, fmt.Errorf("delete calendar outbox batch %d: %w", row.ID, errorValue)
		}
		return result, nil
	}
	if errorValue := service.deleteCalendarOutboxBatch(ctx, row); errorValue != nil {
		return result, fmt.Errorf("delete calendar outbox batch %d: %w", row.ID, errorValue)
	}
	return result, nil
}
