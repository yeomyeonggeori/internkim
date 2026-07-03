package admind

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type googleCalendarReadinessError struct {
	Status string
	Cause  error
}

func (errorValue googleCalendarReadinessError) Error() string {
	return errorValue.Cause.Error()
}

func (errorValue googleCalendarReadinessError) Unwrap() error {
	return errorValue.Cause
}

func googleCalendarReadinessStatusFromError(errorValue error) (string, bool) {
	var readinessError googleCalendarReadinessError
	if errors.As(errorValue, &readinessError) {
		return readinessError.Status, true
	}
	return "", false
}

func (service *Service) verifyGoogleCalendarSelectionReady(ctx context.Context, account remoteCalendarAccount, calendar googleCalendarListEntry) error {
	if !calendar.CanSelect {
		return googleCalendarReadinessError{
			Status: calendarReadinessStatusWritePermissionRequired,
			Cause:  fmt.Errorf("google calendar %s is not writable", strings.TrimSpace(calendar.CalendarID)),
		}
	}
	client, errorValue := service.newGoogleCalDAVClient(ctx, account)
	if errorValue != nil {
		return errorValue
	}
	return service.fetchGoogleCalendarCalDAVProbe(ctx, client, calendar.CalendarURL)
}

func (service *Service) fetchGoogleCalendarCalDAVProbe(ctx context.Context, client *outboundCalDAVClient, calendarURL string) error {
	calendarURL = strings.TrimSpace(calendarURL)
	if calendarURL == "" {
		return googleCalendarReadinessError{
			Status: calendarReadinessStatusCalendarInaccessible,
			Cause:  errors.New("google calendar URL is required"),
		}
	}
	if _, errorValue := client.fetchCalendarCTag(ctx, calendarURL); errorValue != nil {
		if isCalendarAuthError(errorValue) {
			return googleCalendarAuthorizationError{Cause: errorValue}
		}
		if isCalendarInaccessibleError(errorValue) {
			return googleCalendarReadinessError{Status: calendarReadinessStatusCalendarInaccessible, Cause: errorValue}
		}
		return errorValue
	}
	return nil
}

func isCalendarInaccessibleError(errorValue error) bool {
	if errorValue == nil {
		return false
	}
	message := strings.ToLower(errorValue.Error())
	return strings.Contains(message, " status 403") || strings.Contains(message, " status 404")
}
