package tenantruntime

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

const defaultMattermostTeamName = "internkim"
const defaultMattermostBotUsername = "internkim"
const defaultMattermostBotDisplayName = "김인턴"

type MattermostFleetBootstrapOptions struct {
	CredentialsPath   string
	BaseURLTemplate   string
	PublicURLTemplate string
	PortStart         int
	Language          string
	TokenOutputRoot   string
}

type MattermostBootstrapStatus struct {
	TenantID  string            `json:"tenantID"`
	PublicURL string            `json:"publicURL"`
	BotUserID string            `json:"botUserID"`
	Channels  map[string]string `json:"channels"`
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

func BootstrapMattermostFleetResources(options MattermostFleetBootstrapOptions) ([]MattermostBootstrapStatus, error) {
	if strings.TrimSpace(options.CredentialsPath) == "" {
		return nil, errors.New("credentials path is required")
	}
	if strings.TrimSpace(options.BaseURLTemplate) == "" {
		return nil, errors.New("base URL template is required")
	}
	if strings.TrimSpace(options.PublicURLTemplate) == "" {
		return nil, errors.New("public URL template is required")
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
		if strings.TrimSpace(credential.TenantID) == "" || strings.TrimSpace(credential.AdminUsername) == "" || strings.TrimSpace(credential.AdminPassword) == "" {
			return nil, errors.New("Mattermost credential file contains an incomplete tenant credential")
		}
	}
	return credentials, nil
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
	channels, errorValue := client.ensureDefaultChannels(adminToken, teamID, botUserID, options.Language)
	if errorValue != nil {
		return MattermostBootstrapStatus{}, errorValue
	}
	if strings.TrimSpace(botToken) != "" && strings.TrimSpace(options.TokenOutputRoot) != "" {
		if errorValue := writeMattermostBotToken(options.TokenOutputRoot, credential.TenantID, botToken); errorValue != nil {
			return MattermostBootstrapStatus{}, errorValue
		}
	}
	return MattermostBootstrapStatus{
		TenantID:  credential.TenantID,
		PublicURL: publicURL,
		BotUserID: botUserID,
		Channels:  channels,
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

func (client mattermostBootstrapClient) ensureBot(adminToken string, teamID string) (string, string, error) {
	response, errorValue := client.request(http.MethodPost, "/api/v4/bots", adminToken, map[string]string{
		"username":     defaultMattermostBotUsername,
		"display_name": defaultMattermostBotDisplayName,
	})
	if errorValue != nil {
		return "", "", errorValue
	}
	botUserID := response.stringField("user_id")
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

func (client mattermostBootstrapClient) ensureDefaultChannels(adminToken string, teamID string, botUserID string, language string) (map[string]string, error) {
	channels := map[string]string{}
	for _, channel := range append(mattermostdefaults.PublicChannelsForLanguage(language), defaultPrivateMattermostChannels()...) {
		channelID, errorValue := client.ensureChannel(adminToken, teamID, botUserID, channel)
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

func (client mattermostBootstrapClient) ensureChannel(adminToken string, teamID string, botUserID string, channel mattermostdefaults.PublicChannel) (string, error) {
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
	_, _ = client.request(http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", adminToken, map[string]string{"user_id": botUserID})
	return channelID, nil
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
