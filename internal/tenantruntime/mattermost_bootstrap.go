package tenantruntime

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

const defaultMattermostTeamName = "internkim"
const defaultMattermostBotUsername = "internkim"
const defaultMattermostBotDisplayName = "김인턴"
const defaultMattermostBotWelcomeMessage = "Please add me to teams and channels you want me to interact in. To do this, use the browser or Mattermost Desktop App."

type MattermostFleetBootstrapOptions struct {
	CredentialsPath     string
	BaseURLTemplate     string
	BlueclawURLTemplate string
	PublicURLTemplate   string
	PortStart           int
	Language            string
	TokenOutputRoot     string
	AdminEmail          string
	Members             []MattermostBootstrapMember
}

type MattermostBootstrapStatus struct {
	TenantID        string                            `json:"tenantID"`
	PublicURL       string                            `json:"publicURL"`
	AdminEmail      string                            `json:"adminEmail"`
	BotUserID       string                            `json:"botUserID"`
	BlueclawInvited bool                              `json:"blueclawInvited,omitempty"`
	Channels        map[string]string                 `json:"channels"`
	Members         []MattermostBootstrapMemberResult `json:"members,omitempty"`
}

type MattermostBootstrapMember struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password,omitempty"`
}

type MattermostBootstrapMemberResult struct {
	Email           string `json:"email"`
	Name            string `json:"name"`
	Username        string `json:"username,omitempty"`
	UserID          string `json:"userID,omitempty"`
	Outcome         string `json:"outcome"`
	Reason          string `json:"reason,omitempty"`
	BlueclawInvited bool   `json:"blueclawInvited,omitempty"`
}

type mattermostBootstrapCredential struct {
	TenantID      string `json:"tenantID"`
	AdminUsername string `json:"adminUsername"`
	AdminPassword string `json:"adminPassword"`
}

type mattermostBootstrapClient struct {
	baseURL    string
	httpClient *http.Client
}

var mattermostUsernameInvalidCharacterPattern = regexp.MustCompile(`[^a-z0-9._-]+`)

