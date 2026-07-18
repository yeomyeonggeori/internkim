package admind

import (
	"context"
	"encoding/json"
	"net/http"
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
	if !service.canManageGoogleOAuth(request) {
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
	calendar, found, errorValue := service.findSelectableGoogleCalendar(request.Context(), account, calendarID)
	if errorValue != nil {
		if isGoogleCalendarAuthorizationError(errorValue) || isCalendarAuthError(errorValue) {
			service.markRemoteCalendarAccountAuthError(request.Context(), account, errorValue)
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if !found {
		service.markSelectedCalendarInaccessibleIfCurrent(request.Context(), account, calendarID)
		http.Error(responseWriter, "selectable google calendar not found", http.StatusBadRequest)
		return
	}
	if errorValue := service.verifyGoogleCalendarSelectionReady(request.Context(), account, calendar); errorValue != nil {
		if isGoogleCalendarAuthorizationError(errorValue) {
			service.markRemoteCalendarAccountAuthError(request.Context(), account, errorValue)
		}
		if readinessStatus, ok := googleCalendarReadinessStatusFromError(errorValue); ok {
			service.markSelectedCalendarReadinessStatusIfCurrent(request.Context(), account, calendarID, readinessStatus)
		}
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	updated, errorValue := service.saveSelectedCalendar(request.Context(), account, calendar.CalendarID, calendar.Summary, calendar.AccessRole, calendar.CalendarURL, time.Now().UTC())
	if errorValue != nil {
		http.Error(responseWriter, "failed to save selected google calendar", http.StatusInternalServerError)
		return
	}
	service.clearRemoteCalendarAccountAuthError(request.Context(), account)
	service.writeJSON(responseWriter, selectGoogleCalendarResponse{
		AccountEmail:     strings.TrimSpace(updated.AccountEmail),
		SelectedCalendar: calendar,
		CalendarURL:      strings.TrimSpace(updated.SelectedCalendarURL),
	})
}

func (service *Service) markSelectedCalendarInaccessibleIfCurrent(ctx context.Context, account remoteCalendarAccount, calendarID string) {
	service.markSelectedCalendarReadinessStatusIfCurrent(ctx, account, calendarID, calendarReadinessStatusCalendarInaccessible)
}

func (service *Service) markSelectedCalendarReadinessStatusIfCurrent(ctx context.Context, account remoteCalendarAccount, calendarID string, readinessStatus string) {
	if strings.TrimSpace(account.SelectedCalendarID) != strings.TrimSpace(calendarID) {
		return
	}
	if strings.TrimSpace(readinessStatus) == "" {
		return
	}
	service.updateSelectedCalendarReadinessStatus(ctx, account, readinessStatus)
}
