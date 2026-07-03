package admind

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
)

const (
	googleCalendarSelectionDisabledReasonWritePermissionRequired = "write_permission_required"
	googleCalendarSelectionDisabledReasonUnsupportedCalendar     = "unsupported_calendar"
)

type googleCalendarListResponse struct {
	AccountEmail string                    `json:"accountEmail"`
	Calendars    []googleCalendarListEntry `json:"calendars"`
}

type googleCalendarListEntry struct {
	CalendarID              string `json:"calendarID"`
	Summary                 string `json:"summary"`
	AccessRole              string `json:"accessRole"`
	TimeZone                string `json:"timeZone,omitempty"`
	Primary                 bool   `json:"primary"`
	BackgroundColor         string `json:"backgroundColor,omitempty"`
	CanWrite                bool   `json:"canWrite"`
	CanSelect               bool   `json:"canSelect"`
	SelectionDisabledReason string `json:"selectionDisabledReason,omitempty"`
	CalendarURL             string `json:"-"`
}

type googleCalendarAuthorizationError struct {
	Cause error
}

func (errorValue googleCalendarAuthorizationError) Error() string {
	return errorValue.Cause.Error()
}

func (errorValue googleCalendarAuthorizationError) Unwrap() error {
	return errorValue.Cause
}

func (service *Service) serveGoogleCalendarList(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.canManageGoogleOAuth(request) {
		http.Error(responseWriter, "admin access required", http.StatusForbidden)
		return
	}
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(request.Context(), remoteCalendarProviderGoogle)
	if errorValue != nil {
		http.Error(responseWriter, "failed to read google calendar account", http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "google calendar account is not connected", http.StatusConflict)
		return
	}
	calendars, errorValue := service.listGoogleCalendars(request.Context(), account)
	if errorValue != nil {
		if isGoogleCalendarAuthorizationError(errorValue) || isCalendarAuthError(errorValue) {
			service.markRemoteCalendarAccountAuthError(request.Context(), account, errorValue)
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.clearRemoteCalendarAccountAuthError(request.Context(), account)
	service.writeJSON(responseWriter, googleCalendarListResponse{
		AccountEmail: strings.TrimSpace(account.AccountEmail),
		Calendars:    calendars,
	})
}

func (service *Service) listGoogleCalendars(ctx context.Context, account remoteCalendarAccount) ([]googleCalendarListEntry, error) {
	client, errorValue := service.newGoogleCalDAVClient(ctx, account)
	if errorValue != nil {
		return nil, errorValue
	}
	homeSetURL, errorValue := service.googleCalDAVHomeSetURL(ctx, account, client)
	if errorValue != nil {
		return nil, errorValue
	}
	calendars, errorValue := client.listCalendars(ctx, homeSetURL)
	if errorValue != nil {
		return nil, errorValue
	}
	return googleCalendarListEntriesFromCalDAV(client, calendars, account.AccountEmail)
}

func (service *Service) newGoogleCalDAVClient(ctx context.Context, account remoteCalendarAccount) (*outboundCalDAVClient, error) {
	tokenSource, errorValue := service.googleOAuthTokenSource(ctx, account, "")
	if errorValue != nil {
		return nil, errorValue
	}
	httpClient := newCalDAVBearerHTTPClient(service.googleOAuthHTTPClient(), tokenSource)
	return newOutboundCalDAVClient(googleCalendarProvider{}.Endpoint(account), httpClient)
}

func (service *Service) googleCalDAVHomeSetURL(ctx context.Context, account remoteCalendarAccount, client *outboundCalDAVClient) (string, error) {
	if homeSetURL := strings.TrimSpace(account.HomeSetURL); homeSetURL != "" {
		return homeSetURL, nil
	}
	if accountEmail := strings.TrimSpace(account.AccountEmail); accountEmail != "" {
		return googleCalDAVBaseURL + "/" + accountEmail + "/", nil
	}
	principalURL := strings.TrimSpace(account.PrincipalURL)
	if principalURL == "" {
		discoveredPrincipalURL, errorValue := client.discoverPrincipalURL(ctx)
		if errorValue != nil {
			return "", errorValue
		}
		principalURL = discoveredPrincipalURL
	}
	homeSetURL, errorValue := client.discoverHomeSetURL(ctx, principalURL)
	if errorValue != nil {
		return "", errorValue
	}
	return homeSetURL, nil
}

func (service *Service) findSelectableGoogleCalendar(ctx context.Context, account remoteCalendarAccount, calendarID string) (googleCalendarListEntry, bool, error) {
	calendars, errorValue := service.listGoogleCalendars(ctx, account)
	if errorValue != nil {
		return googleCalendarListEntry{}, false, errorValue
	}
	for _, calendar := range calendars {
		if strings.TrimSpace(calendar.CalendarID) == strings.TrimSpace(calendarID) && calendar.CanSelect {
			return calendar, true, nil
		}
	}
	return googleCalendarListEntry{}, false, nil
}

func googleCalendarListEntriesFromCalDAV(client *outboundCalDAVClient, infos []calDAVCalendarInfo, accountEmail string) ([]googleCalendarListEntry, error) {
	calendars := make([]googleCalendarListEntry, 0, len(infos))
	for _, info := range infos {
		calendarID := googleCalendarIDFromCalDAVPath(info.Path, accountEmail)
		if calendarID == "" {
			continue
		}
		calendarURL, errorValue := client.absoluteURL(info.Path)
		if errorValue != nil {
			return nil, errorValue
		}
		accessRole := googleCalendarAccessRoleFromCalDAV(info)
		canWrite := isWritableRemoteCalendarAccessRole(accessRole)
		disabledReason := googleCalendarSelectionDisabledReason(calendarID, canWrite, info.SupportedComponents)
		calendars = append(calendars, googleCalendarListEntry{
			CalendarID:              calendarID,
			Summary:                 googleCalendarSummaryFromCalDAV(info, calendarID),
			AccessRole:              accessRole,
			Primary:                 strings.EqualFold(calendarID, strings.TrimSpace(accountEmail)),
			BackgroundColor:         strings.TrimSpace(info.Color),
			CanWrite:                canWrite,
			CanSelect:               disabledReason == "",
			SelectionDisabledReason: disabledReason,
			CalendarURL:             calendarURL,
		})
	}
	return calendars, nil
}

func googleCalendarAccessRoleFromCalDAV(info calDAVCalendarInfo) string {
	if info.CanWrite {
		return "writer"
	}
	if info.CanRead {
		return "reader"
	}
	return "none"
}

func googleCalendarSummaryFromCalDAV(info calDAVCalendarInfo, calendarID string) string {
	if summary := strings.TrimSpace(info.Name); summary != "" {
		return summary
	}
	return strings.TrimSpace(calendarID)
}

func googleCalendarSelectionDisabledReason(calendarID string, canWrite bool, components []string) string {
	if !canWrite {
		return googleCalendarSelectionDisabledReasonWritePermissionRequired
	}
	if isUnsupportedGoogleCalendarSelectionTarget(calendarID, components) {
		return googleCalendarSelectionDisabledReasonUnsupportedCalendar
	}
	return ""
}

func isUnsupportedGoogleCalendarSelectionTarget(calendarID string, components []string) bool {
	calendarID = strings.ToLower(strings.TrimSpace(calendarID))
	return strings.Contains(calendarID, "#contacts@group.v.calendar.google.com") ||
		strings.Contains(calendarID, "#tasks") ||
		strings.Contains(calendarID, "tasks#") ||
		strings.Contains(calendarID, "tasks@group.v.calendar.google.com") ||
		!googleCalendarSupportsEventComponent(components)
}

func googleCalendarSupportsEventComponent(components []string) bool {
	if len(components) == 0 {
		return true
	}
	for _, component := range components {
		if strings.EqualFold(strings.TrimSpace(component), "VEVENT") {
			return true
		}
	}
	return false
}

func googleCalendarIDFromCalDAVPath(calendarPath string, accountEmail string) string {
	path := strings.TrimSpace(calendarPath)
	if parsed, errorValue := url.Parse(path); errorValue == nil && parsed.Path != "" {
		path = parsed.EscapedPath()
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	calendarSegment := parts[len(parts)-1]
	if strings.EqualFold(calendarSegment, "events") && len(parts) >= 2 {
		calendarSegment = parts[len(parts)-2]
	}
	decoded, errorValue := url.PathUnescape(calendarSegment)
	if errorValue != nil {
		return strings.TrimSpace(calendarSegment)
	}
	return strings.TrimSpace(decoded)
}

func isGoogleCalendarAuthorizationError(errorValue error) bool {
	var authorizationError googleCalendarAuthorizationError
	return errors.As(errorValue, &authorizationError)
}
