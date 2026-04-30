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
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/anthropic-lab/internkim/internal/identity"
)

type adminUserMutation struct {
	UserID                 string `json:"userID,omitempty"`
	Handle                 string `json:"handle,omitempty"`
	Name                   string `json:"name,omitempty"`
	Email                  string `json:"email"`
	Role                   string `json:"role"`
	MattermostUserID       string `json:"mattermostUserID,omitempty"`
	MattermostUsername     string `json:"mattermostUsername,omitempty"`
	Status                 string `json:"status,omitempty"`
	TemporaryPassword      string `json:"temporaryPassword,omitempty"`
	TemporaryPasswordEmail string `json:"temporaryPasswordEmail,omitempty"`
}

type mattermostUserRecord struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	FirstName   string `json:"first_name"`
	Nickname    string `json:"nickname"`
	Position    string `json:"position"`
	Roles       string `json:"roles"`
	DeleteAt    int64  `json:"delete_at"`
}

type mattermostTeamRecord struct {
	ID string `json:"id"`
}

type mattermostChannelRecord struct {
	ID string `json:"id"`
}

type mattermostProvisionResult struct {
	UserID            string
	Username          string
	Status            string
	TemporaryPassword string
}

type mattermostPreferenceRecord struct {
	UserID   string `json:"user_id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

const mattermostProvisionerUsername = "admin"
const mattermostProvisionerEmail = "admin@localhost"
const firstAdminMattermostPassword = "admin"

func (service *Service) provisionMattermostUser(ctx context.Context, email string, role string) (mattermostProvisionResult, error) {
	return service.provisionMattermostUserWithPassword(ctx, adminUserMutation{Email: email, Role: role}, "")
}

func (service *Service) provisionMattermostUserWithPassword(ctx context.Context, user adminUserMutation, initialPassword string) (mattermostProvisionResult, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(user.Email))
	if normalizedEmail == "" {
		return mattermostProvisionResult{}, fmt.Errorf("email required")
	}
	normalizedHandle := normalizeMattermostHandle(firstNonEmpty(user.Handle, user.MattermostUsername, mattermostUsernameBase(normalizedEmail)))
	if !isValidMattermostHandle(normalizedHandle) {
		return mattermostProvisionResult{}, fmt.Errorf("handle must start with a letter and contain 3-22 lowercase letters, numbers, dots, dashes, or underscores")
	}
	displayName := strings.TrimSpace(user.Name)

	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return mattermostProvisionResult{}, errorValue
	}
	if strings.TrimSpace(initialPassword) != "" {
		if errorValue := service.ensureMattermostPasswordPolicyAllows(ctx, adminToken, initialPassword); errorValue != nil {
			return mattermostProvisionResult{}, errorValue
		}
	} else if errorValue := service.ensureMattermostFullNameDisplay(ctx, adminToken); errorValue != nil {
		return mattermostProvisionResult{}, errorValue
	}

	userRecord, found, errorValue := service.mattermostUserRecordForAdminRecord(ctx, adminToken, adminUserMutation{
		Email:            normalizedEmail,
		MattermostUserID: user.MattermostUserID,
	})
	if errorValue != nil {
		return mattermostProvisionResult{}, errorValue
	}

	result := mattermostProvisionResult{Status: "active"}
	if found {
		if userRecord.Username != normalizedHandle || displayName != "" {
			userRecord, errorValue = service.updateMattermostUserIdentity(ctx, adminToken, userRecord.ID, normalizedHandle, displayName)
			if errorValue != nil {
				return mattermostProvisionResult{}, errorValue
			}
		}
		result.UserID = userRecord.ID
		result.Username = userRecord.Username
		if strings.TrimSpace(initialPassword) != "" {
			if errorValue := service.updateMattermostUserPassword(ctx, adminToken, userRecord.ID, initialPassword); errorValue != nil {
				return mattermostProvisionResult{}, errorValue
			}
			result.TemporaryPassword = initialPassword
		}
	} else {
		temporaryPassword := firstNonEmpty(strings.TrimSpace(initialPassword), generateTemporaryPassword())
		userRecord, errorValue = service.createMattermostUser(ctx, adminToken, normalizedEmail, normalizedHandle, displayName, temporaryPassword)
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
	if errorValue := service.setMattermostRole(ctx, adminToken, result.UserID, user.Role); errorValue != nil {
		return mattermostProvisionResult{}, errorValue
	}
	if errorValue := service.ensureMattermostBotDirectChannel(ctx, adminToken, result.UserID); errorValue != nil {
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
		"login_id": mattermostProvisionerUsername,
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

func (service *Service) findMattermostUserByUsername(ctx context.Context, token string, username string) (mattermostUserRecord, bool, error) {
	var userRecord mattermostUserRecord
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/username/"+url.PathEscape(username), token, nil, &userRecord)
	if errorValue == nil {
		return userRecord, true, nil
	}
	if isMattermostNotFound(errorValue) {
		return mattermostUserRecord{}, false, nil
	}
	return mattermostUserRecord{}, false, errorValue
}

func (service *Service) findMattermostUserByID(ctx context.Context, token string, userID string) (mattermostUserRecord, bool, error) {
	var userRecord mattermostUserRecord
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/"+url.PathEscape(userID), token, nil, &userRecord)
	if errorValue == nil {
		return userRecord, true, nil
	}
	if isMattermostNotFound(errorValue) {
		return mattermostUserRecord{}, false, nil
	}
	return mattermostUserRecord{}, false, errorValue
}

func (service *Service) ensureMattermostProvisionerAccount(ctx context.Context) error {
	adminPassword := strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostAdminPasswordPath))
	if adminPassword == "" {
		return fmt.Errorf("Mattermost admin password is not configured")
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	adminUser, found, errorValue := service.findMattermostUserByUsername(ctx, adminToken, mattermostProvisionerUsername)
	if errorValue != nil || !found {
		return errorValue
	}
	if !strings.EqualFold(adminUser.Email, mattermostProvisionerEmail) {
		body := map[string]string{
			"email":    mattermostProvisionerEmail,
			"password": adminPassword,
		}
		if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(adminUser.ID)+"/patch", adminToken, body, nil); errorValue != nil {
			return errorValue
		}
	}
	return service.setMattermostRole(ctx, adminToken, adminUser.ID, "admin")
}

func (service *Service) deactivateMattermostUserByID(ctx context.Context, userID string) error {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	userRecord, found, errorValue := service.findMattermostUserByID(ctx, adminToken, normalizedUserID)
	if errorValue != nil || !found {
		return errorValue
	}
	if isProtectedMattermostUser(userRecord) {
		return nil
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/users/"+url.PathEscape(userRecord.ID), adminToken, nil, nil); errorValue != nil {
		return errorValue
	}
	return service.deleteMattermostVisibleUserSystemPosts(ctx, userRecord)
}

func (service *Service) deactivateMattermostUserByEmail(ctx context.Context, email string) error {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" {
		return nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	userRecord, found, errorValue := service.findMattermostUserByEmail(ctx, adminToken, normalizedEmail)
	if errorValue != nil || !found {
		return errorValue
	}
	if isProtectedMattermostUser(userRecord) {
		return nil
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/users/"+url.PathEscape(userRecord.ID), adminToken, nil, nil); errorValue != nil {
		return errorValue
	}
	return service.deleteMattermostVisibleUserSystemPosts(ctx, userRecord)
}

func (service *Service) deleteMattermostVisibleUserSystemPosts(ctx context.Context, userRecord mattermostUserRecord) error {
	username := strings.TrimSpace(userRecord.Username)
	if username == "" {
		return nil
	}
	command := fmt.Sprintf(
		`su postgres -c %s`,
		quoteShellValue(fmt.Sprintf(
			`psql -d mattermost -v ON_ERROR_STOP=1 -c %s`,
			quoteShellValue(fmt.Sprintf(
				`UPDATE posts SET deleteat = (extract(epoch from now()) * 1000)::bigint, updateat = (extract(epoch from now()) * 1000)::bigint WHERE deleteat = 0 AND (message LIKE %s OR props::text LIKE %s)`,
				quoteSQLLikePattern("%"+username+"%"),
				quoteSQLLikePattern("%"+username+"%"),
			)),
		)),
	)
	_, errorValue := service.runCommand(ctx, "sh", "-c", command)
	return errorValue
}

func quoteShellValue(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func quoteSQLLikePattern(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func isProtectedMattermostUser(userRecord mattermostUserRecord) bool {
	username := strings.ToLower(strings.TrimSpace(userRecord.Username))
	return username == mattermostProvisionerUsername || username == "internkim" || username == "system-bot"
}

func (service *Service) createMattermostUser(ctx context.Context, token string, email string, handle string, name string, password string) (mattermostUserRecord, error) {
	body := map[string]string{
		"email":    email,
		"username": handle,
		"password": password,
	}
	addMattermostNameFields(body, name)
	var userRecord mattermostUserRecord
	errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/users", token, body, &userRecord)
	if errorValue == nil {
		return userRecord, nil
	}
	return mattermostUserRecord{}, errorValue
}

func (service *Service) updateMattermostUserIdentity(ctx context.Context, token string, userID string, handle string, name string) (mattermostUserRecord, error) {
	body := map[string]string{"username": handle}
	addMattermostNameFields(body, name)
	var userRecord mattermostUserRecord
	errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(userID)+"/patch", token, body, &userRecord)
	if errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	if userRecord.ID == "" {
		return service.findMattermostUserByIDRequired(ctx, token, userID)
	}
	return userRecord, nil
}

func (service *Service) findMattermostUserByIDRequired(ctx context.Context, token string, userID string) (mattermostUserRecord, error) {
	userRecord, found, errorValue := service.findMattermostUserByID(ctx, token, userID)
	if errorValue != nil {
		return mattermostUserRecord{}, errorValue
	}
	if !found {
		return mattermostUserRecord{}, fmt.Errorf("Mattermost user %s was not found after patch", userID)
	}
	return userRecord, nil
}

func addMattermostNameFields(body map[string]string, name string) {
	canonicalName := strings.TrimSpace(name)
	if canonicalName == "" {
		return
	}
	body["nickname"] = canonicalName
	firstName, lastName := identity.SplitNameForMattermost(canonicalName)
	if firstName != "" {
		body["first_name"] = firstName
	}
	if lastName != "" {
		body["last_name"] = lastName
	}
}

func (service *Service) updateMattermostUserPassword(ctx context.Context, token string, userID string, password string) error {
	body := map[string]string{"new_password": password}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(userID)+"/password", token, body, nil)
}

func (service *Service) ensureMattermostPasswordPolicyAllows(ctx context.Context, token string, password string) error {
	minimumLength := len([]rune(password))
	if minimumLength < 5 {
		minimumLength = 5
	}
	body := map[string]any{
		"PasswordSettings": map[string]any{
			"MinimumLength": minimumLength,
			"Lowercase":     false,
			"Uppercase":     false,
			"Number":        false,
			"Symbol":        false,
		},
		"TeamSettings": map[string]any{
			"TeammateNameDisplay": "full_name",
		},
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/config/patch", token, body, nil)
}

func (service *Service) ensureMattermostFullNameDisplay(ctx context.Context, token string) error {
	body := map[string]any{
		"TeamSettings": map[string]any{
			"TeammateNameDisplay": "full_name",
		},
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/config/patch", token, body, nil)
}

func (service *Service) ensureMattermostMembership(ctx context.Context, token string, userID string) error {
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	teamMember := map[string]string{
		"team_id": teamRecord.ID,
		"user_id": userID,
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/teams/"+url.PathEscape(teamRecord.ID)+"/members", token, teamMember, nil); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}

	channelID, errorValue := service.ensureMattermostTownSquareChannel(ctx, token, teamRecord.ID)
	if errorValue != nil {
		return errorValue
	}
	if channelID == "" {
		return nil
	}
	channelMember := map[string]string{"user_id": userID}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", token, channelMember, nil); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}
	return nil
}

func (service *Service) ensureMattermostTeam(ctx context.Context, token string) (mattermostTeamRecord, error) {
	var teamRecord mattermostTeamRecord
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/teams/name/internkim", token, nil, &teamRecord)
	if errorValue == nil && teamRecord.ID != "" {
		return teamRecord, nil
	}
	if errorValue != nil && !isMattermostNotFound(errorValue) {
		return mattermostTeamRecord{}, errorValue
	}

	body := map[string]string{"name": "internkim", "display_name": "Intern Kim", "type": "I"}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/teams", token, body, &teamRecord); errorValue != nil {
		return mattermostTeamRecord{}, errorValue
	}
	if teamRecord.ID == "" {
		return mattermostTeamRecord{}, fmt.Errorf("Mattermost team internkim was not created")
	}
	return teamRecord, nil
}

func (service *Service) ensureMattermostTownSquareChannel(ctx context.Context, token string, teamID string) (string, error) {
	channelIDPath := filepath.Join(filepath.Dir(service.Configuration.DeviceIDPath), "channel-id")
	channelID := strings.TrimSpace(readTrimmedFile(channelIDPath))
	if channelID != "" {
		var channelRecord mattermostChannelRecord
		errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/channels/"+url.PathEscape(channelID), token, nil, &channelRecord)
		if errorValue == nil && channelRecord.ID != "" {
			return channelRecord.ID, nil
		}
		if errorValue != nil && !isMattermostNotFound(errorValue) {
			return "", errorValue
		}
	}

	var channelRecord mattermostChannelRecord
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/teams/"+url.PathEscape(teamID)+"/channels/name/town-square", token, nil, &channelRecord)
	if errorValue != nil && !isMattermostNotFound(errorValue) {
		return "", errorValue
	}
	if channelRecord.ID == "" {
		body := map[string]string{"team_id": teamID, "name": "town-square", "display_name": "Town Square", "type": "O"}
		if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels", token, body, &channelRecord); errorValue != nil {
			return "", errorValue
		}
	}
	if channelRecord.ID == "" {
		return "", fmt.Errorf("Mattermost channel town-square was not created")
	}
	if errorValue := os.WriteFile(channelIDPath, []byte(channelRecord.ID), 0o640); errorValue != nil {
		return "", errorValue
	}
	return channelRecord.ID, nil
}

func (service *Service) ensureMattermostBotDirectChannel(ctx context.Context, token string, userID string) error {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil
	}
	botRecord, found, errorValue := service.findMattermostUserByUsername(ctx, token, "internkim")
	if errorValue != nil {
		return errorValue
	}
	if !found || botRecord.ID == "" || botRecord.DeleteAt != 0 || botRecord.ID == normalizedUserID {
		return nil
	}
	body := []string{normalizedUserID, botRecord.ID}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/direct", token, body, nil); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}
	return service.showMattermostDirectChannel(ctx, token, normalizedUserID, botRecord.ID)
}

func (service *Service) showMattermostDirectChannel(ctx context.Context, token string, userID string, directUserID string) error {
	preferences := []mattermostPreferenceRecord{{
		UserID:   userID,
		Category: "direct_channel_show",
		Name:     directUserID,
		Value:    "true",
	}}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(userID)+"/preferences", token, preferences, nil)
}

func (service *Service) ensureMattermostBotDirectChannelsForRecords(ctx context.Context, records []adminUserMutation) error {
	if len(records) == 0 || strings.TrimSpace(service.Configuration.MattermostAdminPasswordPath) == "" {
		return nil
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	for _, record := range records {
		userRecord, found, errorValue := service.mattermostUserRecordForAdminRecord(ctx, adminToken, record)
		if errorValue != nil {
			return errorValue
		}
		if !found || userRecord.DeleteAt != 0 || isProtectedMattermostUser(userRecord) {
			continue
		}
		if errorValue := service.ensureMattermostBotDirectChannel(ctx, adminToken, userRecord.ID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) mattermostUserRecordForAdminRecord(ctx context.Context, token string, record adminUserMutation) (mattermostUserRecord, bool, error) {
	if strings.TrimSpace(record.MattermostUserID) != "" {
		return service.findMattermostUserByID(ctx, token, record.MattermostUserID)
	}
	if strings.TrimSpace(record.Email) != "" {
		return service.findMattermostUserByEmail(ctx, token, record.Email)
	}
	return mattermostUserRecord{}, false, nil
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
	if len(username) < 3 || !isLowercaseASCIIAlpha(rune(username[0])) {
		username = "user-" + randomHex(3)
	}
	if len(username) > 22 {
		username = username[:22]
	}
	return username
}

func normalizeMattermostHandle(handle string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(handle)) {
		if isLowercaseASCIIAlpha(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.' {
			builder.WriteRune(character)
		}
	}
	return strings.Trim(builder.String(), "-_.")
}

func isValidMattermostHandle(handle string) bool {
	if len(handle) < 3 || len(handle) > 22 {
		return false
	}
	if !isLowercaseASCIIAlpha(rune(handle[0])) {
		return false
	}
	for _, character := range handle {
		if isLowercaseASCIIAlpha(character) || unicode.IsDigit(character) || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func isLowercaseASCIIAlpha(character rune) bool {
	return character >= 'a' && character <= 'z'
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
