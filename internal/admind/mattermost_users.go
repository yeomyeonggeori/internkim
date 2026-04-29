package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"
)

type adminUserMutation struct {
	Email                  string `json:"email"`
	Role                   string `json:"role"`
	MattermostUserID       string `json:"mattermostUserID,omitempty"`
	MattermostUsername     string `json:"mattermostUsername,omitempty"`
	Status                 string `json:"status,omitempty"`
	TemporaryPassword      string `json:"temporaryPassword,omitempty"`
	TemporaryPasswordEmail string `json:"temporaryPasswordEmail,omitempty"`
}

type mattermostUserRecord struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Roles    string `json:"roles"`
}

type mattermostTeamRecord struct {
	ID string `json:"id"`
}

type mattermostProvisionResult struct {
	UserID            string
	Username          string
	Status            string
	TemporaryPassword string
}

func (service *Service) provisionMattermostUser(ctx context.Context, email string, role string) (mattermostProvisionResult, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return mattermostProvisionResult{}, fmt.Errorf("email required")
	}

	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return mattermostProvisionResult{}, errorValue
	}

	userRecord, found, errorValue := service.findMattermostUserByEmail(ctx, adminToken, normalizedEmail)
	if errorValue != nil {
		return mattermostProvisionResult{}, errorValue
	}

	result := mattermostProvisionResult{Status: "active"}
	if found {
		result.UserID = userRecord.ID
		result.Username = userRecord.Username
	} else {
		temporaryPassword := generateTemporaryPassword()
		userRecord, errorValue = service.createMattermostUser(ctx, adminToken, normalizedEmail, temporaryPassword)
		if errorValue != nil {
			return mattermostProvisionResult{}, errorValue
		}
		result.UserID = userRecord.ID
		result.Username = userRecord.Username
		result.TemporaryPassword = temporaryPassword
	}

	if errorValue := service.ensureMattermostMembership(ctx, adminToken, result.UserID); errorValue != nil {
		return mattermostProvisionResult{}, errorValue
	}
	if errorValue := service.setMattermostRole(ctx, adminToken, result.UserID, role); errorValue != nil {
		return mattermostProvisionResult{}, errorValue
	}

	return result, nil
}

func (service *Service) mattermostAdminToken(ctx context.Context) (string, error) {
	adminPassword := strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostAdminPasswordPath))
	if adminPassword == "" {
		return "", fmt.Errorf("Mattermost admin password is not configured")
	}
	body := map[string]string{
		"login_id": "admin",
		"password": adminPassword,
	}
	requestBody, errorValue := json.Marshal(body)
	if errorValue != nil {
		return "", errorValue
	}
	requestURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + "/api/v4/users/login"
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(requestBody))
	if errorValue != nil {
		return "", errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	client := service.httpClient()
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", mattermostStatusError(response)
	}
	token := response.Header.Get("Token")
	if token == "" {
		return "", fmt.Errorf("Mattermost login did not return a token")
	}
	return token, nil
}

func (service *Service) findMattermostUserByEmail(ctx context.Context, token string, email string) (mattermostUserRecord, bool, error) {
	var userRecord mattermostUserRecord
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/email/"+url.PathEscape(email), token, nil, &userRecord)
	if errorValue == nil {
		return userRecord, true, nil
	}
	if isMattermostNotFound(errorValue) {
		return mattermostUserRecord{}, false, nil
	}
	return mattermostUserRecord{}, false, errorValue
}

func (service *Service) createMattermostUser(ctx context.Context, token string, email string, password string) (mattermostUserRecord, error) {
	usernameBase := mattermostUsernameBase(email)
	var lastError error
	for attempt := 0; attempt < 5; attempt++ {
		username := usernameBase
		if attempt > 0 {
			username = usernameBase + "-" + randomHex(2)
		}
		body := map[string]string{
			"email":    email,
			"username": username,
			"password": password,
		}
		var userRecord mattermostUserRecord
		errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/users", token, body, &userRecord)
		if errorValue == nil {
			return userRecord, nil
		}
		lastError = errorValue
		if !isMattermostConflict(errorValue) {
			break
		}
	}
	return mattermostUserRecord{}, lastError
}

func (service *Service) ensureMattermostMembership(ctx context.Context, token string, userID string) error {
	var teamRecord mattermostTeamRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/teams/name/internkim", token, nil, &teamRecord); errorValue != nil {
		return errorValue
	}
	if teamRecord.ID == "" {
		return fmt.Errorf("Mattermost team internkim is missing")
	}
	teamMember := map[string]string{
		"team_id": teamRecord.ID,
		"user_id": userID,
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/teams/"+url.PathEscape(teamRecord.ID)+"/members", token, teamMember, nil); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}

	channelID := strings.TrimSpace(readTrimmedFile(filepath.Join(filepath.Dir(service.Configuration.DeviceIDPath), "channel-id")))
	if channelID == "" {
		return nil
	}
	channelMember := map[string]string{"user_id": userID}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", token, channelMember, nil); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}
	return nil
}

func (service *Service) setMattermostRole(ctx context.Context, token string, userID string, role string) error {
	mattermostRoles := "system_user"
	if role == "admin" {
		mattermostRoles = "system_admin system_user"
	}
	body := map[string]string{"roles": mattermostRoles}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(userID)+"/roles", token, body, nil)
}

func (service *Service) mattermostRequest(ctx context.Context, method string, path string, token string, body any, responseValue any) error {
	var requestBody io.Reader
	if body != nil {
		document, errorValue := json.Marshal(body)
		if errorValue != nil {
			return errorValue
		}
		requestBody = bytes.NewReader(document)
	}
	requestURL := strings.TrimRight(service.Configuration.MattermostBaseURL, "/") + path
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
	response, errorValue := service.httpClient().Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return mattermostStatusError(response)
	}
	if responseValue == nil {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}
	return json.NewDecoder(response.Body).Decode(responseValue)
}

type mattermostAPIError struct {
	StatusCode int
	Body       string
}

func (errorValue mattermostAPIError) Error() string {
	return fmt.Sprintf("Mattermost API returned %d: %s", errorValue.StatusCode, errorValue.Body)
}

func mattermostStatusError(response *http.Response) error {
	document, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	return mattermostAPIError{StatusCode: response.StatusCode, Body: strings.TrimSpace(string(document))}
}

func isMattermostNotFound(errorValue error) bool {
	var apiError mattermostAPIError
	return errors.As(errorValue, &apiError) && apiError.StatusCode == http.StatusNotFound
}

func isMattermostConflict(errorValue error) bool {
	var apiError mattermostAPIError
	return errors.As(errorValue, &apiError) && apiError.StatusCode == http.StatusBadRequest
}

func isMattermostBadRequest(errorValue error) bool {
	var apiError mattermostAPIError
	return errors.As(errorValue, &apiError) && apiError.StatusCode == http.StatusBadRequest
}

func mattermostUsernameBase(email string) string {
	localPart := strings.Split(email, "@")[0]
	var builder strings.Builder
	for _, character := range strings.ToLower(localPart) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.' {
			builder.WriteRune(character)
			continue
		}
		builder.WriteByte('-')
	}
	username := strings.Trim(builder.String(), "-_.")
	if len(username) < 3 {
		username = "user-" + randomHex(3)
	}
	if len(username) > 48 {
		username = username[:48]
	}
	return username
}

func generateTemporaryPassword() string {
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789!@#$%+=?"
	randomValue := randomBytes(24)
	var builder strings.Builder
	for _, value := range randomValue {
		builder.WriteByte(alphabet[int(value)%len(alphabet)])
	}
	return builder.String()
}
