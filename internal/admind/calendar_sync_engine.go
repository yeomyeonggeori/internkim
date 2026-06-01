package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

const (
	calendarOutboxMaxAttempts = 10
)

type calDAVPullClient interface {
	discoverPrincipalURL(ctx context.Context) (string, error)
	discoverHomeSetURL(ctx context.Context, principalURL string) (string, error)
	listCalendars(ctx context.Context, homeSetURL string) ([]calDAVCalendarInfo, error)
	fetchCalendarCTag(ctx context.Context, calendarPath string) (string, error)
	queryAllCalendarEvents(ctx context.Context, calendarPath string) ([]calDAVCalendarObject, error)
}

type calDAVPushClient interface {
	putCalendarObject(ctx context.Context, objectPath string, ics []byte, ifMatch string, ifNoneMatch string) (string, error)
	deleteCalendarObject(ctx context.Context, objectPath string, ifMatch string) error
	getCalendarObject(ctx context.Context, objectPath string) (calDAVCalendarObject, error)
}

func (service *Service) pullGoogleCalendarChanges(ctx context.Context) (bool, error) {
	return service.pullCalendarChangesForProvider(ctx, googleCalendarProvider{}, false, nil)
}

func (service *Service) pullGoogleCalendarChangesWithProtection(ctx context.Context, protectedUIDs map[string]struct{}) (bool, error) {
	return service.pullCalendarChangesForProvider(ctx, googleCalendarProvider{}, false, protectedUIDs)
}

func (service *Service) pullCalendarChangesForProvider(ctx context.Context, provider calendarProvider, forceQuery bool, protectedUIDs map[string]struct{}) (bool, error) {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, provider.Name())
	if errorValue != nil {
		return false, errorValue
	}
	if !found {
		return false, nil
	}
	httpClient, errorValue := provider.BuildHTTPClient(ctx, service, account)
	if errorValue != nil {
		service.markRemoteCalendarAccountAuthError(ctx, account, errorValue)
		return false, fmt.Errorf("build http client: %w", errorValue)
	}
	client, errorValue := newOutboundCalDAVClient(provider.Endpoint(account), httpClient)
	if errorValue != nil {
		return false, errorValue
	}
	changed, errorValue := service.runCalendarPull(ctx, provider, account, client, forceQuery, protectedUIDs)
	if errorValue != nil {
		if isCalendarAuthError(errorValue) {
			service.markRemoteCalendarAccountAuthError(ctx, account, errorValue)
		}
		return false, errorValue
	}
	service.clearRemoteCalendarAccountAuthError(ctx, account)
	return changed, nil
}

func (service *Service) runGoogleCalendarPull(ctx context.Context, account remoteCalendarAccount, client calDAVPullClient) (bool, error) {
	return service.runCalendarPull(ctx, googleCalendarProvider{}, account, client, false, nil)
}

func (service *Service) runCalendarPull(ctx context.Context, provider calendarProvider, account remoteCalendarAccount, client calDAVPullClient, forceQuery bool, protectedUIDs map[string]struct{}) (bool, error) {
	if strings.TrimSpace(account.DefaultCalendarURL) == "" {
		discovered, errorValue := provider.Discover(ctx, service, account)
		if errorValue != nil {
			return false, errorValue
		}
		account = discovered
	}
	serverCTag, errorValue := client.fetchCalendarCTag(ctx, account.DefaultCalendarURL)
	if errorValue != nil {
		return false, fmt.Errorf("fetch ctag: %w", errorValue)
	}
	if !forceQuery && serverCTag != "" && serverCTag == account.DefaultCalendarCTag {
		return false, nil
	}
	objects, errorValue := client.queryAllCalendarEvents(ctx, account.DefaultCalendarURL)
	if errorValue != nil {
		return false, fmt.Errorf("query calendar: %w", errorValue)
	}
	if errorValue := service.reconcileGoogleCalendarPull(ctx, account, objects, protectedUIDs); errorValue != nil {
		return false, errorValue
	}
	if serverCTag != "" && serverCTag != account.DefaultCalendarCTag {
		account.DefaultCalendarCTag = serverCTag
		if _, errorValue := service.upsertRemoteCalendarAccount(ctx, account); errorValue != nil {
			log.Printf("update calendar ctag: %v", errorValue)
		}
	}
	return true, nil
}

