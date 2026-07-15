package cli

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
)

const (
	mattermostProbeMaximumJSONBytes = 8 << 20
	mattermostProbeMaximumFileBytes = 64 << 20
)

type mattermostProbeAPI interface {
	Login(context.Context, string, string) (string, error)
	CurrentUser(context.Context, string) (mattermostProbeUser, error)
	CreateUser(context.Context, string, mattermostProbeNewUser) (mattermostProbeUser, error)
	TeamByName(context.Context, string, string) (mattermostProbeTeam, error)
	AddTeamMember(context.Context, string, string, string) error
	CreateDirectChannel(context.Context, string, string, string) (mattermostProbeChannel, error)
	PostMessage(context.Context, string, mattermostProbeMessage) (mattermostProbePost, error)
	ListChannelPosts(context.Context, string, string) ([]mattermostProbePost, error)
	FileMetadata(context.Context, string, string) (mattermostProbeFileMetadata, error)
	DownloadFile(context.Context, string, string) ([]byte, error)
	DeletePost(context.Context, string, string) error
	DeleteUserPermanently(context.Context, string, string) error
}

type mattermostProbeClient struct {
	baseURL    *url.URL
	httpClient *http.Client
}

var _ mattermostProbeAPI = (*mattermostProbeClient)(nil)

type mattermostProbeUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	IsBot    bool   `json:"is_bot"`
}

type mattermostProbeNewUser struct {
	Username  string `json:"username"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

type mattermostProbeChannel struct {
	ID string `json:"id"`
}

type mattermostProbeTeam struct {
	ID string `json:"id"`
}

type mattermostProbeMessage struct {
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id,omitempty"`
	Message   string `json:"message"`
}

type mattermostProbePost struct {
	ID        string   `json:"id"`
	RootID    string   `json:"root_id"`
	UserID    string   `json:"user_id"`
	Message   string   `json:"message"`
	FileIDs   []string `json:"file_ids"`
	CreatedAt int64    `json:"create_at"`
}

type mattermostProbeFileMetadata struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	MimeType string `json:"mime_type"`
	Size     int64  `json:"size"`
}

type mattermostProbePostsResponse struct {
	Order []string                       `json:"order"`
	Posts map[string]mattermostProbePost `json:"posts"`
}

type mattermostProbeHTTPError struct {
	Method     string
	RequestURL string
	StatusCode int
	Body       string
}

func (errorValue mattermostProbeHTTPError) Error() string {
	return fmt.Sprintf("Mattermost %s %s returned HTTP %d: %s", errorValue.Method, errorValue.RequestURL, errorValue.StatusCode, errorValue.Body)
}

func newMattermostProbeClient(baseURL string) (*mattermostProbeClient, error) {
	parsedURL, errorValue := url.Parse(baseURL)
	if errorValue != nil {
		return nil, fmt.Errorf("parse Mattermost base URL: %w", errorValue)
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, errors.New("Mattermost base URL requires a scheme and host")
	}
	if parsedURL.RawQuery != "" || parsedURL.Fragment != "" {
		return nil, errors.New("Mattermost base URL must not include a query or fragment")
	}
	parsedURL.Path = strings.TrimRight(parsedURL.Path, "/")
	return &mattermostProbeClient{baseURL: parsedURL, httpClient: &http.Client{}}, nil
}

func (client *mattermostProbeClient) Login(contextValue context.Context, loginID string, password string) (string, error) {
	payload := struct {
		LoginID  string `json:"login_id"`
		Password string `json:"password"`
	}{LoginID: loginID, Password: password}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return "", fmt.Errorf("encode Mattermost login request: %w", errorValue)
	}
	response, errorValue := client.sendRequest(contextValue, http.MethodPost, "/api/v4/users/login", "", document)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if errorValue := requireSuccessfulMattermostResponse(response); errorValue != nil {
		return "", errorValue
	}
	token := strings.TrimSpace(response.Header.Get("Token"))
	if token == "" {
		return "", errors.New("Mattermost login response did not include a Token header")
	}
	return token, nil
}

func (client *mattermostProbeClient) CurrentUser(contextValue context.Context, token string) (mattermostProbeUser, error) {
	var user mattermostProbeUser
	document, errorValue := client.requestJSON(contextValue, http.MethodGet, "/api/v4/users/me", token, nil)
	if errorValue == nil {
		errorValue = json.Unmarshal(document, &user)
	}
	return user, validateMattermostProbeUser(user, errorValue)
}

