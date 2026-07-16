package admind

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var errCalendarPushTargetChanged = errors.New("calendar push target changed")

type calDAVPutResponseETagClient interface {
	putCalendarObjectResponseETag(ctx context.Context, objectPath string, encoded []byte, ifMatch string, ifNoneMatch string) (string, error)
}

func (service *Service) calendarPushTargetIsCurrent(ctx context.Context, row calendarOutboxRow) (bool, error) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	return service.calendarOutboxTargetIsActive(ctx, row)
}

func (service *Service) guardedCalendarPut(ctx context.Context, client calDAVPushClient, row calendarOutboxRow, objectPath string, encoded []byte, ifMatch string, ifNoneMatch string) (string, error) {
	isCurrentTarget, errorValue := service.prepareCalendarPutIntent(ctx, row, time.Now().UTC())
	if errorValue != nil {
		return "", errorValue
	}
	if !isCurrentTarget {
		return "", errCalendarPushTargetChanged
	}
	responseClient, canGuardRecovery := client.(calDAVPutResponseETagClient)
	if !canGuardRecovery {
		return client.putCalendarObject(ctx, objectPath, encoded, ifMatch, ifNoneMatch)
	}
	responseETag, errorValue := responseClient.putCalendarObjectResponseETag(ctx, objectPath, encoded, ifMatch, ifNoneMatch)
	if errorValue != nil || strings.TrimSpace(responseETag) != "" {
		return responseETag, errorValue
	}
	remoteObject, errorValue := service.guardedCalendarGet(ctx, client, row, objectPath)
	if errorValue != nil {
		return "", fmt.Errorf("recover canonical ETag after CalDAV PUT %s: %v", objectPath, errorValue)
	}
	remoteETag := strings.TrimSpace(remoteObject.ETag)
	if remoteETag == "" {
		return "", fmt.Errorf("recover canonical ETag after CalDAV PUT %s: GET response ETag is empty", objectPath)
	}
	return remoteETag, nil
}

func (service *Service) prepareCalendarPutIntent(ctx context.Context, row calendarOutboxRow, createdAt time.Time) (bool, error) {
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	database, errorValue := service.openCalendarDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return false, errorValue
	}
	activeTarget, errorValue := readActiveCalendarOutboxTargetURLWithRunner(ctx, transaction, row.AccountID)
	if errorValue != nil {
		_ = transaction.Rollback()
		return false, errorValue
	}
	if canonicalCalendarTargetURL(row.TargetCalendarURL) != activeTarget {
		_ = transaction.Rollback()
		return false, nil
	}
	if errorValue := persistCalendarPushObservationFenceWithRunner(ctx, transaction, row.AccountID, row.TargetCalendarURL, row.EventUID, createdAt); errorValue != nil {
		_ = transaction.Rollback()
		return false, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return false, errorValue
	}
	return true, nil
}

func (service *Service) guardedCalendarDelete(ctx context.Context, client calDAVPushClient, row calendarOutboxRow, objectPath string, ifMatch string) error {
	isCurrentTarget, errorValue := service.calendarPushTargetIsCurrent(ctx, row)
	if errorValue != nil {
		return errorValue
	}
	if !isCurrentTarget {
		return errCalendarPushTargetChanged
	}
	return client.deleteCalendarObject(ctx, objectPath, ifMatch)
}

func (service *Service) guardedCalendarGet(ctx context.Context, client calDAVPushClient, row calendarOutboxRow, objectPath string) (calDAVCalendarObject, error) {
	isCurrentTarget, errorValue := service.calendarPushTargetIsCurrent(ctx, row)
	if errorValue != nil {
		return calDAVCalendarObject{}, errorValue
	}
	if !isCurrentTarget {
		return calDAVCalendarObject{}, errCalendarPushTargetChanged
	}
	return client.getCalendarObject(ctx, objectPath)
}