func (service *Service) reconcileGoogleCalendarPull(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, protectedUIDs map[string]struct{}) error {
	activeEvents, errorValue := service.readCalendarEvents(ctx, time.Time{}, time.Time{})
	if errorValue != nil {
		return errorValue
	}
	softDeletedEvents, errorValue := service.readSoftDeletedCalendarEvents(ctx)
	if errorValue != nil {
		return errorValue
	}
	existingByUID := map[string]calendarEvent{}
	for _, event := range activeEvents {
		existingByUID[event.UID] = event
	}
	for _, event := range softDeletedEvents {
		if _, alreadyActive := existingByUID[event.UID]; alreadyActive {
			continue
		}
		existingByUID[event.UID] = event
	}
	remoteUIDs, errorValue := service.applyPulledRemoteEvents(ctx, account, remoteObjects, existingByUID)
	if errorValue != nil {
		return errorValue
	}
	service.markCalendarPushUIDsObserved(remoteUIDs)
	return service.softDeleteMissingRemoteEvents(ctx, activeEvents, remoteUIDs, protectedUIDs)
}

func (service *Service) applyPulledRemoteEvents(ctx context.Context, account remoteCalendarAccount, remoteObjects []calDAVCalendarObject, existingByUID map[string]calendarEvent) (map[string]struct{}, error) {
	remoteUIDs := map[string]struct{}{}
	for _, object := range remoteObjects {
		event, errorValue := decodeRemoteCalendarObject(object, account.AccountEmail)
		if errorValue != nil {
			log.Printf("calendar pull decode failed for %s: %v", object.Path, errorValue)
			continue
		}
		remoteUIDs[event.UID] = struct{}{}
		previous, found := existingByUID[event.UID]
		if found {
			if previous.RemoteETag == event.RemoteETag && previous.RemoteETag != "" {
				continue
			}
			event.ID = previous.ID
		} else {
			event.ID = randomHex(16)
		}
		if errorValue := service.writeCalendarEventWithSource(ctx, event, calendarSourcePull); errorValue != nil {
			return nil, fmt.Errorf("write pulled event %s: %w", event.UID, errorValue)
		}
	}
	return remoteUIDs, nil
}

func (service *Service) softDeleteMissingRemoteEvents(ctx context.Context, allEvents []calendarEvent, remoteUIDs map[string]struct{}, protectedUIDs map[string]struct{}) error {
	for _, event := range allEvents {
		if event.RemoteSource != remoteCalendarProviderGoogle {
			continue
		}
		if _, kept := remoteUIDs[event.UID]; kept {
			continue
		}
		if _, protected := protectedUIDs[event.UID]; protected {
			continue
		}
		if errorValue := service.softDeleteCalendarEventWithSource(ctx, event.ID, calendarSourcePull); errorValue != nil {
			return fmt.Errorf("soft delete %s: %w", event.ID, errorValue)
		}
	}
	return nil
}

func eventUpdatedWithin(updatedAt string, now time.Time, window time.Duration) bool {
	trimmed := strings.TrimSpace(updatedAt)
	if trimmed == "" {
		return false
	}
	parsed, errorValue := time.Parse(time.RFC3339Nano, trimmed)
	if errorValue != nil {
		return false
	}
	return now.Sub(parsed) < window
}

const (
	calendarFieldTitle             = "title"
	calendarFieldDescription       = "description"
	calendarFieldLocation          = "location"
	calendarFieldStart             = "start"
	calendarFieldEnd               = "end"
	calendarFieldTimeZone          = "timeZone"
	calendarFieldIsAllDay          = "isAllDay"
	calendarFieldColor             = "color"
	calendarFieldReminderLeadHours = "reminderLeadHours"
)

func calendarAllUserEditableFields() []string {
	return []string{
		calendarFieldTitle,
		calendarFieldDescription,
		calendarFieldLocation,
		calendarFieldStart,
		calendarFieldEnd,
		calendarFieldTimeZone,
		calendarFieldIsAllDay,
		calendarFieldColor,
		calendarFieldReminderLeadHours,
	}
}

