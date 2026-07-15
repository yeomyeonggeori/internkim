package admind

import (
	"context"
	"fmt"
	"log"
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
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
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
	rows, errorValue := service.listPendingCalendarOutbox(ctx, account.ID)
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
	hasAuthError := false
	hasSuccessfulRemoteOperation := false
	for _, row := range rows {
		if !calendarOutboxPutTargetsRemoteTarget(row, target) {
			if errorValue := service.deleteCalendarOutbox(ctx, row.ID); errorValue != nil {
				log.Printf("outbox retarget cleanup %d: %v", row.ID, errorValue)
			}
			continue
		}
		if row.AttemptCount >= calendarOutboxMaxAttempts {
			log.Printf("calendar outbox row %d (event %s) blocked after %d attempts: %s",
				row.ID, row.EventUID, row.AttemptCount, row.LastError)
			failedAt := time.Now().UTC().Format(time.RFC3339Nano)
			if errorValue := service.markCalendarOutboxBlocked(ctx, row.ID, failedAt, row.LastError); errorValue != nil {
				log.Printf("outbox block %d: %v", row.ID, errorValue)
			}
			continue
		}
		pushed, errorValue := service.processCalendarOutboxRow(ctx, account, client, row)
		if errorValue != nil {
			log.Printf("calendar outbox row %d failed: %v", row.ID, errorValue)
			if isCalendarAuthError(errorValue) {
				hasAuthError = true
				service.markRemoteCalendarAccountAuthError(ctx, account, errorValue)
			}
			if outboxErr := service.markCalendarOutboxAttempt(ctx, row.ID, errorValue.Error()); outboxErr != nil {
				log.Printf("mark outbox attempt: %v", outboxErr)
			}
			continue
		}
		if pushed || (row.Operation == calendarOutboxOperationDelete && strings.TrimSpace(row.RemoteHref) != "") {
			hasSuccessfulRemoteOperation = true
		}
		if errorValue := service.deleteCalendarOutbox(ctx, row.ID); errorValue != nil {
			log.Printf("outbox cleanup %d: %v", row.ID, errorValue)
		}
		if pushed && strings.TrimSpace(row.EventUID) != "" {
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
		return false, service.pushCalendarOutboxDelete(ctx, account, client, row)
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
	objectPath, ifMatch, ifNoneMatch := resolveCalendarPushTarget(account, row, event)
	ics, errorValue := encodeEventToICS(event)
	if errorValue != nil {
		return false, errorValue
	}
	newETag, errorValue := client.putCalendarObject(ctx, objectPath, ics, ifMatch, ifNoneMatch)
	if errorValue != nil {
		if isCalDAVPreconditionFailed(errorValue) {
			return service.handleCalendarPushConflict(ctx, account, client, row, event, objectPath)
		}
		if isCalDAVObjectNotFound(errorValue) {
			return service.reconcileCalendarRemoteDeletionDuringPush(ctx, account, client, row, event, time.Now().UTC())
		}
		return false, errorValue
	}
	return true, service.applyCalendarPushSuccess(ctx, event, objectPath, newETag, ics)
}

func (service *Service) handleCalendarPushConflict(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow, localEvent calendarEvent, objectPath string) (bool, error) {
	remoteObject, errorValue := client.getCalendarObject(ctx, objectPath)
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
	localChangedAt := parseCalendarConflictTime(row.CreatedAt)
	fieldChangedAt := map[string]time.Time{}
	for _, field := range row.ChangedFields {
		fieldChangedAt[field] = localChangedAt
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
		return false, service.applyCalendarPushSuccess(ctx, mergedEvent, remoteObject.Path, remoteObject.ETag, remoteObject.Data)
	}
	mergedICS, errorValue := encodeEventToICS(mergedEvent)
	if errorValue != nil {
		return false, errorValue
	}
	newETag, errorValue := client.putCalendarObject(ctx, remoteObject.Path, mergedICS, remoteObject.ETag, "")
	if errorValue != nil {
		if isCalDAVPreconditionFailed(errorValue) {
			log.Printf("calendar push retry precondition failed for %s — outbox row will retry", localEvent.UID)
		}
		return false, errorValue
	}
	log.Printf("calendar push conflict resolved for %s — merged %d local field(s) over remote update", localEvent.UID, len(localWinningFields))
	return true, service.applyCalendarPushSuccess(ctx, mergedEvent, remoteObject.Path, newETag, mergedICS)
}

func resolveCalendarPushTarget(account remoteCalendarAccount, row calendarOutboxRow, event calendarEvent) (string, string, string) {
	objectPath := row.RemoteHref
	ifMatch := row.IfMatchETag
	ifNoneMatch := ""
	if strings.TrimSpace(objectPath) == "" {
		objectPath = strings.TrimRight(activeRemoteCalendarTarget(account).CalendarURL, "/") + "/" + event.UID + ".ics"
		ifNoneMatch = caldavWildcardETag
		ifMatch = ""
	}
	return objectPath, ifMatch, ifNoneMatch
}

func (service *Service) applyCalendarPushSuccess(ctx context.Context, event calendarEvent, objectPath string, newETag string, rawICS []byte) error {
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = objectPath
	event.RemoteETag = newETag
	event.RawICS = string(rawICS)
	return service.writeCalendarEventWithSource(ctx, event, calendarSourcePull)
}

func (service *Service) pushCalendarOutboxDelete(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow) error {
	if strings.TrimSpace(row.RemoteHref) == "" {
		return nil
	}
	errorValue := client.deleteCalendarObject(ctx, row.RemoteHref, row.IfMatchETag)
	if errorValue != nil && isCalDAVPreconditionFailed(errorValue) {
		return service.reconcileCalendarLocalDeletionDuringPush(ctx, account, client, row, time.Now().UTC())
	}
	return errorValue
}
