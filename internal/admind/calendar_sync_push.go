package admind

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"
)

const calendarOutboxMaxAttempts = 10

type calDAVPushClient interface {
	putCalendarObject(ctx context.Context, objectPath string, ics []byte, ifMatch string, ifNoneMatch string) (string, error)
	deleteCalendarObject(ctx context.Context, objectPath string, ifMatch string) error
	getCalendarObject(ctx context.Context, objectPath string) (calDAVCalendarObject, error)
}

func (service *Service) pushPendingCalendarOutbox(ctx context.Context) (map[string]struct{}, error) {
	return service.pushPendingCalendarOutboxForProvider(ctx, googleCalendarProvider{})
}

func (service *Service) pushPendingCalendarOutboxForProvider(ctx context.Context, provider calendarProvider) (map[string]struct{}, error) {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, provider.Name())
	if errorValue != nil {
		return nil, errorValue
	}
	if !found || selectedRemoteCalendarTarget(account).CalendarURL == "" {
		return nil, nil
	}
	if !remoteCalendarAccountCanWrite(account) {
		return nil, nil
	}
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		return nil, errorValue
	}
	if len(rows) == 0 {
		return nil, nil
	}
	httpClient, errorValue := provider.BuildHTTPClient(ctx, service, account)
	if errorValue != nil {
		service.markRemoteCalendarAccountAuthError(ctx, account, errorValue)
		return nil, fmt.Errorf("build http client: %w", errorValue)
	}
	client, errorValue := newOutboundCalDAVClient(provider.Endpoint(account), httpClient)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.pushCalendarOutboxRowsForAccount(ctx, account, client, rows)
}

func isCalendarAuthError(errorValue error) bool {
	if errorValue == nil {
		return false
	}
	message := strings.ToLower(errorValue.Error())
	return strings.Contains(message, "invalid_grant") ||
		strings.Contains(message, "unauthorized") ||
		strings.Contains(message, "401")
}

func (service *Service) pushCalendarOutboxForAccount(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient) (map[string]struct{}, error) {
	rows, errorValue := service.listCalendarOutbox(ctx, account.ID, true)
	if errorValue != nil {
		return nil, errorValue
	}
	return service.pushCalendarOutboxRowsForAccount(ctx, account, client, rows)
}