func diffCalendarEventFields(previous calendarEvent, current calendarEvent) []string {
	if strings.TrimSpace(previous.ID) == "" {
		return calendarAllUserEditableFields()
	}
	fields := []string{}
	if previous.Title != current.Title {
		fields = append(fields, calendarFieldTitle)
	}
	if previous.Description != current.Description {
		fields = append(fields, calendarFieldDescription)
	}
	if previous.Location != current.Location {
		fields = append(fields, calendarFieldLocation)
	}
	if previous.StartISO != current.StartISO {
		fields = append(fields, calendarFieldStart)
	}
	if previous.EndISO != current.EndISO {
		fields = append(fields, calendarFieldEnd)
	}
	if previous.TimeZone != current.TimeZone {
		fields = append(fields, calendarFieldTimeZone)
	}
	if previous.IsAllDay != current.IsAllDay {
		fields = append(fields, calendarFieldIsAllDay)
	}
	if previous.Color != current.Color {
		fields = append(fields, calendarFieldColor)
	}
	if previous.ReminderLeadHours != current.ReminderLeadHours {
		fields = append(fields, calendarFieldReminderLeadHours)
	}
	return fields
}

func (service *Service) enqueueCalendarOutboxForWrite(ctx context.Context, event calendarEvent, changedFields []string) error {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return nil
	}
	if strings.TrimSpace(account.DefaultCalendarURL) == "" {
		return nil
	}
	if len(changedFields) == 0 {
		return nil
	}
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID:     account.ID,
		EventID:       event.ID,
		EventUID:      event.UID,
		Operation:     calendarOutboxOperationPut,
		IfMatchETag:   event.RemoteETag,
		RemoteHref:    event.RemoteHref,
		ChangedFields: changedFields,
	}); errorValue != nil {
		return errorValue
	}
	service.signalCalendarSyncWakeUp()
	return nil
}

func (service *Service) enqueueCalendarOutboxForDelete(ctx context.Context, event calendarEvent) error {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return nil
	}
	if strings.TrimSpace(event.RemoteHref) == "" && strings.TrimSpace(account.DefaultCalendarURL) == "" {
		return nil
	}
	if errorValue := service.enqueueCalendarOutbox(ctx, calendarOutboxRow{
		AccountID:   account.ID,
		EventID:     event.ID,
		EventUID:    event.UID,
		Operation:   calendarOutboxOperationDelete,
		IfMatchETag: event.RemoteETag,
		RemoteHref:  event.RemoteHref,
	}); errorValue != nil {
		return errorValue
	}
	service.signalCalendarSyncWakeUp()
	return nil
}

func (service *Service) pushPendingCalendarOutbox(ctx context.Context) (map[string]struct{}, error) {
	return service.pushPendingCalendarOutboxForProvider(ctx, googleCalendarProvider{})
}

func (service *Service) pushPendingCalendarOutboxForProvider(ctx context.Context, provider calendarProvider) (map[string]struct{}, error) {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, provider.Name())
	if errorValue != nil {
		return nil, errorValue
	}
	if !found || strings.TrimSpace(account.DefaultCalendarURL) == "" {
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
	for _, row := range rows {
		if row.AttemptCount >= calendarOutboxMaxAttempts {
			log.Printf("calendar outbox row %d (event %s) dropped after %d attempts: %s",
				row.ID, row.EventUID, row.AttemptCount, row.LastError)
			if errorValue := service.deleteCalendarOutbox(ctx, row.ID); errorValue != nil {
				log.Printf("outbox cap drop %d: %v", row.ID, errorValue)
			}
			continue
		}
		pushed, errorValue := service.processCalendarOutboxRow(ctx, account, client, row)
		if errorValue != nil {
			log.Printf("calendar outbox row %d failed: %v", row.ID, errorValue)
			if outboxErr := service.markCalendarOutboxAttempt(ctx, row.ID, errorValue.Error()); outboxErr != nil {
				log.Printf("mark outbox attempt: %v", outboxErr)
			}
			continue
		}
		if errorValue := service.deleteCalendarOutbox(ctx, row.ID); errorValue != nil {
			log.Printf("outbox cleanup %d: %v", row.ID, errorValue)
		}
		if pushed && strings.TrimSpace(row.EventUID) != "" {
			pushedUIDs[row.EventUID] = struct{}{}
		}
	}
	return pushedUIDs, nil
}