func BootstrapMattermostFleetResources(options MattermostFleetBootstrapOptions) ([]MattermostBootstrapStatus, error) {
	if errorValue := validateMattermostFleetBootstrapOptions(options); errorValue != nil {
		return nil, errorValue
	}
	credentials, errorValue := readMattermostBootstrapCredentials(options.CredentialsPath)
	if errorValue != nil {
		return nil, errorValue
	}
	statuses := make([]MattermostBootstrapStatus, 0, len(credentials))
	for _, credential := range credentials {
		status, errorValue := bootstrapMattermostTenantResources(credential, options)
		if errorValue != nil {
			return nil, errorValue
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func BootstrapMattermostTenantResources(tenantID string, adminUsername string, adminPassword string, options MattermostFleetBootstrapOptions) (MattermostBootstrapStatus, error) {
	if errorValue := validateMattermostBootstrapTemplates(options); errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	credential := mattermostBootstrapCredential{
		TenantID:      strings.TrimSpace(tenantID),
		AdminUsername: strings.TrimSpace(adminUsername),
		AdminPassword: adminPassword,
	}
	if errorValue := validateMattermostBootstrapCredential(credential); errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	return bootstrapMattermostTenantResources(credential, options)
}

func validateMattermostFleetBootstrapOptions(options MattermostFleetBootstrapOptions) error {
	if strings.TrimSpace(options.CredentialsPath) == "" {
		return errors.New("credentials path is required")
	}
	return validateMattermostBootstrapTemplates(options)
}

func validateMattermostBootstrapTemplates(options MattermostFleetBootstrapOptions) error {
	if strings.TrimSpace(options.BaseURLTemplate) == "" {
		return errors.New("base URL template is required")
	}
	if strings.TrimSpace(options.PublicURLTemplate) == "" {
		return errors.New("public URL template is required")
	}
	return nil
}

func readMattermostBootstrapCredentials(path string) ([]mattermostBootstrapCredential, error) {
	document, errorValue := os.ReadFile(path)
	if errorValue != nil {
		return nil, errorValue
	}
	credentials := []mattermostBootstrapCredential{}
	if errorValue := json.Unmarshal(document, &credentials); errorValue != nil {
		return nil, errorValue
	}
	for _, credential := range credentials {
		if errorValue := validateMattermostBootstrapCredential(credential); errorValue != nil {
			return nil, errors.New("Mattermost credential file contains an incomplete tenant credential")
		}
	}
	return credentials, nil
}

func validateMattermostBootstrapCredential(credential mattermostBootstrapCredential) error {
	if strings.TrimSpace(credential.TenantID) == "" {
		return errors.New("tenant id is required")
	}
	if strings.TrimSpace(credential.AdminUsername) == "" {
		return errors.New("Mattermost admin username is required")
	}
	if strings.TrimSpace(credential.AdminPassword) == "" {
		return errors.New("Mattermost admin password is required")
	}
	return nil
}

func bootstrapMattermostTenantResources(credential mattermostBootstrapCredential, options MattermostFleetBootstrapOptions) (MattermostBootstrapStatus, error) {
	tenantPort, errorValue := mattermostTenantPort(credential.TenantID, options.PortStart)
	if errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	baseURL := expandMattermostBootstrapTemplate(options.BaseURLTemplate, credential.TenantID, tenantPort)
	publicURL := expandMattermostBootstrapTemplate(options.PublicURLTemplate, credential.TenantID, tenantPort)
	client := mattermostBootstrapClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: http.DefaultClient}
	adminToken, adminUserID, errorValue := client.login(credential.AdminUsername, credential.AdminPassword)
	if errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	adminEmail := firstNonEmptyMattermostBootstrapString(options.AdminEmail, DefaultTenantAdminEmail(credential.TenantID))
	if errorValue := client.ensureAdminProfile(adminToken, adminUserID, adminEmail, credential.AdminPassword); errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	if errorValue := client.configure(adminToken, publicURL); errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	teamID, errorValue := client.ensureTeam(adminToken)
	if errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	if errorValue := client.ensureTeamMember(adminToken, teamID, adminUserID); errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	botUserID, botToken, errorValue := client.ensureBot(adminToken, teamID)
	if errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	memberResults := client.ensureMembers(adminToken, teamID, credential.TenantID, options)
	channelUserIDs := append([]string{adminUserID, botUserID}, successfulMattermostMemberUserIDs(memberResults)...)
	channels, errorValue := client.ensureDefaultChannels(adminToken, teamID, channelUserIDs, botUserID, options.Language)
	if errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	if errorValue := client.cleanupDefaultBotWelcomeMessage(adminToken, adminUserID, botUserID); errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	blueclawInvited, errorValue := inviteMattermostBootstrapBlueclawAdmin(credential.TenantID, adminEmail, options)
	if errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	if strings.TrimSpace(botToken) != "" && strings.TrimSpace(options.TokenOutputRoot) != "" {
		if errorValue := writeMattermostBotToken(options.TokenOutputRoot, credential.TenantID, botToken); errorValue != nil {
			return MattermostBootstrapStatus{}, errorValue
		}
	}
	return MattermostBootstrapStatus{
		TenantID:        credential.TenantID,
		PublicURL:       publicURL,
		AdminEmail:      adminEmail,
		BotUserID:       botUserID,
		BlueclawInvited: blueclawInvited,
		Channels:        channels,
		Members:         memberResults,
	}, nil
}

func mattermostTenantPort(tenantID string, portStart int) (int, error) {
	if portStart <= 0 {
		return 0, errors.New("port start is required")
	}
	parts := strings.Split(strings.TrimSpace(tenantID), "-")
	if len(parts) == 0 {
		return 0, errors.New("tenant id is required")
	}
	index, errorValue := strconv.Atoi(parts[len(parts)-1])
	if errorValue != nil || index <= 0 {
		return 0, errors.New("tenant id must end with a numeric index")
	}
	return portStart + index - 1, nil
}

func expandMattermostBootstrapTemplate(template string, tenantID string, port int) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(template), "{tenant}", tenantID), "{port}", strconv.Itoa(port))
}