func (client *mattermostProbeClient) CreateUser(contextValue context.Context, token string, newUser mattermostProbeNewUser) (mattermostProbeUser, error) {
	var user mattermostProbeUser
	payload, errorValue := json.Marshal(newUser)
	if errorValue != nil {
		return mattermostProbeUser{}, fmt.Errorf("encode Mattermost user request: %w", errorValue)
	}
	document, errorValue := client.requestJSON(contextValue, http.MethodPost, "/api/v4/users", token, payload)
	if errorValue == nil {
		errorValue = json.Unmarshal(document, &user)
	}
	return user, validateMattermostProbeUser(user, errorValue)
}

func (client *mattermostProbeClient) CreateDirectChannel(contextValue context.Context, token string, firstUserID string, secondUserID string) (mattermostProbeChannel, error) {
	var channel mattermostProbeChannel
	payload, errorValue := json.Marshal([]string{firstUserID, secondUserID})
	if errorValue != nil {
		return mattermostProbeChannel{}, fmt.Errorf("encode Mattermost direct channel request: %w", errorValue)
	}
	document, errorValue := client.requestJSON(contextValue, http.MethodPost, "/api/v4/channels/direct", token, payload)
	if errorValue == nil {
		errorValue = json.Unmarshal(document, &channel)
	}
	if errorValue != nil {
		return mattermostProbeChannel{}, errorValue
	}
	if strings.TrimSpace(channel.ID) == "" {
		return mattermostProbeChannel{}, errors.New("Mattermost direct channel response has no ID")
	}
	return channel, nil
}

func (client *mattermostProbeClient) TeamByName(contextValue context.Context, token string, teamName string) (mattermostProbeTeam, error) {
	requestPath := "/api/v4/teams/name/" + url.PathEscape(teamName)
	var team mattermostProbeTeam
	document, errorValue := client.requestJSON(contextValue, http.MethodGet, requestPath, token, nil)
	if errorValue == nil {
		errorValue = json.Unmarshal(document, &team)
	}
	if errorValue != nil {
		return mattermostProbeTeam{}, errorValue
	}
	if strings.TrimSpace(team.ID) == "" {
		return mattermostProbeTeam{}, errors.New("Mattermost team response has no ID")
	}
	return team, nil
}

func (client *mattermostProbeClient) AddTeamMember(contextValue context.Context, token string, teamID string, userID string) error {
	requestPath := "/api/v4/teams/" + url.PathEscape(teamID) + "/members"
	payload, errorValue := json.Marshal(map[string]string{"team_id": teamID, "user_id": userID})
	if errorValue != nil {
		return fmt.Errorf("encode Mattermost team member request: %w", errorValue)
	}
	_, errorValue = client.requestJSON(contextValue, http.MethodPost, requestPath, token, payload)
	return errorValue
}

func (client *mattermostProbeClient) PostMessage(contextValue context.Context, token string, message mattermostProbeMessage) (mattermostProbePost, error) {
	var post mattermostProbePost
	payload, errorValue := json.Marshal(message)
	if errorValue != nil {
		return mattermostProbePost{}, fmt.Errorf("encode Mattermost post request: %w", errorValue)
	}
	document, errorValue := client.requestJSON(contextValue, http.MethodPost, "/api/v4/posts", token, payload)
	if errorValue == nil {
		errorValue = json.Unmarshal(document, &post)
	}
	if errorValue != nil {
		return mattermostProbePost{}, errorValue
	}
	if errorValue := validateMattermostProbePost(post); errorValue != nil {
		return mattermostProbePost{}, errorValue
	}
	return normalizeMattermostProbePost(post), nil
}

