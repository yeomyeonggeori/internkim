// Package mattermostadmin is the Mattermost client the Buzz bridge and the
// history importer use.
package mattermostadmin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const adminSessionLifetime = 30 * time.Minute

type UserRecord struct {
	ID                string `json:"id"`
	Email             string `json:"email"`
	Username          string `json:"username"`
	DisplayName       string `json:"display_name"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Nickname          string `json:"nickname"`
	Position          string `json:"position"`
	Roles             string `json:"roles"`
	DeleteAt          int64  `json:"delete_at"`
	IsBot             bool   `json:"is_bot"`
	LastPictureUpdate int64  `json:"last_picture_update"`

	Props map[string]json.RawMessage `json:"props,omitempty"`
}

type TeamRecord struct {
	ID string `json:"id"`
}

type ChannelRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Purpose     string `json:"purpose"`
	Type        string `json:"type"`
	DeleteAt    int64  `json:"delete_at"`
}

type channelMemberRecord struct {
	UserID string `json:"user_id"`
}

type Settings struct {
	BaseURL           string
	AdminPasswordPath string
	BotTokenPath      string
	LegacyTokenPath   string
	TeamName          string
	HTTPClient        *http.Client
	ReadTrimmedFile   func(string) string
}

type Client struct {
	settings     Settings
	sessionMutex sync.Mutex
	session      adminSession
}

type adminSession struct {
	token    string
	issuedAt time.Time
}

func (session adminSession) isUsableAt(moment time.Time) bool {
	return session.token != "" && moment.Sub(session.issuedAt) < adminSessionLifetime
}

func New(settings Settings) *Client {
	return &Client{settings: settings}
}

type apiError struct {
	StatusCode int
	Body       string
}

func (errorValue apiError) Error() string {
	return fmt.Sprintf("Mattermost API returned %d: %s", errorValue.StatusCode, errorValue.Body)
}

func IsNotFound(errorValue error) bool {
	var apiError apiError
	return errors.As(errorValue, &apiError) && apiError.StatusCode == http.StatusNotFound
}

func (client *Client) httpClient() *http.Client {
	if client.settings.HTTPClient != nil {
		return client.settings.HTTPClient
	}
	return http.DefaultClient
}

func (client *Client) readFile(path string) string {
	if client.settings.ReadTrimmedFile == nil {
		return ""
	}
	return strings.TrimSpace(client.settings.ReadTrimmedFile(path))
}

func (client *Client) AdminToken(ctx context.Context) (string, error) {
	client.sessionMutex.Lock()
	defer client.sessionMutex.Unlock()
	if client.session.isUsableAt(time.Now()) {
		return client.session.token, nil
	}
	token, errorValue := client.logInAsAdministrator(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	client.session = adminSession{token: token, issuedAt: time.Now()}
	return token, nil
}

func (client *Client) forgetAdminToken(refused string) {
	client.sessionMutex.Lock()
	defer client.sessionMutex.Unlock()
	if client.session.token != refused {
		return
	}
	client.session = adminSession{}
}

func (client *Client) logInAsAdministrator(ctx context.Context) (string, error) {
	adminPassword := client.readFile(client.settings.AdminPasswordPath)
	if adminPassword == "" {
		return "", fmt.Errorf("Mattermost admin password is not configured")
	}
	requestBody, errorValue := json.Marshal(map[string]string{"login_id": "admin", "password": adminPassword})
	if errorValue != nil {
		return "", errorValue
	}
	requestURL := strings.TrimRight(client.settings.BaseURL, "/") + "/api/v4/users/login"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(requestBody))
	if errorValue != nil {
		return "", errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := client.httpClient().Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", statusError(response)
	}
	token := response.Header.Get("Token")
	if token == "" {
		return "", fmt.Errorf("Mattermost login did not return a token")
	}
	return token, nil
}

func (client *Client) BotToken() (string, error) {
	token := client.readFile(client.settings.BotTokenPath)
	if token == "" {
		token = client.readFile(client.settings.LegacyTokenPath)
	}
	if token == "" {
		return "", fmt.Errorf("Mattermost bot token is not configured")
	}
	return token, nil
}

func (client *Client) Request(ctx context.Context, method string, path string, token string, body any, responseValue any) error {
	var requestBody io.Reader
	if body != nil {
		document, errorValue := json.Marshal(body)
		if errorValue != nil {
			return errorValue
		}
		requestBody = bytes.NewReader(document)
	}
	requestURL := strings.TrimRight(client.settings.BaseURL, "/") + path
	request, errorValue := http.NewRequestWithContext(ctx, method, requestURL, requestBody)
	if errorValue != nil {
		return errorValue
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, errorValue := client.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusUnauthorized {
		client.forgetAdminToken(token)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return statusError(response)
	}
	if responseValue == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	return json.NewDecoder(response.Body).Decode(responseValue)
}

func statusError(response *http.Response) error {
	document, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return apiError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(document))}
}

func (client *Client) FindUserByEmail(ctx context.Context, token string, email string) (UserRecord, bool, error) {
	return client.findUser(ctx, token, "/api/v4/users/email/"+url.PathEscape(email))
}

func (client *Client) findUserByID(ctx context.Context, token string, userID string) (UserRecord, bool, error) {
	return client.findUser(ctx, token, "/api/v4/users/"+url.PathEscape(userID))
}

func (client *Client) findUser(ctx context.Context, token string, path string) (UserRecord, bool, error) {
	var userRecord UserRecord
	errorValue := client.Request(ctx, http.MethodGet, path, token, nil, &userRecord)
	if errorValue == nil {
		return userRecord, true, nil
	}
	if IsNotFound(errorValue) {
		return UserRecord{}, false, nil
	}
	return UserRecord{}, false, errorValue
}

func (client *Client) EnsureTeam(ctx context.Context, token string) (TeamRecord, error) {
	teamName := strings.TrimSpace(client.settings.TeamName)
	if teamName == "" {
		teamName = "internkim"
	}
	var teamRecord TeamRecord
	errorValue := client.Request(ctx, http.MethodGet, "/api/v4/teams/name/"+url.PathEscape(teamName), token, nil, &teamRecord)
	if errorValue == nil && teamRecord.ID != "" {
		return teamRecord, nil
	}
	if errorValue != nil && !IsNotFound(errorValue) {
		return TeamRecord{}, errorValue
	}
	body := map[string]string{"name": teamName, "display_name": "Intern Kim", "type": "I"}
	if errorValue := client.Request(ctx, http.MethodPost, "/api/v4/teams", token, body, &teamRecord); errorValue != nil {
		return TeamRecord{}, errorValue
	}
	if teamRecord.ID == "" {
		return TeamRecord{}, fmt.Errorf("Mattermost team %s was not created", teamName)
	}
	return teamRecord, nil
}

func (client *Client) ChannelMemberEmails(ctx context.Context, token string, channelID string) (map[string]bool, error) {
	var members []channelMemberRecord
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/members?per_page=200"
	if errorValue := client.Request(ctx, http.MethodGet, path, token, nil, &members); errorValue != nil {
		return nil, errorValue
	}
	emails := map[string]bool{}
	for _, member := range members {
		userRecord, isFound, errorValue := client.findUserByID(ctx, token, member.UserID)
		if errorValue != nil {
			return nil, errorValue
		}
		if !isFound {
			continue
		}
		if email := strings.ToLower(strings.TrimSpace(userRecord.Email)); email != "" {
			emails[email] = true
		}
	}
	return emails, nil
}