func expandMattermostBootstrapBlueclawTemplate(template string, tenantID string) string {
	tenantIndex := tenantNumericSuffix(tenantID)
	blueclawPort := defaultTenantHostRuntimePortBase(tenantIndex)
	expandedURL := strings.TrimSpace(template)
	expandedURL = strings.ReplaceAll(expandedURL, "{tenant}", tenantID)
	expandedURL = strings.ReplaceAll(expandedURL, "{blueclawPort}", strconv.Itoa(blueclawPort))
	return expandedURL
}

func firstNonEmptyMattermostBootstrapString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (client mattermostBootstrapClient) login(username string, password string) (string, string, error) {
	response, errorValue := client.request(http.MethodPost, "/api/v4/users/login", "", map[string]string{
		"login_id": strings.TrimSpace(username),
		"password": password,
	})
	if errorValue != nil {
		return "", "", errorValue
	}
	token := strings.TrimSpace(response.Header.Get("Token"))
	if response.StatusCode != http.StatusOK || token == "" {
		return "", "", errors.New("Mattermost admin login failed")
	}
	return token, response.stringField("id"), nil
}

func (client mattermostBootstrapClient) ensureAdminProfile(adminToken string, adminUserID string, adminEmail string, adminPassword string) error {
	if strings.TrimSpace(adminUserID) == "" {
		return errors.New("Mattermost admin user id is required")
	}
	response, errorValue := client.request(http.MethodPut, "/api/v4/users/"+url.PathEscape(adminUserID)+"/patch", adminToken, map[string]string{
		"email":    strings.TrimSpace(adminEmail),
		"password": adminPassword,
	})
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return errors.New("Mattermost admin profile patch failed")
	}
	response, errorValue = client.request(http.MethodPut, "/api/v4/users/"+url.PathEscape(adminUserID)+"/roles", adminToken, map[string]string{
		"roles": "system_admin system_user",
	})
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return errors.New("Mattermost admin role patch failed")
	}
	return nil
}

func (client mattermostBootstrapClient) configure(adminToken string, publicURL string) error {
	response, errorValue := client.request(http.MethodPut, "/api/v4/config/patch", adminToken, map[string]any{
		"ServiceSettings": map[string]any{
			"SiteURL":                             strings.TrimSpace(publicURL),
			"AllowCorsFrom":                       strings.TrimSpace(publicURL),
			"CorsAllowCredentials":                true,
			"AllowedUntrustedInternalConnections": "127.0.0.1 localhost",
			"EnableUserAccessTokens":              true,
			"EnableBotAccountCreation":            true,
			"ManagedResourcePaths":                mattermostdefaults.ManagedResourcePathSetting(),
		},
		"TeamSettings": map[string]any{
			"TeammateNameDisplay": "nickname_full_name",
		},
		"EmailSettings": map[string]any{
			"SendPushNotifications":    true,
			"PushNotificationServer":   "https://push-test.mattermost.com",
			"PushNotificationContents": "id_loaded",
		},
	})
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return errors.New("Mattermost configuration patch failed")
	}
	return nil
}

