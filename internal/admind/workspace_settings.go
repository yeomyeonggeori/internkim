package admind

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// What the company works in: the language every screen speaks and the time
// zone every date is read in. Both are the plane's company row, so this
// carries what the record answers rather than a copy the device keeps.
type workspaceSettings struct {
	TimeZone string `json:"timeZone"`
	Language string `json:"language"`
}

const (
	workspaceLanguageKorean  = "ko"
	workspaceLanguageEnglish = "en"
)

// The record holds a BCP 47 tag and the workspace speaks two languages, so a
// tag it does not speak reads as the default rather than as itself.
func workspaceLanguageOf(locale string) string {
	language, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(locale)), "-")
	if language == workspaceLanguageKorean || language == workspaceLanguageEnglish {
		return language
	}
	return workspaceLanguageKorean
}

func workspaceSettingsOf(settings companySettings) workspaceSettings {
	return workspaceSettings{TimeZone: settings.timeZone, Language: settings.language}
}

func (service *Service) writeWorkspaceSettings(responseWriter http.ResponseWriter, request *http.Request) {
	settings, _ := service.readCompanySettings(request.Context())
	service.writeJSON(responseWriter, workspaceSettingsOf(settings))
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
	written := service.holdCompanySettings(companySettings{
		timeZone: loadableTimeZoneName(answered.TimeZone),
		language: workspaceLanguageOf(answered.Locale),
	})
	service.writeJSON(responseWriter, workspaceSettingsOf(written))
}

func workspaceSettingsChange(payload workspaceSettings) (map[string]any, error) {
	change := map[string]any{}
	if language := strings.ToLower(strings.TrimSpace(payload.Language)); language != "" {
		if errorValue := validateWorkspaceLanguage(language); errorValue != nil {
			return nil, errorValue
		}
		change["locale"] = language
	}
	if timeZone := strings.TrimSpace(payload.TimeZone); timeZone != "" {
		if _, errorValue := time.LoadLocation(timeZone); errorValue != nil {
			return nil, fmt.Errorf("%q is not a time zone name this device knows; name an IANA zone such as Asia/Seoul", timeZone)
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
