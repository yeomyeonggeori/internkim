package admind

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type adminLocaleResponse struct {
	Locale string `json:"locale"`
}

type adminLocaleUpdateRequest struct {
	Locale string `json:"locale"`
}

func (service *Service) writeAdminLocale(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, adminLocaleResponse{Locale: service.adminLocale()})
}

func (service *Service) updateAdminLocale(responseWriter http.ResponseWriter, request *http.Request) {
	var payload adminLocaleUpdateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	locale := normalizeAdminLocale(payload.Locale)
	if errorValue := os.MkdirAll(service.Configuration.StateDirectory, 0o700); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue := os.WriteFile(service.adminLocalePath(), []byte(locale), 0o600); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, adminLocaleResponse{Locale: locale})
}

func (service *Service) adminLocale() string {
	return normalizeAdminLocale(readTrimmedFile(service.adminLocalePath()))
}

func (service *Service) adminLocalePath() string {
	return filepath.Join(service.Configuration.StateDirectory, "admin-locale")
}

func normalizeAdminLocale(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "en") {
		return "en"
	}
	return "ko"
}