func (client mattermostBootstrapClient) ensureTeam(adminToken string) (string, error) {
	response, errorValue := client.request(http.MethodGet, "/api/v4/teams/name/"+url.PathEscape(defaultMattermostTeamName), adminToken, nil)
	if errorValue != nil {
		return "", errorValue
	}
	if response.StatusCode == http.StatusOK {
		return response.stringField("id"), nil
	}
	response, errorValue = client.request(http.MethodPost, "/api/v4/teams", adminToken, map[string]string{
		"name":         defaultMattermostTeamName,
		"display_name": "Intern Kim",
		"type":         "I",
	})
	if errorValue != nil {
		return "", errorValue
	}
	teamID := response.stringField("id")
	if !isSuccessStatus(response.StatusCode) || teamID == "" {
		return "", errors.New("Mattermost team create failed")
	}
	return teamID, nil
}

func (client mattermostBootstrapClient) ensureTeamMember(adminToken string, teamID string, userID string) error {
	if strings.TrimSpace(teamID) == "" || strings.TrimSpace(userID) == "" {
		return errors.New("Mattermost team member requires team and user ids")
	}
	response, errorValue := client.request(http.MethodPost, "/api/v4/teams/"+url.PathEscape(teamID)+"/members", adminToken, map[string]string{
		"team_id": teamID,
		"user_id": userID,
	})
	if errorValue != nil {
		return errorValue
	}
	if response.StatusCode == http.StatusCreated || response.StatusCode == http.StatusOK || response.StatusCode == http.StatusBadRequest {
		return nil
	}
	return errors.New("Mattermost team member create failed")
}

func (client mattermostBootstrapClient) ensureMembers(adminToken string, teamID string, tenantID string, options MattermostFleetBootstrapOptions) []MattermostBootstrapMemberResult {
	results := make([]MattermostBootstrapMemberResult, 0, len(options.Members))
	for _, member := range options.Members {
		results = append(results, client.ensureMember(adminToken, teamID, tenantID, options, member))
	}
	return results
}

func (client mattermostBootstrapClient) ensureMember(adminToken string, teamID string, tenantID string, options MattermostFleetBootstrapOptions, member MattermostBootstrapMember) MattermostBootstrapMemberResult {
	result := MattermostBootstrapMemberResult{
		Email: strings.TrimSpace(member.Email),
		Name:  strings.TrimSpace(member.Name),
	}
	username := mattermostUsernameFromEmail(member.Email)
	if errorValue := validateMattermostBootstrapMember(member); errorValue != nil {
		result.Outcome = "failed"
		result.Username = username
		result.Reason = errorValue.Error()
		return result
	}
	userID, username, alreadyExisted, errorValue := client.ensureUser(adminToken, member, username)
	if errorValue != nil {
		result.Outcome = "failed"
		result.Username = username
		result.Reason = errorValue.Error()
		return result
	}
	if errorValue := client.ensureTeamMember(adminToken, teamID, userID); errorValue != nil {
		result.Outcome = "failed"
		result.Username = username
		result.UserID = userID
		result.Reason = errorValue.Error()
		return result
	}
	blueclawInvited, errorValue := inviteMattermostBootstrapBlueclawMember(tenantID, member, username, options)
	if errorValue != nil {
		result.Outcome = "failed"
		result.Username = username
		result.UserID = userID
		result.Reason = errorValue.Error()
		return result
	}
	result.Username = username
	result.UserID = userID
	result.BlueclawInvited = blueclawInvited
	if alreadyExisted {
		result.Outcome = "already existed"
		return result
	}
	result.Outcome = "created"
	return result
}

func validateMattermostBootstrapMember(member MattermostBootstrapMember) error {
	if strings.TrimSpace(member.Email) == "" {
		return errors.New("member email is required")
	}
	address, errorValue := mail.ParseAddress(strings.TrimSpace(member.Email))
	if errorValue != nil || address.Address != strings.TrimSpace(member.Email) {
		return errors.New("member email is invalid")
	}
	if strings.TrimSpace(member.Name) == "" {
		return errors.New("member name is required")
	}
	if strings.TrimSpace(member.Password) == "" {
		return errors.New("member password is required")
	}
	return nil
}

