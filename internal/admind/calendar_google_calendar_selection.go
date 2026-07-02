package admind

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type selectGoogleCalendarRequest struct {
	CalendarID string `json:"calendarID"`
}

type selectGoogleCalendarResponse struct {
	AccountEmail     string                  `json:"accountEmail"`
	SelectedCalendar googleCalendarListEntry `json:"selectedCalendar"`
	CalendarURL      string                  `json:"calendarURL"`
}

func (service *Service) selectGoogleCalendar(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.isAuthorized(request) {
		http.Error(responseWriter, "admin access required", http.StatusForbidden)
		return
	}
	input, ok := readSelectGoogleCalendarRequest(responseWriter, request)
	if !ok {
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
	service.saveGoogleCalendarSelection(responseWriter, request, account, input.CalendarID)
}

func readSelectGoogleCalendarRequest(responseWriter http.ResponseWriter, request *http.Request) (selectGoogleCalendarRequest, bool) {
	var input selectGoogleCalendarRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&input); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return selectGoogleCalendarRequest{}, false
	}
	input.CalendarID = strings.TrimSpace(input.CalendarID)
	if input.CalendarID == "" {
		http.Error(responseWriter, "calendarID is required", http.StatusBadRequest)
		return selectGoogleCalendarRequest{}, false
	}
	return input, true
}

func (service *Service) saveGoogleCalendarSelection(responseWriter http.ResponseWriter, request *http.Request, account remoteCalendarAccount, calendarID string) {
	calendar, found, errorValue := service.findWritableGoogleCalendar(request.Context(), account, calendarID)
	if errorValue != nil {
		if isGoogleCalendarAuthorizationError(errorValue) {
			service.markRemoteCalendarAccountAuthError(request.Context(), account, errorValue)
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if !found {
		http.Error(responseWriter, "writable google calendar not found", http.StatusBadRequest)
		return
	}
	if errorValue := service.verifyGoogleCalendarSelectionReady(request.Context(), account, calendar); errorValue != nil {
		if isGoogleCalendarAuthorizationError(errorValue) {
			service.markRemoteCalendarAccountAuthError(request.Context(), account, errorValue)
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	calendarURL := googleCalDAVCalendarEventsURL(calendar.CalendarID, account.AccountEmail)
	updated, errorValue := service.saveSelectedCalendar(request.Context(), account, calendar.CalendarID, calendar.Summary, calendar.AccessRole, calendarURL, time.Now().UTC())
	if errorValue != nil {
		http.Error(responseWriter, "failed to save selected google calendar", http.StatusInternalServerError)
		return
	}
	service.clearRemoteCalendarAccountAuthError(request.Context(), updated)
	service.writeJSON(responseWriter, selectGoogleCalendarResponse{
		AccountEmail:     strings.TrimSpace(updated.AccountEmail),
		SelectedCalendar: calendar,
		CalendarURL:      strings.TrimSpace(updated.SelectedCalendarURL),
	})
}

func googleCalDAVCalendarEventsURL(calendarID string, accountEmail string) string {
	normalizedCalendarID := strings.TrimSpace(calendarID)
	if strings.EqualFold(normalizedCalendarID, "primary") {
		normalizedCalendarID = strings.TrimSpace(accountEmail)
	}
	return googleCalDAVBaseURL + "/" + url.PathEscape(normalizedCalendarID) + "/events/"
}
