package admind

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type calendarAccountStatusResponse struct {
	Connected       bool   `json:"connected"`
	Provider        string `json:"provider,omitempty"`
	AccountEmail    string `json:"accountEmail,omitempty"`
	DefaultCalendar string `json:"defaultCalendarURL,omitempty"`
	LastAuthError   string `json:"lastAuthError,omitempty"`
	LastAuthErrorAt string `json:"lastAuthErrorAt,omitempty"`
	NeedsReauth     bool   `json:"needsReauth"`
}

func (service *Service) serveCalendarAccountStatus(writer http.ResponseWriter, request *http.Request) {
	account, found, errorValue := service.readRemoteCalendarAccountByProvider(request.Context(), remoteCalendarProviderGoogle)
	if errorValue != nil {
		http.Error(writer, "failed to read account status", http.StatusInternalServerError)
		log.Printf("account status read: %v", errorValue)
		return
	}
	response := calendarAccountStatusResponse{Connected: found}
	if found {
		response.Provider = account.Provider
		response.AccountEmail = account.AccountEmail
		response.DefaultCalendar = account.DefaultCalendarURL
		response.LastAuthError = account.LastAuthError
		response.LastAuthErrorAt = account.LastAuthErrorAt
		response.NeedsReauth = strings.TrimSpace(account.LastAuthError) != ""
	}
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	if errorValue := json.NewEncoder(writer).Encode(response); errorValue != nil {
		log.Printf("account status encode: %v", errorValue)
	}
}
