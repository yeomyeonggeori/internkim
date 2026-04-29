package admind

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type botProfile struct {
	Username           string   `json:"username"`
	DisplayName        string   `json:"displayName"`
	EnglishDisplayName string   `json:"englishDisplayName,omitempty"`
	Aliases            []string `json:"aliases,omitempty"`
	PublicDescription  string   `json:"publicDescription"`
	IdentityExtension  string   `json:"identityExtension,omitempty"`
}

const legacyDefaultBotPublicDescriptionHash = "108227eaad94edd77ad7c16c676581f6d18882d2d507e2f33ff5a326dfc2ef02"

func defaultBotProfile() botProfile {
	return botProfile{
		Username:           "internkim",
		DisplayName:        "김인턴",
		EnglishDisplayName: "Intern Kim",
		Aliases:            []string{"인턴킴", "intern kim"},
		IdentityExtension:  "Use the current displayName naturally when introducing yourself.",
	}
}

func (service *Service) writeBotProfile(responseWriter http.ResponseWriter, request *http.Request) {
	profile, errorValue := service.loadOrSeedBotProfile(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, profile)
}

func (service *Service) updateBotProfile(responseWriter http.ResponseWriter, request *http.Request) {
	var profile botProfile
	if errorValue := json.NewDecoder(request.Body).Decode(&profile); errorValue != nil {
		http.Error(responseWriter, "invalid request body", http.StatusBadRequest)
		return
	}
	profile = normalizeBotProfile(profile)
	if errorValue := validateBotProfile(profile); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.saveBotProfile(profile); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue := service.writeWorkspaceBotProfile(profile); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue := service.syncMattermostBotProfile(request.Context(), profile); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, profile)
}

func (service *Service) loadOrSeedBotProfile(ctx context.Context) (botProfile, error) {
	profile, found := service.loadBotProfile()
	if !found {
		profile = service.seedBotProfileFromMattermost(ctx)
		if errorValue := service.saveBotProfile(profile); errorValue != nil {
			return botProfile{}, errorValue
		}
	}
	if errorValue := service.writeWorkspaceBotProfile(profile); errorValue != nil {
		return botProfile{}, errorValue
	}
	return profile, nil
}

func (service *Service) loadBotProfile() (botProfile, bool) {
	document, errorValue := os.ReadFile(service.Configuration.BotProfilePath)
	if errorValue != nil {
		return botProfile{}, false
	}
	var profile botProfile
	if errorValue := json.Unmarshal(document, &profile); errorValue != nil {
		return botProfile{}, false
	}
	return normalizeBotProfile(profile), true
}

func (service *Service) seedBotProfileFromMattermost(ctx context.Context) botProfile {
	profile := defaultBotProfile()
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return profile
	}
	userRecord, found, errorValue := service.findMattermostUserByUsername(ctx, adminToken, "internkim")
	if errorValue != nil || !found {
		return profile
	}
	if strings.TrimSpace(userRecord.DisplayName) != "" {
		profile.DisplayName = strings.TrimSpace(userRecord.DisplayName)
	}
	if strings.TrimSpace(userRecord.FirstName) != "" {
		profile.DisplayName = strings.TrimSpace(userRecord.FirstName)
	}
	if strings.TrimSpace(userRecord.Nickname) != "" {
		profile.EnglishDisplayName = strings.TrimSpace(userRecord.Nickname)
	}
	if strings.TrimSpace(userRecord.Position) != "" {
		profile.PublicDescription = strings.TrimSpace(userRecord.Position)
	}
	if isLegacyDefaultBotPublicDescription(profile.PublicDescription) {
		profile.PublicDescription = ""
	}
	return profile
}

func (service *Service) saveBotProfile(profile botProfile) error {
	document, errorValue := json.MarshalIndent(normalizeBotProfile(profile), "", "  ")
	if errorValue != nil {
		return errorValue
	}
	if errorValue := os.MkdirAll(filepath.Dir(service.Configuration.BotProfilePath), 0o700); errorValue != nil {
		return errorValue
	}
	temporaryPath := service.Configuration.BotProfilePath + ".tmp"
	if errorValue := os.WriteFile(temporaryPath, document, 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporaryPath, service.Configuration.BotProfilePath)
}

