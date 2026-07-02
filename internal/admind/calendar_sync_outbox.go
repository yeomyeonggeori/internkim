package admind

import (
	"context"
	"strings"
)

func (service *Service) enqueueCalendarOutboxForWrite(ctx context.Context, event calendarEvent, changedFields []string) error {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return errorValue
	}
	if !found {
		return nil
	}
	if activeRemoteCalendarTarget(account).CalendarURL == "" {
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
	if strings.TrimSpace(event.RemoteHref) == "" && activeRemoteCalendarTarget(account).CalendarURL == "" {
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
