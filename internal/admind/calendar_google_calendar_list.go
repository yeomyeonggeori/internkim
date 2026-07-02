package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/oauth2"
)

const (
	googleCalendarListEndpoint         = "https://www.googleapis.com/calendar/v3/users/me/calendarList"
	googleCalendarListMaxResults       = "250"
	googleCalendarListResponseMaxBytes = 1 << 20
)

type googleCalendarListResponse struct {
	AccountEmail string                    `json:"accountEmail"`
	Calendars    []googleCalendarListEntry `json:"calendars"`
}

type googleCalendarListEntry struct {
	CalendarID      string `json:"calendarID"`
	Summary         string `json:"summary"`
	AccessRole      string `json:"accessRole"`
	Primary         bool   `json:"primary"`
	BackgroundColor string `json:"backgroundColor,omitempty"`
}

type googleCalendarListDocument struct {
	Items         []googleCalendarListDocumentItem `json:"items"`
	NextPageToken string                           `json:"nextPageToken"`
}

type googleCalendarListDocumentItem struct {
	ID              string `json:"id"`
	Summary         string `json:"summary"`
	AccessRole      string `json:"accessRole"`
	Primary         bool   `json:"primary"`
	BackgroundColor string `json:"backgroundColor"`
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
	if !service.isAuthorized(request) {
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
	calendars, errorValue := service.listWritableGoogleCalendars(request.Context(), account)
	if errorValue != nil {
		if isGoogleCalendarAuthorizationError(errorValue) {
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

func (service *Service) listWritableGoogleCalendars(ctx context.Context, account remoteCalendarAccount) ([]googleCalendarListEntry, error) {
	tokenSource, errorValue := service.googleOAuthTokenSource(ctx, account, "")
	if errorValue != nil {
		return nil, errorValue
	}
	document, errorValue := service.fetchGoogleCalendarList(ctx, tokenSource)
	if errorValue != nil {
		return nil, errorValue
	}
	return writableGoogleCalendarListEntries(document.Items), nil
}

func (service *Service) findWritableGoogleCalendar(ctx context.Context, account remoteCalendarAccount, calendarID string) (googleCalendarListEntry, bool, error) {
	calendars, errorValue := service.listWritableGoogleCalendars(ctx, account)
	if errorValue != nil {
		return googleCalendarListEntry{}, false, errorValue
	}
	for _, calendar := range calendars {
		if strings.TrimSpace(calendar.CalendarID) == strings.TrimSpace(calendarID) {
			return calendar, true, nil
		}
	}
	return googleCalendarListEntry{}, false, nil
}

func (service *Service) fetchGoogleCalendarList(ctx context.Context, tokenSource oauth2.TokenSource) (googleCalendarListDocument, error) {
	token, errorValue := tokenSource.Token()
	if errorValue != nil {
		return googleCalendarListDocument{}, googleCalendarAuthorizationError{Cause: errorValue}
	}
	var result googleCalendarListDocument
	pageToken := ""
	for {
		document, errorValue := service.fetchGoogleCalendarListPage(ctx, token, pageToken)
		if errorValue != nil {
			return googleCalendarListDocument{}, errorValue
		}
		result.Items = append(result.Items, document.Items...)
		pageToken = strings.TrimSpace(document.NextPageToken)
		if pageToken == "" {
			return result, nil
		}
	}
}

func (service *Service) fetchGoogleCalendarListPage(ctx context.Context, token *oauth2.Token, pageToken string) (googleCalendarListDocument, error) {
	requestURL, errorValue := googleCalendarListRequestURL(pageToken)
	if errorValue != nil {
		return googleCalendarListDocument{}, errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if errorValue != nil {
		return googleCalendarListDocument{}, errorValue
	}
	token.SetAuthHeader(request)
	response, errorValue := service.googleOAuthHTTPClient().Do(request)
	if errorValue != nil {
		return googleCalendarListDocument{}, errorValue
	}
	defer response.Body.Close()
	body, errorValue := io.ReadAll(io.LimitReader(response.Body, googleCalendarListResponseMaxBytes))
	if errorValue != nil {
		return googleCalendarListDocument{}, errorValue
	}
	if response.StatusCode != http.StatusOK {
		statusError := fmt.Errorf("google calendar list status %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return googleCalendarListDocument{}, googleCalendarAuthorizationError{Cause: statusError}
		}
		return googleCalendarListDocument{}, statusError
	}
	var document googleCalendarListDocument
	if errorValue := json.Unmarshal(body, &document); errorValue != nil {
		return googleCalendarListDocument{}, fmt.Errorf("parse google calendar list: %w", errorValue)
	}
	return document, nil
}

func googleCalendarListRequestURL(pageToken string) (string, error) {
	requestURL, errorValue := url.Parse(googleCalendarListEndpoint)
	if errorValue != nil {
		return "", errorValue
	}
	query := requestURL.Query()
	query.Set("maxResults", googleCalendarListMaxResults)
	query.Set("fields", "items(id,summary,accessRole,primary,backgroundColor),nextPageToken")
	if strings.TrimSpace(pageToken) != "" {
		query.Set("pageToken", strings.TrimSpace(pageToken))
	}
	requestURL.RawQuery = query.Encode()
	return requestURL.String(), nil
}

func writableGoogleCalendarListEntries(items []googleCalendarListDocumentItem) []googleCalendarListEntry {
	calendars := make([]googleCalendarListEntry, 0, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ID) == "" || !isWritableRemoteCalendarAccessRole(item.AccessRole) {
			continue
		}
		calendars = append(calendars, googleCalendarListEntry{
			CalendarID:      strings.TrimSpace(item.ID),
			Summary:         strings.TrimSpace(item.Summary),
			AccessRole:      strings.TrimSpace(item.AccessRole),
			Primary:         item.Primary,
			BackgroundColor: strings.TrimSpace(item.BackgroundColor),
		})
	}
	return calendars
}

func isGoogleCalendarAuthorizationError(errorValue error) bool {
	var authorizationError googleCalendarAuthorizationError
	return errors.As(errorValue, &authorizationError)
}
