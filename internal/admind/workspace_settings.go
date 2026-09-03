package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type workspaceSettings struct {
	TimeZone string `json:"timeZone"`
	Language string `json:"language"`
}

const (
	workspaceLanguageKorean  = "ko"
	workspaceLanguageEnglish = "en"
)

// The time zone is read once per calendar event and the language once per
// message, so the company is asked at most this often and the last answer
// stands in between.
const workspaceSettingsFreshFor = 30 * time.Second

type heldWorkspaceSettings struct {
	mutex    sync.Mutex
	settings workspaceSettings
	readAt   time.Time
	isHeld   bool
}

func defaultWorkspaceSettings() workspaceSettings {
	return workspaceSettings{
		TimeZone: workspaceSystemTimeZone,
		Language: workspaceLanguageKorean,
	}
}

func (service *Service) readWorkspaceSettings() workspaceSettings {
	service.workspaceSettingsCache.mutex.Lock()
	defer service.workspaceSettingsCache.mutex.Unlock()
	if service.workspaceSettingsCache.isHeld && time.Since(service.workspaceSettingsCache.readAt) < workspaceSettingsFreshFor {
		return service.workspaceSettingsCache.settings
	}
	client := service.centralPlane()
	if client == nil {
		return defaultWorkspaceSettings()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	answered, errorValue := client.CompanySettings(ctx, service.claimedAdminEmail())
	if errorValue != nil {
		if service.workspaceSettingsCache.isHeld {
			return service.workspaceSettingsCache.settings
		}
		return defaultWorkspaceSettings()
	}
	service.workspaceSettingsCache.settings = workspaceSettingsOf(answered.TimeZone, answered.Locale)
	service.workspaceSettingsCache.readAt = time.Now()
	service.workspaceSettingsCache.isHeld = true
	return service.workspaceSettingsCache.settings
}

func (service *Service) forgetWorkspaceSettings() {
	service.workspaceSettingsCache.mutex.Lock()
	defer service.workspaceSettingsCache.mutex.Unlock()
	service.workspaceSettingsCache.isHeld = false
}

// The record holds a BCP 47 tag and the workspace speaks two languages, so a
// tag it does not speak reads as the default rather than as itself.
func workspaceSettingsOf(timeZone string, locale string) workspaceSettings {
	defaults := defaultWorkspaceSettings()
	settings := workspaceSettings{TimeZone: strings.TrimSpace(timeZone), Language: defaults.Language}
	if settings.TimeZone == "" {
		settings.TimeZone = defaults.TimeZone
	}
	language, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(locale)), "-")
	if language == workspaceLanguageKorean || language == workspaceLanguageEnglish {
		settings.Language = language
	}
	return settings
}

func (service *Service) workspaceLanguage() string {
	return service.readWorkspaceSettings().Language
}

func (service *Service) writeWorkspaceSettings(responseWriter http.ResponseWriter) {
	service.writeJSON(responseWriter, service.readWorkspaceSettings())
}

func (service *Service) updateWorkspaceSettings(responseWriter http.ResponseWriter, request *http.Request) {
	var payload workspaceSettings
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	change, errorValue := workspaceSettingsChange(payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	client := service.centralPlane()
	if client == nil {
		http.Error(responseWriter, errNoCompanyDirectory.Error(), http.StatusBadGateway)
		return
	}
	answered, errorValue := client.WriteCompanySettings(request.Context(), service.recordReaderEmail(request), change)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.forgetWorkspaceSettings()
	service.writeJSON(responseWriter, workspaceSettingsOf(answered.TimeZone, answered.Locale))
}

func workspaceSettingsChange(payload workspaceSettings) (map[string]any, error) {
	change := map[string]any{}
	if language := strings.ToLower(strings.TrimSpace(payload.Language)); language != "" {
		if errorValue := validateWorkspaceLanguage(language); errorValue != nil {
			return nil, errorValue
		}
		change["locale"] = language
	}
	if timeZone := strings.TrimSpace(payload.TimeZone); timeZone != "" && timeZone != workspaceSystemTimeZone {
		if _, errorValue := time.LoadLocation(timeZone); errorValue != nil {
			return nil, errorValue
		}
		change["timeZone"] = timeZone
	}
	if len(change) == 0 {
		return nil, fmt.Errorf("name a language or a time zone to change")
	}
	return change, nil
}

func validateWorkspaceLanguage(language string) error {
	if language == workspaceLanguageKorean || language == workspaceLanguageEnglish {
		return nil
	}
	return fmt.Errorf("language must be ko or en")
}