func (service *Service) writeWorkspaceBotProfile(profile botProfile) error {
	document := renderBotProfileMarkdown(normalizeBotProfile(profile))
	path := filepath.Join(service.Configuration.BlueclawWorkspacePath, "BOT_PROFILE.md")
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o750); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(path, []byte(document), 0o644)
}

func (service *Service) syncMattermostBotProfile(ctx context.Context, profile botProfile) error {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	botRecord, found, errorValue := service.findMattermostUserByUsername(ctx, adminToken, "internkim")
	if errorValue != nil {
		return errorValue
	}
	if !found || botRecord.ID == "" {
		return fmt.Errorf("Mattermost bot internkim is missing")
	}
	body := map[string]string{
		"first_name": profile.DisplayName,
		"nickname":   profile.EnglishDisplayName,
		"position":   profile.PublicDescription,
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(botRecord.ID)+"/patch", adminToken, body, nil); errorValue != nil {
		return errorValue
	}
	return nil
}

func normalizeBotProfile(profile botProfile) botProfile {
	defaultProfile := defaultBotProfile()
	profile.Username = "internkim"
	profile.DisplayName = strings.TrimSpace(firstNonEmpty(profile.DisplayName, defaultProfile.DisplayName))
	profile.EnglishDisplayName = strings.TrimSpace(firstNonEmpty(profile.EnglishDisplayName, defaultProfile.EnglishDisplayName))
	profile.PublicDescription = strings.TrimSpace(profile.PublicDescription)
	if isLegacyDefaultBotPublicDescription(profile.PublicDescription) {
		profile.PublicDescription = ""
	}
	profile.IdentityExtension = strings.TrimSpace(profile.IdentityExtension)
	if profile.IdentityExtension == "" {
		profile.IdentityExtension = defaultProfile.IdentityExtension
	}
	aliases := []string{}
	seenAlias := map[string]bool{}
	for _, alias := range append(profile.Aliases, defaultProfile.Aliases...) {
		trimmedAlias := strings.TrimSpace(alias)
		if trimmedAlias == "" || seenAlias[strings.ToLower(trimmedAlias)] {
			continue
		}
		seenAlias[strings.ToLower(trimmedAlias)] = true
		aliases = append(aliases, trimmedAlias)
	}
	profile.Aliases = aliases
	return profile
}

func validateBotProfile(profile botProfile) error {
	if strings.TrimSpace(profile.DisplayName) == "" {
		return fmt.Errorf("display name is required")
	}
	if len([]rune(profile.DisplayName)) > 64 {
		return fmt.Errorf("display name is too long")
	}
	if len([]rune(profile.PublicDescription)) > 256 {
		return fmt.Errorf("public description is too long")
	}
	if len([]rune(profile.IdentityExtension)) > 2000 {
		return fmt.Errorf("identity extension is too long")
	}
	for _, value := range []string{profile.DisplayName, profile.EnglishDisplayName, profile.PublicDescription, profile.IdentityExtension, strings.Join(profile.Aliases, "\n")} {
		if containsSecretLikeText(value) {
			return fmt.Errorf("bot profile must not contain secrets")
		}
	}
	return nil
}

func isLegacyDefaultBotPublicDescription(value string) bool {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return fmt.Sprintf("%x", sum) == legacyDefaultBotPublicDescriptionHash
}

func containsSecretLikeText(value string) bool {
	lowerValue := strings.ToLower(value)
	for _, marker := range []string{"api_key", "apikey", "token=", "secret=", "bearer ", "sk-", "xoxb-", "xapp-"} {
		if strings.Contains(lowerValue, marker) {
			return true
		}
	}
	return false
}

func renderBotProfileMarkdown(profile botProfile) string {
	lines := []string{
		"# BOT_PROFILE.md",
		"",
		"username: " + profile.Username,
		"displayName: " + profile.DisplayName,
		"englishDisplayName: " + profile.EnglishDisplayName,
		"aliases:",
	}
	for _, alias := range profile.Aliases {
		lines = append(lines, "  - "+alias)
	}
	lines = append(lines,
		"publicDescription: "+profile.PublicDescription,
		"",
		"## identityExtension",
		"",
		profile.IdentityExtension,
		"",
	)
	return strings.Join(lines, "\n")
}
