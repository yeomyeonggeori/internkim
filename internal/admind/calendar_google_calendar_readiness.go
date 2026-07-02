package admind

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	googleCalendarEventsEndpointBase          = "https://www.googleapis.com/calendar/v3/calendars"
	googleCalendarEventsProbeResponseMaxBytes = 1 << 20
)

func (service *Service) verifyGoogleCalendarSelectionReady(ctx context.Context, account remoteCalendarAccount, calendar googleCalendarListEntry) error {
	if !isWritableRemoteCalendarAccessRole(calendar.AccessRole) {
		return fmt.Errorf("google calendar %s is not writable", strings.TrimSpace(calendar.CalendarID))
	}
	tokenSource, errorValue := service.googleOAuthTokenSource(ctx, account, "")
	if errorValue != nil {
		return errorValue
	}
	token, errorValue := tokenSource.Token()
	if errorValue != nil {
		return googleCalendarAuthorizationError{Cause: errorValue}
	}
	return service.fetchGoogleCalendarEventsProbe(ctx, token, calendar.CalendarID)
}

func (service *Service) fetchGoogleCalendarEventsProbe(ctx context.Context, token interface{ SetAuthHeader(*http.Request) }, calendarID string) error {
	requestURL, errorValue := googleCalendarEventsProbeURL(calendarID)
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return errorValue
	}
	token.SetAuthHeader(request)
	response, errorValue := service.googleOAuthHTTPClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	body, errorValue := io.ReadAll(io.LimitReader(response.Body, googleCalendarEventsProbeResponseMaxBytes))
	if errorValue != nil {
		return errorValue
	}
	if response.StatusCode == http.StatusOK {
		return nil
	}
	statusError := fmt.Errorf("google calendar events list status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return googleCalendarAuthorizationError{Cause: statusError}
	}
	return statusError
}

func googleCalendarEventsProbeURL(calendarID string) (string, error) {
	requestURL, errorValue := url.Parse(googleCalendarEventsEndpointBase + "/" + url.PathEscape(strings.TrimSpace(calendarID)) + "/events")
	if errorValue != nil {
		return "", errorValue
	}
	query := requestURL.Query()
	query.Set("maxResults", "1")
	query.Set("singleEvents", "true")
	query.Set("fields", "items(id)")
	requestURL.RawQuery = query.Encode()
	return requestURL.String(), nil
}
