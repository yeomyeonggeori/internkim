package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

type workspaceSettings struct {
	CountryCode string `json:"countryCode"`
	TimeZone    string `json:"timeZone"`
	Language    string `json:"language"`
	CallingCode string `json:"callingCode"`
	UpdatedAt   string `json:"updatedAt,omitempty"`
}

const (
	workspaceLanguageKorean     = "ko"
	workspaceLanguageEnglish    = "en"
	workspaceDefaultCountryCode = "KR"
	workspaceDefaultCallingCode = "82"
)

func defaultWorkspaceSettings() workspaceSettings {
	return workspaceSettings{
		CountryCode: workspaceDefaultCountryCode,
		TimeZone:    workspaceBusinessTimeZone,
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

func (service *Service) workspaceCountryCode() string {
	settings, errorValue := service.readWorkspaceSettings()
	if errorValue != nil {
		return workspaceDefaultCountryCode
	}
	return normalizeWorkspaceSettingsWithoutValidation(settings).CountryCode
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
	settings, errorValue := normalizeWorkspaceSettings(payload)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(payload.CountryCode) != "" {
		countries, countriesError := service.ensureCalendarHolidayCountries(request.Context(), time.Now().UTC())
		if countriesError != nil {
			http.Error(responseWriter, countriesError.Error(), http.StatusBadGateway)
			return
		}
		if !calendarHolidayCountryIsSupported(countries, settings.CountryCode) {
			http.Error(responseWriter, "countryCode is not supported by the holiday provider", http.StatusBadRequest)
			return
		}
	}
	currentTime := time.Now().UTC()
	settings.UpdatedAt = currentTime.Format(time.RFC3339)
	if errorValue := service.writeWorkspaceSettingsFile(settings); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	countryChanged := previousSettings.CountryCode != settings.CountryCode
	languageChanged := previousSettings.Language != settings.Language
	if countryChanged {
		if errorValue := service.refreshCalendarHolidayCache(request.Context(), currentTime); errorValue != nil {
			slog.WarnContext(request.Context(), "calendar holiday refresh after country change failed",
				"country_code", settings.CountryCode,
				"error", errorValue,
			)
		}
	}
	if !countryChanged || languageChanged {
		if errorValue := service.syncMattermostWorkspaceChannelDisplayNames(request.Context(), settings.Language); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
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
	countryCode := strings.ToUpper(strings.TrimSpace(settings.CountryCode))
	if countryCode != "" {
		if errorValue := validateWorkspaceCountryCode(countryCode); errorValue != nil {
			return workspaceSettings{}, errorValue
		}
	}
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
	countryCode := strings.ToUpper(strings.TrimSpace(settings.CountryCode))
	if validateWorkspaceCountryCode(countryCode) != nil {
		countryCode = defaults.CountryCode
	}
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
		CountryCode: countryCode,
		TimeZone:    timeZone,
		Language:    language,
		CallingCode: callingCode,
		UpdatedAt:   strings.TrimSpace(settings.UpdatedAt),
	}
}

func validateWorkspaceCountryCode(countryCode string) error {
	if len(countryCode) != 2 {
		return fmt.Errorf("countryCode must be an ISO 3166-1 alpha-2 code")
	}
	for _, character := range countryCode {
		if character < 'A' || character > 'Z' {
			return fmt.Errorf("countryCode must be an ISO 3166-1 alpha-2 code")
		}
	}
	return nil
}

func validateWorkspaceLanguage(language string) error {
	if language == workspaceLanguageKorean || language == workspaceLanguageEnglish {
		return nil
	}
	return fmt.Errorf("language must be ko or en")
}

func (service *Service) syncMattermostWorkspaceChannelDisplayNames(ctx context.Context, language string) error {
	if errorValue := validateWorkspaceLanguage(language); errorValue != nil {
		return errorValue
	}
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	for _, channel := range mattermostdefaults.PublicChannelsForLanguage(language) {
		if errorValue := service.ensureMattermostLocalizedPublicChannel(ctx, token, teamRecord.ID, channel); errorValue != nil {
			return errorValue
		}
	}
	return service.ensureMattermostPublicChannelDisplayName(ctx, token, teamRecord.ID, calendarAnnouncementsChannelName, announcementsChannelDisplayName(language))
}

func (service *Service) ensureMattermostLocalizedPublicChannel(ctx context.Context, token string, teamID string, channel mattermostdefaults.PublicChannel) error {
	channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamID, channel.Name, channel.DisplayName)
	if errorValue != nil {
		return errorValue
	}
	return service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel)
}

func (service *Service) ensureMattermostPublicChannelDisplayName(ctx context.Context, token string, teamID string, channelName string, displayName string) error {
	channelID, errorValue := service.mattermostChannelIDByName(ctx, token, teamID, channelName)
	if errorValue == nil && channelID != "" {
		return service.updateMattermostChannelDisplayName(ctx, token, channelID, displayName)
	}
	if errorValue != nil && !isMattermostNotFound(errorValue) {
		return errorValue
	}
	body := map[string]string{
		"team_id":      teamID,
		"name":         channelName,
		"display_name": displayName,
		"type":         "O",
	}
	var channelRecord mattermostChannelRecord
	return service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels", token, body, &channelRecord)
}

func (service *Service) updateMattermostChannelDisplayName(ctx context.Context, token string, channelID string, displayName string) error {
	body := map[string]string{"display_name": displayName}
	if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", token, body, nil); errorValue != nil {
		return errorValue
	}
	return service.cleanupMattermostManagedChannelSystemPosts(ctx, token, channelID)
}

func flowChannelDisplayName(language string) string {
	channel, _ := mattermostdefaults.PublicChannelForLanguage(mattermostFlowChannelName, language)
	return channel.DisplayName
}

func announcementsChannelDisplayName(language string) string {
	if strings.EqualFold(strings.TrimSpace(language), workspaceLanguageEnglish) {
		return "Announcements"
	}
	return "공지사항"
}