func (client mattermostBootstrapClient) ensureUser(adminToken string, member MattermostBootstrapMember, username string) (string, string, bool, error) {
	userID, errorValue := client.findUserIDByEmail(adminToken, member.Email)
	if errorValue != nil {
		return "", username, false, errorValue
	}
	if userID != "" {
		return userID, username, true, nil
	}
	userID, errorValue = client.createUser(adminToken, member, username)
	if errorValue == nil {
		return userID, username, false, nil
	}
	userID, lookupError := client.findUserIDByEmail(adminToken, member.Email)
	if lookupError != nil {
		return "", username, false, lookupError
	}
	if userID != "" {
		return userID, username, true, nil
	}
	fallbackUsername := mattermostUsernameWithEmailSuffix(username, member.Email)
	if fallbackUsername == username {
		return "", username, false, errorValue
	}
	userID, errorValue = client.createUser(adminToken, member, fallbackUsername)
	if errorValue != nil {
		return "", fallbackUsername, false, errorValue
	}
	return userID, fallbackUsername, false, nil
}

func (client mattermostBootstrapClient) findUserIDByEmail(adminToken string, email string) (string, error) {
	response, errorValue := client.request(http.MethodGet, "/api/v4/users/email/"+url.PathEscape(strings.TrimSpace(email)), adminToken, nil)
	if errorValue != nil {
		return "", errorValue
	}
	if response.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if !isSuccessStatus(response.StatusCode) {
		return "", errors.New("Mattermost user lookup failed: " + response.errorMessage())
	}
	return response.stringField("id"), nil
}

func (client mattermostBootstrapClient) createUser(adminToken string, member MattermostBootstrapMember, username string) (string, error) {
	response, errorValue := client.request(http.MethodPost, "/api/v4/users", adminToken, map[string]string{
		"email":      strings.TrimSpace(member.Email),
		"username":   username,
		"password":   member.Password,
		"first_name": strings.TrimSpace(member.Name),
		"nickname":   strings.TrimSpace(member.Name),
	})
	if errorValue != nil {
		return "", errorValue
	}
	userID := response.stringField("id")
	if !isSuccessStatus(response.StatusCode) || userID == "" {
		return "", errors.New("Mattermost user create failed: " + response.errorMessage())
	}
	return userID, nil
}

func mattermostUsernameFromEmail(email string) string {
	localPart := strings.Split(strings.TrimSpace(strings.ToLower(email)), "@")[0]
	username := mattermostUsernameInvalidCharacterPattern.ReplaceAllString(localPart, "-")
	username = strings.Trim(username, "._-")
	if len(username) > 22 {
		username = username[:22]
		username = strings.Trim(username, "._-")
	}
	if len(username) >= 3 {
		return username
	}
	if username == "" {
		return "user"
	}
	return username + strings.Repeat("0", 3-len(username))
}

func mattermostUsernameWithEmailSuffix(username string, email string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(strings.ToLower(email))))
	suffix := hex.EncodeToString(digest[:])[:6]
	prefixLength := 22 - len(suffix) - 1
	if prefixLength <= 0 {
		return username
	}
	prefix := username
	if len(prefix) > prefixLength {
		prefix = strings.Trim(prefix[:prefixLength], "._-")
	}
	if prefix == "" {
		prefix = "user"
	}
	return prefix + "-" + suffix
}

func successfulMattermostMemberUserIDs(results []MattermostBootstrapMemberResult) []string {
	userIDs := []string{}
	for _, result := range results {
		if strings.TrimSpace(result.UserID) != "" && result.Outcome != "failed" {
			userIDs = append(userIDs, result.UserID)
		}
	}
	return userIDs
}