func (service *Service) processCalendarOutboxRow(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow) (bool, error) {
	switch row.Operation {
	case calendarOutboxOperationPut:
		return service.pushCalendarOutboxPut(ctx, account, client, row)
	case calendarOutboxOperationDelete:
		return false, service.pushCalendarOutboxDelete(ctx, client, row)
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
		return false, errorValue
	}
	return true, service.applyCalendarPushSuccess(ctx, event, objectPath, newETag)
}

func (service *Service) handleCalendarPushConflict(ctx context.Context, account remoteCalendarAccount, client calDAVPushClient, row calendarOutboxRow, localEvent calendarEvent, objectPath string) (bool, error) {
	remoteObject, errorValue := client.getCalendarObject(ctx, objectPath)
	if errorValue != nil {
		log.Printf("calendar push conflict fetch failed for %s: %v", localEvent.UID, errorValue)
		return false, errorValue
	}
	remoteEvent, errorValue := decodeRemoteCalendarObject(remoteObject, account.AccountEmail)
	if errorValue != nil {
		log.Printf("calendar push conflict decode failed for %s: %v", localEvent.UID, errorValue)
		return false, errorValue
	}
	service.recordCalendarFieldConflicts(ctx, localEvent, remoteEvent, row.ChangedFields)
	mergedEvent := mergeCalendarEventChanges(remoteEvent, localEvent, row.ChangedFields)
	mergedEvent.ID = localEvent.ID
	mergedEvent.CreatedByEmail = localEvent.CreatedByEmail
	mergedEvent.CreatedByName = localEvent.CreatedByName
	mergedEvent.UpdatedByEmail = localEvent.UpdatedByEmail
	mergedEvent.UpdatedByName = localEvent.UpdatedByName
	mergedEvent.UpdatedByAt = localEvent.UpdatedByAt
	mergedEvent.MattermostPostID = localEvent.MattermostPostID
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
	log.Printf("calendar push conflict resolved for %s — merged %d local field(s) over remote update", localEvent.UID, len(row.ChangedFields))
	return true, service.applyCalendarPushSuccess(ctx, mergedEvent, remoteObject.Path, newETag)
}

func (service *Service) recordCalendarFieldConflicts(ctx context.Context, localEvent calendarEvent, remoteEvent calendarEvent, localChangedFields []string) {
	if len(localChangedFields) == 0 {
		return
	}
	previousRemote := decodeCalendarEventFromRawICS(localEvent.RawICS, localEvent.RemoteHref, localEvent.CreatedByEmail)
	remoteChangedFields := diffCalendarEventFields(previousRemote, remoteEvent)
	collisions := intersectCalendarFields(localChangedFields, remoteChangedFields)
	if len(collisions) == 0 {
		return
	}
	for _, field := range collisions {
		localValue := calendarEventFieldStringValue(localEvent, field)
		remoteValue := calendarEventFieldStringValue(remoteEvent, field)
		if errorValue := service.recordCalendarConflict(ctx, localEvent.ID, localEvent.UID, field, localValue, remoteValue); errorValue != nil {
			log.Printf("record calendar conflict %s/%s failed: %v", localEvent.UID, field, errorValue)
		}
	}
}

func decodeCalendarEventFromRawICS(rawICS string, remoteHref string, accountEmail string) calendarEvent {
	trimmed := strings.TrimSpace(rawICS)
	if trimmed == "" {
		return calendarEvent{}
	}
	decoded, errorValue := decodeCalendarObject(trimmed)
	if errorValue != nil {
		return calendarEvent{}
	}
	event, errorValue := calendarEventFromCalendarObject(remoteHref, decoded, accountEmail)
	if errorValue != nil {
		return calendarEvent{}
	}
	return event
}

func mergeCalendarEventChanges(remote calendarEvent, local calendarEvent, changedFields []string) calendarEvent {
	merged := remote
	for _, field := range changedFields {
		switch field {
		case calendarFieldTitle:
			merged.Title = local.Title
		case calendarFieldDescription:
			merged.Description = local.Description
		case calendarFieldLocation:
			merged.Location = local.Location
		case calendarFieldStart:
			merged.StartISO = local.StartISO
		case calendarFieldEnd:
			merged.EndISO = local.EndISO
		case calendarFieldTimeZone:
			merged.TimeZone = local.TimeZone
		case calendarFieldIsAllDay:
			merged.IsAllDay = local.IsAllDay
		case calendarFieldColor:
			merged.Color = local.Color
		case calendarFieldReminderLeadHours:
			merged.ReminderLeadHours = local.ReminderLeadHours
		}
	}
	return merged
}

