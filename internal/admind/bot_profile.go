package admind

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/identity"
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

func (service *Service) startBotProfileSync(ctx context.Context) {
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		profile, errorValue := service.loadOrSeedBotProfile(ctx)
		if errorValue != nil {
			log.Printf("bot profile startup sync skipped: %v", errorValue)
			return
		}
		if errorValue := service.syncMattermostBotProfile(ctx, profile); errorValue != nil {
			log.Printf("bot profile startup sync skipped: %v", errorValue)
		}
	}()
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
	if errorValue == nil {
		profile, parseError := parseBotProfileDocument(document)
		if parseError == nil {
			return normalizeBotProfile(profile), true
		}
	}
	for _, legacyPath := range legacyBotProfilePaths(service.Configuration.BotProfilePath) {
		legacyDocument, legacyError := os.ReadFile(legacyPath)
		if legacyError != nil {
			continue
		}
		legacyProfile, parseError := parseBotProfileDocument(legacyDocument)
		if parseError != nil {
			continue
		}
		_ = service.saveBotProfile(legacyProfile)
		return normalizeBotProfile(legacyProfile), true
	}
	return botProfile{}, false
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
	if strings.TrimSpace(userRecord.Nickname) != "" {
		profile.DisplayName = strings.TrimSpace(userRecord.Nickname)
	}
	fullName := strings.TrimSpace(strings.Join(strings.Fields(userRecord.FirstName+" "+userRecord.LastName), " "))
	if fullName != "" {
		profile.EnglishDisplayName = fullName
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
	document := []byte(renderBotProfileYAML(normalizeBotProfile(profile)))
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
	document := renderBotProfileYAML(normalizeBotProfile(profile))
	path := filepath.Join(service.Configuration.BlueclawWorkspacePath, "BOT_PROFILE.yaml")
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o750); errorValue != nil {
		return errorValue
	}
	if errorValue := os.WriteFile(path, []byte(document), 0o644); errorValue != nil {
		return errorValue
	}
	_ = os.Remove(filepath.Join(service.Configuration.BlueclawWorkspacePath, "BOT_PROFILE.md"))
	return nil
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
	body := mattermostBotProfilePatch(profile)
	if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(botRecord.ID)+"/patch", adminToken, body, nil); errorValue != nil {
		return errorValue
	}
	if errorValue := service.syncMattermostBotProfileImage(ctx, adminToken, botRecord.ID); errorValue != nil {
		return errorValue
	}
	return nil
}

func (service *Service) syncMattermostBotProfileImage(ctx context.Context, token string, userID string) error {
	imagePath := strings.TrimSpace(service.Configuration.BotProfileImagePath)
	if imagePath == "" {
		return nil
	}
	imageDocument, errorValue := os.ReadFile(imagePath)
	if errorValue != nil {
		if os.IsNotExist(errorValue) {
			return nil
		}
		return fmt.Errorf("read bot profile image: %w", errorValue)
	}
	return service.mattermostUploadUserImage(ctx, token, userID, filepath.Base(imagePath), imageDocument)
}

func (service *Service) mattermostUploadUserImage(ctx context.Context, token string, userID string, filename string, imageDocument []byte) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	imagePart, errorValue := writer.CreateFormFile("image", filename)
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := imagePart.Write(imageDocument); errorValue != nil {
		return errorValue
	}
	if errorValue := writer.Close(); errorValue != nil {
		return errorValue
	}

	requestURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/api/v4/users/" + url.PathEscape(userID) + "/image"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, body)
	if errorValue != nil {
		return errorValue
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return mattermostStatusError(response)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

func mattermostBotProfilePatch(profile botProfile) map[string]string {
	body := map[string]string{
		"nickname": firstNonEmpty(profile.DisplayName, profile.Username),
		"position": profile.PublicDescription,
	}
	firstName, lastName := identity.SplitNameForMattermost(firstNonEmpty(profile.EnglishDisplayName, profile.Username))
	if firstName != "" {
		body["first_name"] = firstName
	}
	if lastName != "" {
		body["last_name"] = lastName
	}
	return body
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

func parseBotProfileDocument(document []byte) (botProfile, error) {
	var jsonProfile botProfile
	if json.Unmarshal(document, &jsonProfile) == nil && strings.TrimSpace(jsonProfile.DisplayName) != "" {
		return jsonProfile, nil
	}
	return parseBotProfileYAML(string(document))
}

func legacyBotProfilePaths(path string) []string {
	paths := []string{}
	if strings.HasSuffix(path, ".yaml") {
		paths = append(paths, strings.TrimSuffix(path, ".yaml")+".json")
	}
	if strings.HasSuffix(path, ".yml") {
		paths = append(paths, strings.TrimSuffix(path, ".yml")+".json")
	}
	if filepath.Base(filepath.Dir(path)) == "config" && filepath.Base(path) == "bot-profile.yaml" {
		rootPath := filepath.Dir(filepath.Dir(path))
		paths = append(paths, filepath.Join(rootPath, "state", "bot-profile.yaml"), filepath.Join(rootPath, "state", "bot-profile.json"))
	}
	return uniqueStrings(paths)
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	uniqueValues := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		uniqueValues = append(uniqueValues, value)
	}
	return uniqueValues
}

func renderBotProfileYAML(profile botProfile) string {
	lines := []string{
		"username: " + yamlQuote(profile.Username),
		"displayName: " + yamlQuote(profile.DisplayName),
		"englishDisplayName: " + yamlQuote(profile.EnglishDisplayName),
		"aliases:",
	}
	for _, alias := range profile.Aliases {
		lines = append(lines, "  - "+yamlQuote(alias))
	}
	lines = append(lines,
		"publicDescription: "+yamlQuote(profile.PublicDescription),
		"identityExtension: "+yamlQuote(profile.IdentityExtension),
	)
	return strings.Join(lines, "\n") + "\n"
}

func parseBotProfileYAML(document string) (botProfile, error) {
	profile := botProfile{}
	lines := strings.Split(document, "\n")
	for index := 0; index < len(lines); index++ {
		line := strings.TrimSpace(lines[index])
		if line == "" || strings.HasPrefix(line, "#") || line == "---" {
			continue
		}
		if line == "aliases:" {
			aliases := []string{}
			for index+1 < len(lines) {
				nextLine := lines[index+1]
				trimmedNextLine := strings.TrimSpace(nextLine)
				if !strings.HasPrefix(trimmedNextLine, "- ") {
					break
				}
				aliases = append(aliases, yamlUnquote(strings.TrimSpace(strings.TrimPrefix(trimmedNextLine, "- "))))
				index++
			}
			profile.Aliases = aliases
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		setBotProfileYAMLValue(&profile, strings.TrimSpace(key), yamlUnquote(strings.TrimSpace(value)))
	}
	if strings.TrimSpace(profile.DisplayName) == "" {
		return botProfile{}, fmt.Errorf("bot profile YAML is missing displayName")
	}
	return profile, nil
}

func setBotProfileYAMLValue(profile *botProfile, key string, value string) {
	switch key {
	case "username":
		profile.Username = value
	case "displayName":
		profile.DisplayName = value
	case "englishDisplayName":
		profile.EnglishDisplayName = value
	case "publicDescription":
		profile.PublicDescription = value
	case "identityExtension":
		profile.IdentityExtension = value
	}
}

func yamlQuote(value string) string {
	document, errorValue := json.Marshal(value)
	if errorValue != nil {
		return `""`
	}
	return string(document)
}

func yamlUnquote(value string) string {
	var result string
	if json.Unmarshal([]byte(value), &result) == nil {
		return result
	}
	return strings.Trim(value, `"'`)
}
