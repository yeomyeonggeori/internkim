package admind

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type workspaceSettings struct {
	TimeZone    string `json:"timeZone"`
	Language    string `json:"language"`
	CallingCode string `json:"callingCode"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

const (
	workspaceLanguageKorean     = "ko"
	workspaceLanguageEnglish    = "en"
	workspaceDefaultCallingCode = "82"
)

func defaultWorkspaceSettings() workspaceSettings {
	return workspaceSettings{
		TimeZone:    workspaceSystemTimeZone,
		Language:    workspaceLanguageKorean,
		CallingCode: workspaceDefaultCallingCode,
	}
}

func (service *Service) workspaceCallingCode() string {
	settings, errorValue := service.readWorkspaceSettings()
	if errorValue != nil {
		return workspaceDefaultCallingCode
	}
	return normalizeWorkspaceSettingsWithoutValidation(settings).CallingCode
}

func (service *Service) writeWorkspaceSettings(responseWriter http.ResponseWriter) {
	settings, errorValue := service.readWorkspaceSettings()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, settings)
}

func (service *Service) updateWorkspaceSettings(responseWriter http.ResponseWriter, request *http.Request) {
	previousSettings, errorValue := service.readWorkspaceSettings()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	var payload workspaceSettings
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if service.centralPlane() != nil && strings.TrimSpace(payload.TimeZone) != previousSettings.TimeZone {
		http.Error(responseWriter, "the company record keeps the time zone", http.StatusConflict)
		return
	}
	settings, errorValue := normalizeWorkspaceSettings(payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	currentTime := time.Now().UTC()
	settings.UpdatedAt = currentTime.Format(time.RFC3339)
	if errorValue := service.writeWorkspaceSettingsFile(settings); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, settings)
}

func (service *Service) readWorkspaceSettings() (workspaceSettings, error) {
	document, errorValue := os.ReadFile(service.workspaceSettingsPath())
	if os.IsNotExist(errorValue) {
		return defaultWorkspaceSettings(), nil
	}
	if errorValue != nil {
		return workspaceSettings{}, errorValue
	}
	var settings workspaceSettings
	if errorValue := json.Unmarshal(document, &settings); errorValue != nil {
		return defaultWorkspaceSettings(), nil
	}
	return normalizeWorkspaceSettingsWithoutValidation(settings), nil
}

func (service *Service) writeWorkspaceSettingsFile(settings workspaceSettings) error {
	document, errorValue := json.MarshalIndent(settings, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	path := service.workspaceSettingsPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return errorValue
	}
	temporaryPath := path + ".tmp"
	if errorValue := os.WriteFile(temporaryPath, append(document, '\n'), 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporaryPath, path)
}

func (service *Service) workspaceSettingsPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "workspace-settings.json")
}

func (service *Service) workspaceLanguage() string {
	settings, errorValue := service.readWorkspaceSettings()
	if errorValue != nil {
		return workspaceLanguageKorean
	}
	return settings.Language
}

func normalizeWorkspaceSettings(settings workspaceSettings) (workspaceSettings, error) {
	normalized := normalizeWorkspaceSettingsWithoutValidation(settings)
	language := strings.ToLower(strings.TrimSpace(settings.Language))
	if language != "" {
		if errorValue := validateWorkspaceLanguage(language); errorValue != nil {
			return workspaceSettings{}, errorValue
		}
	}
	if normalized.TimeZone == workspaceSystemTimeZone {
		return normalized, nil
	}
	if _, errorValue := time.LoadLocation(normalized.TimeZone); errorValue != nil {
		return workspaceSettings{}, errorValue
	}
	return normalized, nil
}

func normalizeWorkspaceSettingsWithoutValidation(settings workspaceSettings) workspaceSettings {
	defaults := defaultWorkspaceSettings()
	timeZone := strings.TrimSpace(settings.TimeZone)
	if timeZone == "" {
		timeZone = defaults.TimeZone
	}
	language := strings.ToLower(strings.TrimSpace(settings.Language))
	if language != workspaceLanguageKorean && language != workspaceLanguageEnglish {
		language = defaults.Language
	}
	callingCode := normalizeCallingCode(settings.CallingCode)
	if callingCode == "" {
		callingCode = defaults.CallingCode
	}
	return workspaceSettings{
		TimeZone:    timeZone,
		Language:    language,
		CallingCode: callingCode,
		UpdatedAt:   strings.TrimSpace(settings.UpdatedAt),
	}
}

func validateWorkspaceLanguage(language string) error {
	if language == workspaceLanguageKorean || language == workspaceLanguageEnglish {
		return nil
	}
	return fmt.Errorf("language must be ko or en")
}
