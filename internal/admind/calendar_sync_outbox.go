package admind

import "context"

func (service *Service) prepareCalendarOutboxForWrite(ctx context.Context, event calendarEvent, changedFields []string) (calendarOutboxRow, bool, error) {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(ctx, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return calendarOutboxRow{}, false, errorValue
	}
	if !found {
		return calendarOutboxRow{}, false, nil
	}
	if activeRemoteCalendarTarget(account).CalendarURL == "" {
		return calendarOutboxRow{}, false, calendarOutboxTargetUnavailableError(account.ID)
	}
	if len(changedFields) == 0 {
		return calendarOutboxRow{}, false, nil
	}
	return calendarOutboxRow{
		AccountID:     account.ID,
		EventID:       event.ID,
		EventUID:      event.UID,
		Operation:     calendarOutboxOperationPut,
		IfMatchETag:   event.RemoteETag,
		RemoteHref:    event.RemoteHref,
		ChangedFields: changedFields,
	}, true, nil
}

func (service *Service) prepareCalendarOutboxForDelete(ctx context.Context, event calendarEvent) (calendarOutboxRow, bool, error) {
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return calendarOutboxRow{}, false, errorValue
	}
	defer database.Close()
	return prepareCalendarOutboxForDeleteWithRunner(ctx, database, event)
}

func prepareCalendarOutboxForDeleteWithRunner(ctx context.Context, queryRunner calendarSQLRunner, event calendarEvent) (calendarOutboxRow, bool, error) {
	account, found, errorValue := readRemoteCalendarAccountByProviderWithRunner(ctx, queryRunner, remoteCalendarProviderGoogle)
	if errorValue != nil {
		return calendarOutboxRow{}, false, errorValue
	}
	if !found {
		return calendarOutboxRow{}, false, nil
	}
	if activeRemoteCalendarTarget(account).CalendarURL == "" {
		return calendarOutboxRow{}, false, calendarOutboxTargetUnavailableError(account.ID)
	}
	return calendarOutboxRow{
		AccountID:         account.ID,
		EventID:           event.ID,
		EventUID:          event.UID,
		Operation:         calendarOutboxOperationDelete,
		IfMatchETag:       event.RemoteETag,
		RemoteHref:        event.RemoteHref,
		TargetCalendarURL: activeRemoteCalendarTarget(account).CalendarURL,
	}, true, nil
}