func (client *mattermostProbeClient) ListChannelPosts(contextValue context.Context, token string, channelID string) ([]mattermostProbePost, error) {
	requestPath := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?per_page=200"
	var response mattermostProbePostsResponse
	document, errorValue := client.requestJSON(contextValue, http.MethodGet, requestPath, token, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := json.Unmarshal(document, &response); errorValue != nil {
		return nil, fmt.Errorf("decode Mattermost channel posts response: %w", errorValue)
	}
	posts := make([]mattermostProbePost, 0, len(response.Order))
	for _, postID := range response.Order {
		post, isPresent := response.Posts[postID]
		if !isPresent {
			return nil, fmt.Errorf("Mattermost channel posts response is missing ordered post %q", postID)
		}
		if errorValue := validateMattermostProbePost(post); errorValue != nil {
			return nil, errorValue
		}
		posts = append(posts, normalizeMattermostProbePost(post))
	}
	return posts, nil
}

func (client *mattermostProbeClient) FileMetadata(contextValue context.Context, token string, fileID string) (mattermostProbeFileMetadata, error) {
	requestPath := "/api/v4/files/" + url.PathEscape(fileID) + "/info"
	var metadata mattermostProbeFileMetadata
	document, errorValue := client.requestJSON(contextValue, http.MethodGet, requestPath, token, nil)
	if errorValue != nil {
		return mattermostProbeFileMetadata{}, errorValue
	}
	if errorValue := json.Unmarshal(document, &metadata); errorValue != nil {
		return mattermostProbeFileMetadata{}, fmt.Errorf("decode Mattermost file metadata response: %w", errorValue)
	}
	if strings.TrimSpace(metadata.ID) == "" || strings.TrimSpace(metadata.Name) == "" || metadata.Size < 0 {
		return mattermostProbeFileMetadata{}, errors.New("Mattermost file metadata response is invalid")
	}
	return metadata, nil
}

func (client *mattermostProbeClient) DownloadFile(contextValue context.Context, token string, fileID string) ([]byte, error) {
	requestPath := "/api/v4/files/" + url.PathEscape(fileID)
	response, errorValue := client.sendRequest(contextValue, http.MethodGet, requestPath, token, nil)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if errorValue := requireSuccessfulMattermostResponse(response); errorValue != nil {
		return nil, errorValue
	}
	document, errorValue := io.ReadAll(io.LimitReader(response.Body, mattermostProbeMaximumFileBytes+1))
	if errorValue != nil {
		return nil, fmt.Errorf("read Mattermost file %q: %w", fileID, errorValue)
	}
	if len(document) > mattermostProbeMaximumFileBytes {
		return nil, fmt.Errorf("Mattermost file %q exceeds %d bytes", fileID, mattermostProbeMaximumFileBytes)
	}
	return document, nil
}

func (client *mattermostProbeClient) DeletePost(contextValue context.Context, token string, postID string) error {
	requestPath := "/api/v4/posts/" + url.PathEscape(postID)
	return client.requestWithoutResult(contextValue, http.MethodDelete, requestPath, token)
}

func (client *mattermostProbeClient) DeleteUserPermanently(contextValue context.Context, token string, userID string) error {
	requestPath := "/api/v4/users/" + url.PathEscape(userID) + "?permanent=true"
	return client.requestWithoutResult(contextValue, http.MethodDelete, requestPath, token)
}

func (client *mattermostProbeClient) requestJSON(contextValue context.Context, method string, requestPath string, token string, payload []byte) ([]byte, error) {
	response, errorValue := client.sendRequest(contextValue, method, requestPath, token, payload)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if errorValue := requireSuccessfulMattermostResponse(response); errorValue != nil {
		return nil, errorValue
	}
	document, errorValue := io.ReadAll(io.LimitReader(response.Body, mattermostProbeMaximumJSONBytes+1))
	if errorValue != nil {
		return nil, fmt.Errorf("read Mattermost %s %s response: %w", method, requestPath, errorValue)
	}
	if len(document) > mattermostProbeMaximumJSONBytes {
		return nil, fmt.Errorf("Mattermost %s %s response exceeds %d bytes", method, requestPath, mattermostProbeMaximumJSONBytes)
	}
	return document, nil
}

func (client *mattermostProbeClient) requestWithoutResult(contextValue context.Context, method string, requestPath string, token string) error {
	response, errorValue := client.sendRequest(contextValue, method, requestPath, token, nil)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	return requireSuccessfulMattermostResponse(response)
}

func (client *mattermostProbeClient) sendRequest(contextValue context.Context, method string, requestPath string, token string, payload []byte) (*http.Response, error) {
	if contextValue == nil {
		return nil, errors.New("Mattermost request context is required")
	}
	requestURL := strings.TrimRight(client.baseURL.String(), "/") + requestPath
	request, errorValue := http.NewRequestWithContext(contextValue, method, requestURL, bytes.NewReader(payload))
	if errorValue != nil {
		return nil, fmt.Errorf("create Mattermost %s request: %w", method, errorValue)
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return nil, fmt.Errorf("send Mattermost %s %s: %w", method, requestURL, errorValue)
	}
	return response, nil
}

func requireSuccessfulMattermostResponse(response *http.Response) error {
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	document, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return mattermostProbeHTTPError{
		Method:     response.Request.Method,
		RequestURL: response.Request.URL.String(),
		StatusCode: response.StatusCode,
		Body:       strings.TrimSpace(string(document)),
	}
}

func validateMattermostProbeUser(user mattermostProbeUser, errorValue error) error {
	if errorValue != nil {
		return errorValue
	}
	if strings.TrimSpace(user.ID) == "" || strings.TrimSpace(user.Username) == "" {
		return errors.New("Mattermost user response has no ID or username")
	}
	return nil
}

func validateMattermostProbePost(post mattermostProbePost) error {
	if strings.TrimSpace(post.ID) == "" || strings.TrimSpace(post.UserID) == "" {
		return errors.New("Mattermost post response has no ID or user ID")
	}
	return nil
}

func normalizeMattermostProbePost(post mattermostProbePost) mattermostProbePost {
	if post.FileIDs == nil {
		post.FileIDs = []string{}
	}
	return post
}