func (service *Service) pushCalendarOutboxRowsForAccount(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, rows []calendarOutboxRow) (map[string]struct{}, error) {
	pushedUIDs := map[string]struct{}{}
	if !remoteCalendarAccountCanWrite(account) {
		return pushedUIDs, nil
	}
	target := activeRemoteCalendarTarget(account)
	if target.NeedsInitialSyncCompletion && strings.TrimSpace(account.SelectedCalendarReadinessStatus) != calendarReadinessStatusInitialExportPending {
		return pushedUIDs, nil
	}
	activeBatches, isCurrentTarget, errorValue := service.prepareCalendarOutboxPushBatches(ctx, account, rows)
	if errorValue != nil {
		return pushedUIDs, errorValue
	}
	if !isCurrentTarget {
		return pushedUIDs, nil
	}
	hasAuthError := false
	hasSuccessfulRemoteOperation := false
	for _, row := range activeBatches {
		isCurrentTarget, errorValue := service.calendarPushTargetIsCurrent(ctx, row)
		if errorValue != nil {
			return pushedUIDs, errorValue
		}
		if !isCurrentTarget {
			break
		}
		if row.Status != calendarOutboxStatusBlocked && row.AttemptCount >= calendarOutboxMaxAttempts {
			slog.WarnContext(ctx, "calendar outbox batch blocked", "row_id", row.ID, "event_uid", row.EventUID, "attempt_count", row.AttemptCount, "last_error", row.LastError)
			failedAt := time.Now().UTC().Format(time.RFC3339Nano)
			if errorValue := service.markCalendarOutboxBatchBlocked(ctx, row, failedAt, row.LastError); errorValue != nil {
				return pushedUIDs, fmt.Errorf("mark calendar outbox batch %d blocked: %w", row.ID, errorValue)
			}
			continue
		}
		if !canRetryBlockedCalendarOutbox(row, time.Now().UTC()) {
			continue
		}
		pushed, errorValue := service.processCalendarOutboxRow(ctx, account, client, row)
		if errorValue != nil {
			if errors.Is(errorValue, errCalendarPushTargetChanged) {
				break
			}
			log.Printf("calendar outbox row %d failed: %v", row.ID, errorValue)
			isAuthenticationError := isCalendarAuthError(errorValue)
			if isAuthenticationError {
				hasAuthError = true
				service.markRemoteCalendarAccountAuthError(ctx, account, errorValue)
			}
			if outboxErr := service.markCalendarOutboxBatchAttempt(ctx, row, errorValue.Error()); outboxErr != nil {
				return pushedUIDs, fmt.Errorf("mark calendar outbox batch %d attempt: %w", row.ID, outboxErr)
			}
			if isAuthenticationError {
				break
			}
			continue
		}
		if pushed || (row.Operation == calendarOutboxOperationDelete && strings.TrimSpace(row.RemoteHref) != "") {
			hasSuccessfulRemoteOperation = true
		}
		if row.Operation == calendarOutboxOperationDelete {
			if errorValue := service.deleteCompletedCalendarDeleteOutboxBatch(ctx, row); errorValue != nil {
				return pushedUIDs, fmt.Errorf("delete calendar outbox batch %d: %w", row.ID, errorValue)
			}
		} else if errorValue := service.deleteCalendarOutboxBatch(ctx, row); errorValue != nil {
			return pushedUIDs, fmt.Errorf("delete calendar outbox batch %d: %w", row.ID, errorValue)
		}
		if pushed && row.Operation == calendarOutboxOperationPut && strings.TrimSpace(row.EventUID) != "" {
			pushedUIDs[row.EventUID] = struct{}{}
		}
	}
	if hasSuccessfulRemoteOperation && !hasAuthError {
		service.clearRemoteCalendarAccountAuthError(ctx, account)
	}
	if target.NeedsInitialSyncCompletion {
		if errorValue := service.completeCalendarInitialSyncIfReady(ctx, account, time.Now()); errorValue != nil {
			return pushedUIDs, errorValue
		}
	}
	return pushedUIDs, nil
}

func (service *Service) processCalendarOutboxRow(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow) (bool, error) {
	switch row.Operation {
	case calendarOutboxOperationPut:
		return service.pushCalendarOutboxPut(ctx, account, client, row)
	case calendarOutboxOperationDelete:
		return service.pushCalendarOutboxDelete(ctx, account, client, row)
	}
	return false, fmt.Errorf("unknown outbox operation %q", row.Operation)
}

func (service *Service) pushCalendarOutboxPut(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow) (bool, error) {
	event, found, errorValue := service.readCalendarEventByID(ctx, row.EventID)
	if errorValue != nil {
		return false, errorValue
	}
	if !found {
		return false, nil
	}
	objectPath, ifMatch, ifNoneMatch := resolveCalendarPushTarget(row, event)
	ics, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		return false, errorValue
	}
	newETag, errorValue := service.guardedCalendarPut(ctx, client, row, objectPath, ics, ifMatch, ifNoneMatch)
	if errorValue != nil {
		if isCalDAVPreconditionFailed(errorValue) {
			return service.handleCalendarPushConflict(ctx, account, client, row, event, objectPath)
		}
		if isCalDAVObjectNotFound(errorValue) {
			if strings.TrimSpace(row.RemoteHref) == "" {
				return false, errorValue
			}
			return service.reconcileCalendarRemoteDeletionDuringPush(ctx, account, client, row, event, time.Now().UTC())
		}
		return false, errorValue
	}
	return true, service.applyCalendarPushSuccess(ctx, row, event, event, objectPath, newETag, ics)
}

