package admind

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type calendarAccountStatusResponse struct {
	WorkspaceCalendarReady     bool   `json:"workspaceCalendarReady"`
	Connected                  bool   `json:"connected"`
	Provider                   string `json:"provider,omitempty"`
	AccountEmail               string `json:"accountEmail,omitempty"`
	DefaultCalendar            string `json:"defaultCalendarURL,omitempty"`
	SelectedCalendarID         string `json:"selectedCalendarID,omitempty"`
	SelectedCalendarName       string `json:"selectedCalendarName,omitempty"`
	SelectedCalendarAccessRole string `json:"selectedCalendarAccessRole,omitempty"`
	LastAuthError              string `json:"lastAuthError,omitempty"`
	LastAuthErrorAt            string `json:"lastAuthErrorAt,omitempty"`
	NeedsReauth                bool   `json:"needsReauth"`
	NeedsCalendarSelection     bool   `json:"needsCalendarSelection"`
	InitialSyncCompleted       bool   `json:"initialSyncCompleted"`
	CalendarSyncReady          bool   `json:"calendarSyncReady"`
	GoogleOAuthConfigured      bool   `json:"googleOAuthConfigured"`
	CanManageGoogleOAuth       bool   `json:"canManageGoogleOAuth"`
}

func (service *Service) serveCalendarAccountStatus(writer http.ResponseWriter, request *http.Request) {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(request.Context(), remoteCalendarProviderGoogle)
	if errorValue != nil {
		http.Error(writer, "failed to read account status", http.StatusInternalServerError)
		log.Printf("account status read: %v", errorValue)
		return
	}
	response := calendarAccountStatusResponse{
		WorkspaceCalendarReady: true,
		Connected:              found,
		GoogleOAuthConfigured:  service.isGoogleOAuthConfigured(),
		CanManageGoogleOAuth:   service.isAuthorized(request),
	}
	if found {
		response.Provider = account.Provider
		response.AccountEmail = account.AccountEmail
		response.DefaultCalendar = account.DefaultCalendarURL
		response.SelectedCalendarID = account.SelectedCalendarID
		response.SelectedCalendarName = account.SelectedCalendarSummary
		response.SelectedCalendarAccessRole = account.SelectedCalendarAccessRole
		response.LastAuthError = account.LastAuthError
		response.LastAuthErrorAt = account.LastAuthErrorAt
		response.NeedsReauth = strings.TrimSpace(account.LastAuthError) != ""
		response.NeedsCalendarSelection = strings.TrimSpace(account.SelectedCalendarID) == "" || strings.TrimSpace(account.SelectedCalendarURL) == ""
		response.InitialSyncCompleted = strings.TrimSpace(account.InitialSyncCompletedAt) != ""
		response.CalendarSyncReady = !response.NeedsReauth && !response.NeedsCalendarSelection && response.InitialSyncCompleted && remoteCalendarAccountCanWrite(account)
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if errorValue := json.NewEncoder(writer).Encode(response); errorValue != nil {
		log.Printf("account status encode: %v", errorValue)
	}
}