func (client mattermostBootstrapClient) ensureBot(adminToken string, teamID string) (string, string, error) {
	response, errorValue := client.request(http.MethodPost, "/api/v4/bots", adminToken, map[string]string{
		"username":     defaultMattermostBotUsername,
		"display_name": defaultMattermostBotDisplayName,
	})
	if errorValue != nil {
		return "", "", errorValue
	}
	botUserID := ""
	if isSuccessStatus(response.StatusCode) {
		botUserID = response.stringField("user_id")
	}
	if botUserID == "" {
		response, errorValue = client.request(http.MethodGet, "/api/v4/users/username/"+url.PathEscape(defaultMattermostBotUsername), adminToken, nil)
		if errorValue != nil {
			return "", "", errorValue
		}
		botUserID = response.stringField("id")
	}
	if botUserID == "" {
		return "", "", errors.New("Mattermost bot create failed")
	}
	userResponse, errorValue := client.request(http.MethodGet, "/api/v4/users/"+url.PathEscape(botUserID), adminToken, nil)
	if errorValue != nil {
		return "", "", errorValue
	}
	if !isSuccessStatus(userResponse.StatusCode) || !userResponse.boolField("is_bot") {
		return "", "", errors.New("Mattermost InternKim account exists but is not a bot")
	}
	if errorValue := client.patchBotProfile(adminToken, botUserID); errorValue != nil {
		return "", "", errorValue
	}
	_, _ = client.request(http.MethodPost, "/api/v4/teams/"+url.PathEscape(teamID)+"/members", adminToken, map[string]string{
		"team_id": teamID,
		"user_id": botUserID,
	})
	tokenResponse, errorValue := client.request(http.MethodPost, "/api/v4/users/"+url.PathEscape(botUserID)+"/tokens", adminToken, map[string]string{
		"description": "internkim-bot",
	})
	if errorValue != nil {
		return "", "", errorValue
	}
	return botUserID, tokenResponse.stringField("token"), nil
}

func (client mattermostBootstrapClient) patchBotProfile(adminToken string, botUserID string) error {
	response, errorValue := client.request(http.MethodPut, "/api/v4/users/"+url.PathEscape(botUserID)+"/patch", adminToken, map[string]string{
		"first_name": "Intern",
		"last_name":  "Kim",
		"nickname":   defaultMattermostBotDisplayName,
		"position":   "",
	})
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return errors.New("Mattermost bot profile patch failed")
	}
	return nil
}

func (client mattermostBootstrapClient) ensureDefaultChannels(adminToken string, teamID string, memberUserIDs []string, botUserID string, language string) (map[string]string, error) {
	channels := map[string]string{}
	for _, channel := range append(mattermostdefaults.PublicChannelsForLanguage(language), defaultPrivateMattermostChannels()...) {
		channelID, errorValue := client.ensureChannel(adminToken, teamID, memberUserIDs, botUserID, channel)
		if errorValue != nil {
			return nil, errorValue
		}
		channels[channel.Name] = channelID
	}
	return channels, nil
}

func defaultPrivateMattermostChannels() []mattermostdefaults.PublicChannel {
	return []mattermostdefaults.PublicChannel{
		{Name: "circle-c-level", DisplayName: mattermostdefaults.CircleChannelDisplayName("circle-c-level"), Purpose: "private"},
		{Name: "circle-representative", DisplayName: mattermostdefaults.CircleChannelDisplayName("circle-representative"), Purpose: "private"},
		{Name: "circle-admin", DisplayName: mattermostdefaults.CircleChannelDisplayName("circle-admin"), Purpose: "private"},
	}
}