func (service *Service) handleCalendarPushConflict(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow, localEvent calendarEvent, objectPath string) (bool, error) {
	remoteObject, errorValue := service.guardedCalendarGet(ctx, client, row, objectPath)
	if errorValue != nil {
		if isCalDAVObjectNotFound(errorValue) {
			return service.reconcileCalendarRemoteDeletionDuringPush(ctx, account, client, row, localEvent, time.Now().UTC())
		}
		log.Printf("calendar push conflict fetch failed for %s: %v", localEvent.UID, errorValue)
		return false, errorValue
	}
	remoteEvent, errorValue := decodeRemoteCalendarObject(remoteObject, account.AccountEmail)
	if errorValue != nil {
		log.Printf("calendar push conflict decode failed for %s: %v", localEvent.UID, errorValue)
		return false, errorValue
	}
	if errorValue := service.recordCalendarFieldConflicts(ctx, localEvent, remoteEvent, row.ChangedFields); errorValue != nil {
		return false, errorValue
	}
	previousRemote := decodeCalendarEventFromRawICS(localEvent.RawICS, localEvent.RemoteHref, localEvent.CreatedByEmail)
	remoteChangedFields := diffCalendarEventFields(previousRemote, remoteEvent)
	fieldChangedAt := row.FieldChangedAt
	if len(fieldChangedAt) == 0 {
		fieldChangedAt = calendarOutboxFieldChangedAt(row)
	}
	localWinningFields := selectCalendarLocalWinningFields(row.ChangedFields, remoteChangedFields, fieldChangedAt, parseCalendarConflictTime(remoteEvent.RemoteModifiedAt))
	mergedEvent := mergeCalendarEventChanges(remoteEvent, localEvent, localWinningFields)
	mergedEvent = preserveCalendarInternalParticipants(mergedEvent, localEvent, localWinningFields)
	mergedEvent.ID = localEvent.ID
	mergedEvent.CreatedByEmail = localEvent.CreatedByEmail
	mergedEvent.CreatedByName = localEvent.CreatedByName
	mergedEvent.UpdatedByEmail = localEvent.UpdatedByEmail
	mergedEvent.UpdatedByName = localEvent.UpdatedByName
	mergedEvent.UpdatedByAt = localEvent.UpdatedByAt
	mergedEvent.MattermostPostID = localEvent.MattermostPostID
	if len(localWinningFields) == 0 {
		return false, service.applyCalendarPushSuccess(ctx, row, localEvent, mergedEvent, remoteObject.Path, remoteObject.ETag, remoteObject.Data)
	}
	mergedICS, errorValue := encodeEventToICS(mergedEvent)
	if errorValue != nil {
		return false, errorValue
	}
	newETag, errorValue := service.guardedCalendarPut(ctx, client, row, remoteObject.Path, mergedICS, remoteObject.ETag, "")
	if errorValue != nil {
		if isCalDAVPreconditionFailed(errorValue) {
			log.Printf("calendar push retry precondition failed for %s — outbox row will retry", localEvent.UID)
		}
		return false, errorValue
	}
	slog.InfoContext(ctx, "calendar push conflict resolved", "event_uid", localEvent.UID, "local_winning_field_count", len(localWinningFields))
	return true, service.applyCalendarPushSuccess(ctx, row, localEvent, mergedEvent, remoteObject.Path, newETag, mergedICS)
}

func resolveCalendarPushTarget(row calendarOutboxRow, event calendarEvent) (string, string, string) {
	objectPath := row.RemoteHref
	ifMatch := row.IfMatchETag
	ifNoneMatch := ""
	if strings.TrimSpace(objectPath) == "" {
		objectPath = strings.TrimRight(row.TargetCalendarURL, "/") + "/" + event.UID + ".ics"
		ifNoneMatch = caldavWildcardETag
		ifMatch = ""
	}
	return objectPath, ifMatch, ifNoneMatch
}

func (service *Service) pushCalendarOutboxDelete(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow) (bool, error) {
	return service.pushCalendarOutboxDeleteWithRecovery(ctx, account, client, row)
}
