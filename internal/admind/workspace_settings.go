package admind

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type workspaceSettings struct {
	TimeZone  string `json:"timeZone"`
	UpdatedAt string `json:"updatedAt,omitempty"`
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
	var payload workspaceSettings
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	settings, errorValue := normalizeWorkspaceSettings(payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if errorValue := service.writeWorkspaceSettingsFile(settings); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, settings)
}

func (service *Service) readWorkspaceSettings() (workspaceSettings, error) {
	document, errorValue := os.ReadFile(service.workspaceSettingsPath())
	if os.IsNotExist(errorValue) {
		return workspaceSettings{TimeZone: "system"}, nil
	}
	if errorValue != nil {
		return workspaceSettings{}, errorValue
	}
	var settings workspaceSettings
	if errorValue := json.Unmarshal(document, &settings); errorValue != nil {
		return workspaceSettings{}, errorValue
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
	return os.WriteFile(path, append(document, '\n'), 0o600)
}

func (service *Service) workspaceTimeLocation() (*time.Location, string) {
	settings, errorValue := service.readWorkspaceSettings()
	if errorValue == nil {
		timeZone := strings.TrimSpace(settings.TimeZone)
		if timeZone != "" && timeZone != "system" {
			location, loadError := time.LoadLocation(timeZone)
			if loadError == nil {
				return location, timeZone
			}
		}
	}
	return time.Local, systemTimeZoneName()
}

func (service *Service) workspaceSettingsPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "workspace-settings.json")
}

func normalizeWorkspaceSettings(settings workspaceSettings) (workspaceSettings, error) {
	normalized := normalizeWorkspaceSettingsWithoutValidation(settings)
	if normalized.TimeZone == "system" {
		return normalized, nil
	}
	if _, errorValue := time.LoadLocation(normalized.TimeZone); errorValue != nil {
		return workspaceSettings{}, errorValue
	}
	return normalized, nil
}

func normalizeWorkspaceSettingsWithoutValidation(settings workspaceSettings) workspaceSettings {
	timeZone := strings.TrimSpace(settings.TimeZone)
	if timeZone == "" {
		timeZone = "system"
	}
	return workspaceSettings{
		TimeZone:  timeZone,
		UpdatedAt: strings.TrimSpace(settings.UpdatedAt),
	}
}

func systemTimeZoneName() string {
	if timeZone := strings.TrimSpace(os.Getenv("TZ")); timeZone != "" {
		return timeZone
	}
	if time.Local != nil && strings.TrimSpace(time.Local.String()) != "" {
		return time.Local.String()
	}
	return "Local"
}