func (client mattermostBootstrapClient) ensureChannel(adminToken string, teamID string, memberUserIDs []string, botUserID string, channel mattermostdefaults.PublicChannel) (string, error) {
	response, errorValue := client.request(http.MethodGet, "/api/v4/teams/"+url.PathEscape(teamID)+"/channels/name/"+url.PathEscape(channel.Name), adminToken, nil)
	if errorValue != nil {
		return "", errorValue
	}
	channelID := ""
	if response.StatusCode == http.StatusOK {
		channelID = response.stringField("id")
	}
	if channelID == "" {
		channelType := "O"
		if channel.Purpose == "private" {
			channelType = "P"
		}
		response, errorValue = client.request(http.MethodPost, "/api/v4/channels", adminToken, map[string]string{
			"team_id":      teamID,
			"name":         channel.Name,
			"display_name": channel.DisplayName,
			"type":         channelType,
		})
		if errorValue != nil {
			return "", errorValue
		}
		channelID = response.stringField("id")
	}
	if channelID == "" {
		return "", errors.New("Mattermost channel create failed: " + channel.Name)
	}
	if errorValue := client.patchChannel(adminToken, channelID, channel); errorValue != nil {
		return "", errorValue
	}
	for _, memberUserID := range memberUserIDs {
		if errorValue := client.ensureChannelMember(adminToken, channelID, memberUserID); errorValue != nil {
			return "", errorValue
		}
	}
	if errorValue := client.ensureChannelBotCanPost(adminToken, channelID, botUserID); errorValue != nil {
		return "", errorValue
	}
	return channelID, nil
}

func (client mattermostBootstrapClient) ensureChannelMember(adminToken string, channelID string, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	response, errorValue := client.request(http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", adminToken, map[string]string{"user_id": userID})
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) && response.StatusCode != http.StatusBadRequest {
		return errors.New("Mattermost channel member create failed")
	}
	return nil
}

func (client mattermostBootstrapClient) ensureChannelBotCanPost(adminToken string, channelID string, botUserID string) error {
	if strings.TrimSpace(botUserID) == "" {
		return nil
	}
	response, errorValue := client.request(http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/members/"+url.PathEscape(botUserID)+"/schemeRoles", adminToken, map[string]bool{"scheme_admin": true, "scheme_user": true})
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return errors.New("Mattermost bot channel scheme role update failed")
	}
	return nil
}

func (client mattermostBootstrapClient) cleanupDefaultBotWelcomeMessage(adminToken string, adminUserID string, botUserID string) error {
	if strings.TrimSpace(adminUserID) == "" || strings.TrimSpace(botUserID) == "" {
		return nil
	}
	response, errorValue := client.request(http.MethodPost, "/api/v4/channels/direct", adminToken, []string{adminUserID, botUserID})
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return errors.New("Mattermost direct channel lookup failed")
	}
	channelID := response.stringField("id")
	if channelID == "" {
		return nil
	}
	response, errorValue = client.request(http.MethodGet, "/api/v4/channels/"+url.PathEscape(channelID)+"/posts?per_page=200", adminToken, nil)
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return errors.New("Mattermost direct channel posts lookup failed")
	}
	postID := mattermostDefaultBotWelcomePostID(response.Document)
	if postID == "" {
		return nil
	}
	response, errorValue = client.request(http.MethodDelete, "/api/v4/posts/"+url.PathEscape(postID), adminToken, nil)
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return errors.New("Mattermost default bot welcome post delete failed")
	}
	return nil
}

func mattermostDefaultBotWelcomePostID(document []byte) string {
	var response struct {
		Order []string `json:"order"`
		Posts map[string]struct {
			Message string `json:"message"`
		} `json:"posts"`
	}
	if json.Unmarshal(document, &response) != nil || len(response.Order) == 0 {
		return ""
	}
	oldestPostID := response.Order[len(response.Order)-1]
	if response.Posts[oldestPostID].Message != defaultMattermostBotWelcomeMessage {
		return ""
	}
	return oldestPostID
}

func (client mattermostBootstrapClient) patchChannel(adminToken string, channelID string, channel mattermostdefaults.PublicChannel) error {
	response, errorValue := client.request(http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", adminToken, map[string]string{
		"display_name": channel.DisplayName,
		"header":       channel.Header,
		"purpose":      "",
	})
	if errorValue != nil {
		return errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return errors.New("Mattermost channel patch failed: " + channel.Name)
	}
	return nil
}