func intersectCalendarFields(left []string, right []string) []string {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	leftSet := map[string]struct{}{}
	for _, value := range left {
		leftSet[value] = struct{}{}
	}
	result := []string{}
	for _, value := range right {
		if _, ok := leftSet[value]; ok {
			result = append(result, value)
		}
	}
	return result
}

func resolveCalendarPushTarget(account remoteCalendarAccount, row calendarOutboxRow, event calendarEvent) (string, string, string) {
	objectPath := row.RemoteHref
	ifMatch := row.IfMatchETag
	ifNoneMatch := ""
	if strings.TrimSpace(objectPath) == "" {
		objectPath = strings.TrimRight(account.DefaultCalendarURL, "/") + "/" + event.UID + ".ics"
		ifNoneMatch = caldavWildcardETag
		ifMatch = ""
	}
	return objectPath, ifMatch, ifNoneMatch
}

func (service *Service) applyCalendarPushSuccess(ctx context.Context, event calendarEvent, objectPath string, newETag string) error {
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteHref = objectPath
	event.RemoteETag = newETag
	return service.writeCalendarEventWithSource(ctx, event, calendarSourcePull)
}

func (service *Service) pushCalendarOutboxDelete(ctx context.Context, client calDAVPushClient, row calendarOutboxRow) error {
	if strings.TrimSpace(row.RemoteHref) == "" {
		return nil
	}
	errorValue := client.deleteCalendarObject(ctx, row.RemoteHref, row.IfMatchETag)
	if errorValue != nil && isCalDAVPreconditionFailed(errorValue) {
		log.Printf("calendar push delete conflict on event %s — google version wins on next pull", row.EventUID)
		return nil
	}
	return errorValue
}

func encodeEventToICS(event calendarEvent) ([]byte, error) {
	calendar, errorValue := calendarObjectForEvent(event)
	if errorValue != nil {
		return nil, errorValue
	}
	var buffer bytes.Buffer
	if errorValue := ical.NewEncoder(&buffer).Encode(calendar); errorValue != nil {
		return nil, errorValue
	}
	return buffer.Bytes(), nil
}

func decodeRemoteCalendarObject(object calDAVCalendarObject, accountEmail string) (calendarEvent, error) {
	if len(object.Data) == 0 {
		return calendarEvent{}, errors.New("empty calendar object data")
	}
	calendar, errorValue := decodeCalendarObject(string(object.Data))
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	event, errorValue := calendarEventFromCalendarObject(object.Path, calendar, accountEmail)
	if errorValue != nil {
		return calendarEvent{}, errorValue
	}
	event.RemoteSource = remoteCalendarProviderGoogle
	event.RemoteETag = object.ETag
	event.RemoteHref = object.Path
	event.RawICS = string(object.Data)
	return event, nil
}

type calendarAccountStatusResponse struct {
	Connected       bool   `json:"connected"`
	Provider        string `json:"provider,omitempty"`
	AccountEmail    string `json:"accountEmail,omitempty"`
	DefaultCalendar string `json:"defaultCalendarURL,omitempty"`
	LastAuthError   string `json:"lastAuthError,omitempty"`
	LastAuthErrorAt string `json:"lastAuthErrorAt,omitempty"`
	NeedsReauth     bool   `json:"needsReauth"`
}

func (service *Service) serveCalendarAccountStatus(writer http.ResponseWriter, request *http.Request) {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(request.Context(), remoteCalendarProviderGoogle)
	if errorValue != nil {
		http.Error(writer, "failed to read account status", http.StatusInternalServerError)
		log.Printf("account status read: %v", errorValue)
		return
	}
	response := calendarAccountStatusResponse{Connected: found}
	if found {
		response.Provider = account.Provider
		response.AccountEmail = account.AccountEmail
		response.DefaultCalendar = account.DefaultCalendarURL
		response.LastAuthError = account.LastAuthError
		response.LastAuthErrorAt = account.LastAuthErrorAt
		response.NeedsReauth = strings.TrimSpace(account.LastAuthError) != ""
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if errorValue := json.NewEncoder(writer).Encode(response); errorValue != nil {
		log.Printf("account status encode: %v", errorValue)
	}
}