func (client mattermostBootstrapClient) request(method string, path string, token string, body any) (mattermostBootstrapResponse, error) {
	var reader io.Reader
	if body != nil {
		document, errorValue := json.Marshal(body)
		if errorValue != nil {
			return mattermostBootstrapResponse{}, errorValue
		}
		reader = bytes.NewReader(document)
	}
	request, errorValue := http.NewRequest(method, client.baseURL+path, reader)
	if errorValue != nil {
		return mattermostBootstrapResponse{}, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		request.Header.Set("Authorization", "Bearer "+strings.TrimSpace(token))
	}
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return mattermostBootstrapResponse{}, errorValue
	}
	defer response.Body.Close()
	document, errorValue := io.ReadAll(response.Body)
	if errorValue != nil {
		return mattermostBootstrapResponse{}, errorValue
	}
	return mattermostBootstrapResponse{StatusCode: response.StatusCode, Header: response.Header, Document: document}, nil
}

func inviteMattermostBootstrapBlueclawAdmin(tenantID string, adminEmail string, options MattermostFleetBootstrapOptions) (bool, error) {
	return inviteMattermostBootstrapBlueclawPerson(tenantID, "admin", adminEmail, "Intern Kim Admin", options)
}

func inviteMattermostBootstrapBlueclawMember(tenantID string, member MattermostBootstrapMember, username string, options MattermostFleetBootstrapOptions) (bool, error) {
	return inviteMattermostBootstrapBlueclawPerson(tenantID, username, member.Email, member.Name, options)
}

func inviteMattermostBootstrapBlueclawPerson(tenantID string, personIDPart string, email string, displayName string, options MattermostFleetBootstrapOptions) (bool, error) {
	template := strings.TrimSpace(options.BlueclawURLTemplate)
	if template == "" {
		return false, nil
	}
	baseURL := expandMattermostBootstrapBlueclawTemplate(template, tenantID)
	client := mattermostBootstrapClient{baseURL: strings.TrimRight(baseURL, "/"), httpClient: http.DefaultClient}
	response, errorValue := client.request(http.MethodPost, "/admin/api/people/invite", "", map[string]string{
		"personID":    "tenant-" + strings.TrimSpace(tenantID) + "-" + strings.TrimSpace(personIDPart),
		"email":       strings.TrimSpace(email),
		"displayName": strings.TrimSpace(displayName),
	})
	if errorValue != nil {
		return false, errorValue
	}
	if !isSuccessStatus(response.StatusCode) {
		return false, errors.New("Blueclaw invite failed")
	}
	return true, nil
}

type mattermostBootstrapResponse struct {
	StatusCode int
	Header     http.Header
	Document   []byte
}

func (response mattermostBootstrapResponse) stringField(name string) string {
	document := map[string]any{}
	if json.Unmarshal(response.Document, &document) != nil {
		return ""
	}
	value, isString := document[name].(string)
	if !isString {
		return ""
	}
	return strings.TrimSpace(value)
}

func (response mattermostBootstrapResponse) boolField(name string) bool {
	document := map[string]any{}
	if json.Unmarshal(response.Document, &document) != nil {
		return false
	}
	value, isBoolean := document[name].(bool)
	return isBoolean && value
}

func (response mattermostBootstrapResponse) errorMessage() string {
	for _, fieldName := range []string{"message", "error", "id"} {
		value := response.stringField(fieldName)
		if value != "" {
			return value
		}
	}
	return http.StatusText(response.StatusCode)
}

func isSuccessStatus(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

func writeMattermostBotToken(rootPath string, tenantID string, token string) error {
	tokenPath := filepath.Join(filepath.Clean(rootPath), tenantID, "internkim", "secrets", "mattermost-bot-token")
	if errorValue := os.MkdirAll(filepath.Dir(tokenPath), 0o700); errorValue != nil {
		return errorValue
	}
	return os.WriteFile(tokenPath, []byte(strings.TrimSpace(token)), 0o600)
}
