package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"gitlab.com/eastriver/internkim/internal/identity"
	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

type mattermostUserRecord struct {
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
}

type mattermostTeamRecord struct {
	ID string `json:"id"`
}

type mattermostChannelRecord struct {
	ID string `json:"id"`
}

type mattermostChannelMemberRecord struct {
	UserID string `json:"user_id"`
}

type adminCircleRecord struct {
	CircleID            string `json:"circleID"`
	DisplayName         string `json:"displayName"`
	IsMattermostManaged bool   `json:"isMattermostManaged,omitempty"`
}

type mattermostPostRecord struct {
	ID        string         `json:"id"`
	UserID    string         `json:"user_id"`
	ChannelID string         `json:"channel_id"`
	RootID    string         `json:"root_id"`
	Message   string         `json:"message"`
	Type      string         `json:"type"`
	CreateAt  int64          `json:"create_at"`
	DeleteAt  int64          `json:"delete_at"`
	IsPinned  bool           `json:"is_pinned"`
	Props     map[string]any `json:"props"`
}

type mattermostPostsResponse struct {
	Order []string                        `json:"order"`
	Posts map[string]mattermostPostRecord `json:"posts"`
}

type mattermostProvisionResult struct {
	UserID            string
	Username          string
	Status            string
	TemporaryPassword string
}

type mattermostPasswordResetResult struct {
	TemporaryPassword string
	DeletedPostCount  int
}

type mattermostPreferenceRecord struct {
	UserID   string `json:"user_id"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Value    string `json:"value"`
}

type mattermostDefaultChannelProvision struct {
	Name   string
	Ensure func(context.Context, string, string) (string, error)
}

const mattermostProvisionerUsername = "admin"
const mattermostProvisionerEmail = "admin@localhost"
const firstAdminMattermostPassword = "admin"
const mattermostTeammateNameDisplay = "nickname_full_name"
const mattermostFlowChannelName = mattermostdefaults.FlowChannelName
const mattermostFlowChannelDisplayName = mattermostdefaults.FlowChannelDisplayName
const mattermostCalendarChannelName = mattermostdefaults.CalendarChannelName
const mattermostCalendarChannelDisplayName = mattermostdefaults.CalendarChannelDisplayName

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
	} else if errorValue := service.ensureMattermostNicknameDisplay(ctx, adminToken); errorValue != nil {
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

func (service *Service) mattermostBotToken() (string, error) {
	token := strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostBotTokenPath))
	if token == "" {
		token = strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostTokenPath))
	}
	if token == "" {
		return "", fmt.Errorf("Mattermost bot token is not configured")
	}
	return token, nil
}

func (service *Service) mattermostTokenUserID(ctx context.Context, token string) (string, error) {
	var userRecord mattermostUserRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users/me", token, nil, &userRecord); errorValue != nil {
		return "", errorValue
	}
	if strings.TrimSpace(userRecord.ID) == "" {
		return "", fmt.Errorf("Mattermost token user ID is missing")
	}
	return strings.TrimSpace(userRecord.ID), nil
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
	adminToken, _, errorValue := service.ensureMattermostProvisionerIdentity(ctx)
	if errorValue != nil {
		return errorValue
	}
	return service.ensureMattermostConnectCommand(ctx, adminToken)
}

func (service *Service) ensureMattermostProvisionerDefaults(ctx context.Context) error {
	adminToken, adminUser, errorValue := service.ensureMattermostProvisionerIdentity(ctx)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := service.ensureMattermostRuntimeSettings(ctx, adminToken); errorValue != nil {
		return errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, adminToken)
	if errorValue != nil {
		return errorValue
	}
	channelIDs, channelError := service.ensureMattermostDefaultChannelIDs(ctx, adminToken, teamRecord.ID)
	adminMembershipError := service.ensureMattermostDefaultChannelMembership(ctx, adminToken, teamRecord.ID, adminUser.ID, channelIDs)
	botMembershipError := service.ensureMattermostBotDefaultChannelMembership(ctx, adminToken, teamRecord.ID, channelIDs)
	userMembershipError := service.ensureMattermostDefaultChannelMemberships(ctx, adminToken, teamRecord.ID, channelIDs)
	connectCommandError := service.ensureMattermostConnectCommand(ctx, adminToken)
	oauthAppError := service.ensureMattermostWebOAuthApp(ctx, adminToken)
	botPermissionError := service.ensureMattermostBotEphemeralPermission(ctx, adminToken)
	return errors.Join(channelError, adminMembershipError, botMembershipError, userMembershipError, connectCommandError, oauthAppError, botPermissionError)
}

func (service *Service) ensureMattermostProvisionerIdentity(ctx context.Context) (string, mattermostUserRecord, error) {
	adminPassword := strings.TrimSpace(readTrimmedFile(service.Configuration.MattermostAdminPasswordPath))
	if adminPassword == "" {
		return "", mattermostUserRecord{}, fmt.Errorf("Mattermost admin password is not configured")
	}
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return "", mattermostUserRecord{}, errorValue
	}
	adminUser, found, errorValue := service.findMattermostUserByUsername(ctx, adminToken, mattermostProvisionerUsername)
	if errorValue != nil {
		return "", mattermostUserRecord{}, errorValue
	}
	if !found {
		return "", mattermostUserRecord{}, fmt.Errorf("Mattermost admin user was not found")
	}
	if !strings.EqualFold(adminUser.Email, mattermostProvisionerEmail) {
		body := map[string]string{
			"email":    mattermostProvisionerEmail,
			"password": adminPassword,
		}
		if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(adminUser.ID)+"/patch", adminToken, body, nil); errorValue != nil {
			return "", mattermostUserRecord{}, errorValue
		}
	}
	if errorValue := service.setMattermostRole(ctx, adminToken, adminUser.ID, "admin"); errorValue != nil {
		return "", mattermostUserRecord{}, errorValue
	}
	return adminToken, adminUser, nil
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
	if errorValue := service.deleteMattermostVisibleUserSystemPosts(ctx, userRecord); errorValue != nil {
		if service.hasDeviceAuth() {
			return errorValue
		}
		log.Printf("Mattermost system post cleanup skipped without local database access: %v", errorValue)
	}
	return nil
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
	if errorValue := service.deleteMattermostVisibleUserSystemPosts(ctx, userRecord); errorValue != nil {
		if service.hasDeviceAuth() {
			return errorValue
		}
		log.Printf("Mattermost system post cleanup skipped without local database access: %v", errorValue)
	}
	return nil
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
	body["nickname"] = identity.NicknameForMattermost(canonicalName)
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

func (service *Service) resetMattermostUserPasswordAndHistory(ctx context.Context, record adminUserMutation) (mattermostPasswordResetResult, error) {
	adminToken, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return mattermostPasswordResetResult{}, errorValue
	}
	userRecord, found, errorValue := service.mattermostUserRecordForAdminRecord(ctx, adminToken, record)
	if errorValue != nil {
		return mattermostPasswordResetResult{}, errorValue
	}
	if !found {
		return mattermostPasswordResetResult{}, fmt.Errorf("Mattermost user was not found")
	}
	if isProtectedMattermostUser(userRecord) {
		return mattermostPasswordResetResult{}, fmt.Errorf("protected Mattermost user cannot be reset")
	}
	channelID, errorValue := service.ensureMattermostBotDirectChannelID(ctx, adminToken, userRecord.ID)
	if errorValue != nil {
		return mattermostPasswordResetResult{}, errorValue
	}
	deletedPostCount, errorValue := service.deleteMattermostDirectChannelPosts(ctx, adminToken, channelID)
	if errorValue != nil {
		return mattermostPasswordResetResult{}, errorValue
	}
	if errorValue := service.deleteBlueclawDirectConversationHistory(ctx, channelID); errorValue != nil {
		return mattermostPasswordResetResult{}, errorValue
	}
	temporaryPassword := generateTemporaryPassword()
	if errorValue := service.ensureMattermostPasswordPolicyAllows(ctx, adminToken, temporaryPassword); errorValue != nil {
		return mattermostPasswordResetResult{}, errorValue
	}
	if errorValue := service.updateMattermostUserPassword(ctx, adminToken, userRecord.ID, temporaryPassword); errorValue != nil {
		return mattermostPasswordResetResult{}, errorValue
	}
	return mattermostPasswordResetResult{TemporaryPassword: temporaryPassword, DeletedPostCount: deletedPostCount}, nil
}

func (service *Service) deleteMattermostDirectChannelPosts(ctx context.Context, token string, channelID string) (int, error) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return 0, nil
	}
	deletedPostIDs := map[string]bool{}
	for page := 0; page < 100; page++ {
		response, errorValue := service.mattermostChannelPosts(ctx, token, normalizedChannelID, page)
		if errorValue != nil {
			return 0, errorValue
		}
		if len(response.Order) == 0 {
			return len(deletedPostIDs), nil
		}
		for _, postID := range response.Order {
			trimmedPostID := strings.TrimSpace(postID)
			if trimmedPostID == "" || deletedPostIDs[trimmedPostID] {
				continue
			}
			if errorValue := service.deleteMattermostPost(ctx, token, trimmedPostID); errorValue != nil {
				return 0, errorValue
			}
			deletedPostIDs[trimmedPostID] = true
		}
	}
	return len(deletedPostIDs), fmt.Errorf("Mattermost direct channel has more history than one reset can delete")
}

func (service *Service) mattermostChannelPosts(ctx context.Context, token string, channelID string, page int) (mattermostPostsResponse, error) {
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?page=" + fmt.Sprint(page) + "&per_page=200"
	var response mattermostPostsResponse
	errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &response)
	return response, errorValue
}

func (service *Service) deleteMattermostPost(ctx context.Context, token string, postID string) error {
	trimmedPostID := strings.TrimSpace(postID)
	if trimmedPostID == "" {
		return nil
	}
	errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(trimmedPostID), token, nil, nil)
	if errorValue != nil && !isMattermostNotFound(errorValue) {
		return errorValue
	}
	return nil
}

func (service *Service) deleteBlueclawDirectConversationHistory(ctx context.Context, channelID string) error {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	body := map[string]string{"channelID": normalizedChannelID}
	return service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/conversation/reset", body, nil)
}

func (service *Service) ensureMattermostPasswordPolicyAllows(ctx context.Context, token string, password string) error {
	minimumLength := len([]rune(password))
	if minimumLength < 5 {
		minimumLength = 5
	}
	body := map[string]any{
		"ServiceSettings": service.mattermostServiceSettingsPatch(),
		"PasswordSettings": map[string]any{
			"MinimumLength": minimumLength,
			"Lowercase":     false,
			"Uppercase":     false,
			"Number":        false,
			"Symbol":        false,
		},
		"TeamSettings": map[string]any{
			"TeammateNameDisplay": mattermostTeammateNameDisplay,
		},
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/config/patch", token, body, nil)
}

func (service *Service) ensureMattermostNicknameDisplay(ctx context.Context, token string) error {
	body := map[string]any{
		"ServiceSettings": service.mattermostServiceSettingsPatch(),
		"TeamSettings": map[string]any{
			"TeammateNameDisplay": mattermostTeammateNameDisplay,
		},
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/config/patch", token, body, nil)
}

func (service *Service) ensureMattermostRuntimeSettings(ctx context.Context, token string) error {
	body := map[string]any{
		"ServiceSettings": service.mattermostServiceSettingsPatch(),
		"TeamSettings": map[string]any{
			"TeammateNameDisplay": mattermostTeammateNameDisplay,
		},
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/config/patch", token, body, nil)
}

func (service *Service) mattermostServiceSettingsPatch() map[string]any {
	settings := map[string]any{
		"AllowedUntrustedInternalConnections": "127.0.0.1 localhost",
		"EnableBotAccountCreation":            true,
		"EnableOAuthServiceProvider":          true,
		"EnableUserAccessTokens":              true,
		"ManagedResourcePaths":                mattermostdefaults.ManagedResourcePathSetting(),
	}
	if siteURL := strings.TrimRight(strings.TrimSpace(service.mattermostFlowBaseURL()), "/"); siteURL != "" {
		settings["SiteURL"] = siteURL
		settings["AllowCorsFrom"] = siteURL
		settings["CorsAllowCredentials"] = true
	}
	return settings
}

func (service *Service) ensureMattermostMembership(ctx context.Context, token string, userID string) error {
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	channelIDs, channelError := service.ensureMattermostDefaultChannelIDs(ctx, token, teamRecord.ID)
	membershipError := service.ensureMattermostDefaultChannelMembership(ctx, token, teamRecord.ID, userID, channelIDs)
	return errors.Join(channelError, membershipError)
}

func (service *Service) ensureMattermostDefaultChannelMemberships(ctx context.Context, token string, teamID string, channelIDs []string) error {
	users, errorValue := service.channelMembershipUsers(ctx, token, teamID)
	if errorValue != nil {
		return errorValue
	}
	var membershipErrors []error
	for _, user := range users {
		membershipErrors = append(membershipErrors, service.ensureMattermostDefaultChannelMembership(ctx, token, teamID, user.ID, channelIDs))
	}
	return errors.Join(membershipErrors...)
}

func (service *Service) channelMembershipUsers(ctx context.Context, token string, teamID string) ([]mattermostUserRecord, error) {
	if !service.hasDeviceAuth() {
		return service.teamMattermostUsers(ctx, token, teamID)
	}
	return service.allowedMattermostUsers(ctx, token)
}

func (service *Service) teamMattermostUsers(ctx context.Context, token string, teamID string) ([]mattermostUserRecord, error) {
	var users []mattermostUserRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users?in_team="+url.PathEscape(teamID)+"&per_page=200", token, nil, &users); errorValue != nil {
		return nil, errorValue
	}
	teamUsers := make([]mattermostUserRecord, 0, len(users))
	for _, user := range users {
		if user.DeleteAt != 0 || isProtectedMattermostUser(user) {
			continue
		}
		teamUsers = append(teamUsers, user)
	}
	return teamUsers, nil
}

func (service *Service) allowedMattermostUsers(ctx context.Context, token string) ([]mattermostUserRecord, error) {
	if !service.hasDeviceAuth() {
		return service.activeMattermostUsers(ctx, token)
	}
	records, errorValue := service.currentUserRecords(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	users := make([]mattermostUserRecord, 0, len(records))
	for _, record := range records {
		if !isActiveFlowUser(record) {
			continue
		}
		userRecord, found, errorValue := service.mattermostUserRecordForAdminRecord(ctx, token, record)
		if errorValue != nil {
			return nil, errorValue
		}
		if !found || userRecord.DeleteAt != 0 || isProtectedMattermostUser(userRecord) {
			continue
		}
		users = append(users, userRecord)
	}
	return users, nil
}

func (service *Service) ensureMattermostDefaultChannelMembership(ctx context.Context, token string, teamID string, userID string, channelIDs []string) error {
	if strings.TrimSpace(userID) == "" {
		return nil
	}
	teamMember := map[string]string{
		"team_id": teamID,
		"user_id": userID,
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/teams/"+url.PathEscape(teamID)+"/members", token, teamMember, nil); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}
	var membershipErrors []error
	for _, channelID := range channelIDs {
		if channelID == "" {
			continue
		}
		if errorValue := service.ensureMattermostChannelMembership(ctx, token, channelID, userID); errorValue != nil {
			membershipErrors = append(membershipErrors, errorValue)
		}
	}
	_ = service.cleanupSavedMattermostManagedChannelSystemPosts(ctx, token)
	return errors.Join(membershipErrors...)
}

func (service *Service) ensureMattermostBotDefaultChannelMembership(ctx context.Context, token string, teamID string, channelIDs []string) error {
	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return nil
	}
	botUserID, errorValue := service.mattermostTokenUserID(ctx, botToken)
	if errorValue != nil {
		return errorValue
	}
	return service.ensureMattermostDefaultChannelMembership(ctx, token, teamID, botUserID, channelIDs)
}

func (service *Service) ensureMattermostDefaultChannelIDs(ctx context.Context, token string, teamID string) ([]string, error) {
	var channelIDs []string
	var channelErrors []error
	for _, channelProvision := range service.mattermostDefaultChannelProvisions() {
		channelID, errorValue := channelProvision.Ensure(ctx, token, teamID)
		if errorValue != nil {
			channelErrors = append(channelErrors, fmt.Errorf("%s channel: %w", channelProvision.Name, errorValue))
			continue
		}
		channelIDs = append(channelIDs, channelID)
	}
	service.syncExistingMattermostManagedPosts(ctx)
	return uniqueNonEmpty(channelIDs), errors.Join(channelErrors...)
}

func (service *Service) mattermostDefaultChannelProvisions() []mattermostDefaultChannelProvision {
	return []mattermostDefaultChannelProvision{
		{Name: mattermostdefaults.TownSquareChannelName, Ensure: service.ensureMattermostTownSquareChannel},
		{Name: mattermostdefaults.OffTopicChannelName, Ensure: service.ensureMattermostOffTopicChannel},
		{Name: mattermostFlowChannelName, Ensure: service.ensureMattermostFlowChannel},
		{Name: mattermostCalendarChannelName, Ensure: service.ensureMattermostCalendarChannel},
		{Name: attendanceChannelName, Ensure: service.ensureMattermostAttendanceChannel},
	}
}

func (service *Service) activeMattermostUsers(ctx context.Context, token string) ([]mattermostUserRecord, error) {
	var users []mattermostUserRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/users?per_page=200", token, nil, &users); errorValue != nil {
		return nil, errorValue
	}
	activeUsers := make([]mattermostUserRecord, 0, len(users))
	for _, user := range users {
		if user.DeleteAt != 0 || isProtectedMattermostUser(user) {
			continue
		}
		activeUsers = append(activeUsers, user)
	}
	return activeUsers, nil
}

func (service *Service) ensureMattermostCircleChannels(ctx context.Context, token string) error {
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	circleChannels, errorValue := service.mattermostCircleChannelDefinitions(ctx)
	if errorValue != nil {
		return errorValue
	}
	for _, circleChannel := range circleChannels {
		if _, errorValue := service.ensureMattermostPrivateChannel(ctx, token, teamRecord.ID, circleChannel.ChannelName); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) ensureMattermostPrivateChannel(ctx context.Context, token string, teamID string, channelName string) (string, error) {
	channelID, errorValue := service.mattermostChannelIDByName(ctx, token, teamID, channelName)
	if errorValue == nil || channelID != "" {
		if channelID != "" {
			if updateError := service.updateMattermostPrivateChannelDisplayName(ctx, token, channelID, channelName); updateError != nil {
				return "", updateError
			}
		}
		return channelID, errorValue
	}
	if !isMattermostNotFound(errorValue) {
		return "", errorValue
	}
	body := map[string]string{
		"team_id":      teamID,
		"name":         channelName,
		"display_name": mattermostdefaults.CircleChannelDisplayName(channelName),
		"type":         "P",
	}
	var channelRecord mattermostChannelRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels", token, body, &channelRecord); errorValue != nil {
		return "", errorValue
	}
	return channelRecord.ID, nil
}

func (service *Service) updateMattermostPrivateChannelDisplayName(ctx context.Context, token string, channelID string, channelName string) error {
	body := map[string]string{"display_name": mattermostdefaults.CircleChannelDisplayName(channelName)}
	if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", token, body, nil); errorValue != nil {
		return errorValue
	}
	return service.cleanupMattermostManagedChannelSystemPosts(ctx, token, channelID)
}

func (service *Service) ensureMattermostFlowChannel(ctx context.Context, token string, teamID string) (string, error) {
	channel, _ := service.mattermostManagedPublicChannel(mattermostFlowChannelName)
	channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamID, channel.Name, channel.DisplayName)
	if errorValue != nil {
		return "", errorValue
	}
	service.saveMattermostFlowChannelID(channelID)
	if errorValue := service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.ensureMattermostManagedChannelModeration(ctx, token, channelID); errorValue != nil {
		return "", errorValue
	}
	service.syncMattermostFlowEntryPost(ctx, token, channelID)
	return channelID, nil
}

func (service *Service) ensureMattermostCalendarChannel(ctx context.Context, token string, teamID string) (string, error) {
	channel, _ := service.mattermostManagedPublicChannel(mattermostCalendarChannelName)
	channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamID, channel.Name, channel.DisplayName)
	if errorValue != nil {
		return "", errorValue
	}
	service.saveMattermostCalendarChannelID(channelID)
	if errorValue := service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.ensureMattermostManagedChannelModeration(ctx, token, channelID); errorValue != nil {
		return "", errorValue
	}
	return channelID, nil
}

func (service *Service) saveMattermostCalendarChannelID(channelID string) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return
	}
	path := service.mattermostCalendarChannelIDPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return
	}
	_ = os.WriteFile(path, []byte(normalizedChannelID), 0o600)
}

func (service *Service) mattermostCalendarChannelIDPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "mattermost-calendar-channel-id")
}

func (service *Service) saveMattermostFlowChannelID(channelID string) {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return
	}
	path := service.mattermostFlowChannelIDPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o700); errorValue != nil {
		return
	}
	_ = os.WriteFile(path, []byte(normalizedChannelID), 0o600)
}

func (service *Service) mattermostFlowChannelIDPath() string {
	return filepath.Join(service.Configuration.StateDirectory, "mattermost-flow-channel-id")
}

func (service *Service) ensureMattermostPublicChannel(ctx context.Context, token string, teamID string, channelName string, displayName string) (string, error) {
	channelID, errorValue := service.mattermostChannelIDByName(ctx, token, teamID, channelName)
	if errorValue == nil || channelID != "" {
		return channelID, errorValue
	}
	if !isMattermostNotFound(errorValue) {
		return "", errorValue
	}
	body := map[string]string{
		"team_id":      teamID,
		"name":         channelName,
		"display_name": displayName,
		"type":         "O",
	}
	var channelRecord mattermostChannelRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels", token, body, &channelRecord); errorValue != nil {
		return "", errorValue
	}
	return channelRecord.ID, nil
}

func (service *Service) updateMattermostFlowChannelText(ctx context.Context, token string, channelID string) error {
	channel, _ := service.mattermostManagedPublicChannel(mattermostFlowChannelName)
	return service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel)
}

func (service *Service) updateMattermostCalendarChannelText(ctx context.Context, token string, channelID string) error {
	channel, _ := service.mattermostManagedPublicChannel(mattermostCalendarChannelName)
	return service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel)
}

func (service *Service) mattermostManagedPublicChannel(channelName string) (mattermostdefaults.PublicChannel, bool) {
	return mattermostdefaults.PublicChannelForLanguage(channelName, service.workspaceLanguage())
}

func (service *Service) updateMattermostManagedPublicChannelText(ctx context.Context, token string, channelID string, channel mattermostdefaults.PublicChannel) error {
	header := service.mattermostManagedPublicChannelHeader(channel)
	body := map[string]string{
		"display_name": channel.DisplayName,
		"header":       header,
		"purpose":      service.mattermostManagedPublicChannelPurpose(channel),
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", token, body, nil); errorValue != nil {
		return errorValue
	}
	return service.cleanupMattermostManagedChannelSystemPosts(ctx, token, channelID)
}

func (service *Service) mattermostManagedPublicChannelHeader(channel mattermostdefaults.PublicChannel) string {
	switch channel.Name {
	case mattermostFlowChannelName:
		return service.mattermostFlowLink("")
	case mattermostCalendarChannelName:
		return service.mattermostCalendarLink("")
	case attendanceChannelName:
		return service.mattermostAttendanceLink()
	default:
		return channel.Header
	}
}

func (service *Service) mattermostManagedPublicChannelPurpose(channel mattermostdefaults.PublicChannel) string {
	return channel.Purpose
}

func (service *Service) ensureMattermostManagedChannelModeration(ctx context.Context, token string, channelID string) error {
	body := []map[string]any{
		mattermostChannelModerationPatch("create_post", map[string]bool{"members": true, "guests": false}),
		mattermostChannelModerationPatch("create_reactions", map[string]bool{"members": false, "guests": false}),
		mattermostChannelModerationPatch("manage_members", map[string]bool{"members": false}),
		mattermostChannelModerationPatch("use_channel_mentions", map[string]bool{"members": false, "guests": false}),
	}
	errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/moderations/patch", token, body, nil)
	if errorValue != nil && isMattermostChannelModerationUnavailable(errorValue) {
		log.Printf("channel moderation unavailable on this Mattermost edition, leaving channel %s unmoderated", channelID)
		return nil
	}
	return errorValue
}

func isMattermostChannelModerationUnavailable(errorValue error) bool {
	message := errorValue.Error()
	return strings.Contains(message, "channel moderation") || strings.Contains(message, "patch_channel_moderations")
}

func mattermostChannelModerationPatch(name string, roles map[string]bool) map[string]any {
	return map[string]any{
		"name":  name,
		"roles": roles,
	}
}

func (service *Service) cleanupSavedMattermostManagedChannelSystemPosts(ctx context.Context, token string) error {
	channelIDs := []string{
		readTrimmedFile(service.mattermostFlowChannelIDPath()),
		readTrimmedFile(service.mattermostCalendarChannelIDPath()),
		readTrimmedFile(service.mattermostAttendanceChannelIDPath()),
	}
	var cleanupErrors []error
	for _, channelID := range uniqueNonEmpty(channelIDs) {
		if errorValue := service.cleanupMattermostManagedChannelSystemPosts(ctx, token, channelID); errorValue != nil {
			cleanupErrors = append(cleanupErrors, errorValue)
		}
	}
	return errors.Join(cleanupErrors...)
}

func (service *Service) cleanupMattermostManagedChannelSystemPosts(ctx context.Context, token string, channelID string) error {
	for _, postRecord := range service.mattermostFlowPosts(ctx, token, channelID, 100) {
		if !isMattermostManagedChannelSystemPost(postRecord) || strings.TrimSpace(postRecord.ID) == "" {
			continue
		}
		if errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(postRecord.ID), token, nil, nil); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) syncMattermostFlowEntryPost(ctx context.Context, adminToken string, channelID string) {
	if errorValue := service.deleteMattermostFlowEntryPost(ctx, adminToken, channelID); errorValue != nil {
		log.Printf("Mattermost Flow entry post sync failed: %v", errorValue)
	}
}

func (service *Service) deleteMattermostFlowEntryPost(ctx context.Context, adminToken string, channelID string) error {
	botToken, errorValue := service.mattermostBotToken()
	if errorValue != nil {
		return errorValue
	}
	botUserID, errorValue := service.mattermostTokenUserID(ctx, botToken)
	if errorValue != nil {
		return errorValue
	}
	post, found := service.mattermostFlowEntryPost(ctx, adminToken, channelID)
	if !found || strings.TrimSpace(post.ID) == "" || strings.TrimSpace(post.UserID) != botUserID {
		return nil
	}
	path := "/api/v4/posts/" + url.PathEscape(post.ID)
	if errorValue := service.mattermostRequest(ctx, http.MethodDelete, path, adminToken, nil, nil); errorValue != nil && !isMattermostNotFound(errorValue) {
		return errorValue
	}
	return nil
}

func (service *Service) mattermostFlowEntryPost(ctx context.Context, token string, channelID string) (mattermostPostRecord, bool) {
	for _, postRecord := range service.mattermostFlowPosts(ctx, token, channelID, 50) {
		if isMattermostFlowEntryPost(postRecord) {
			return postRecord, true
		}
	}
	return mattermostPostRecord{}, false
}

func (service *Service) mattermostFlowPosts(ctx context.Context, token string, channelID string, limit int) []mattermostPostRecord {
	var response mattermostPostsResponse
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/posts?per_page=" + strconv.Itoa(limit)
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &response); errorValue != nil {
		return nil
	}
	posts := make([]mattermostPostRecord, 0, len(response.Posts))
	for _, postID := range response.Order {
		if postRecord, found := response.Posts[postID]; found {
			posts = append(posts, postRecord)
		}
	}
	if len(posts) > 0 {
		return posts
	}
	for _, postRecord := range response.Posts {
		posts = append(posts, postRecord)
	}
	return posts
}

func isMattermostFlowEntryPost(post mattermostPostRecord) bool {
	if post.Props == nil {
		return false
	}
	value, found := post.Props["internkim_flow_entry"]
	return found && value == true
}

func isMattermostManagedChannelSystemPost(post mattermostPostRecord) bool {
	switch strings.TrimSpace(post.Type) {
	case "system_add_to_channel", "system_displayname_change", "system_header_change", "system_join_channel", "system_purpose_change":
		return true
	default:
		return false
	}
}

func (service *Service) syncMattermostCircleMemberships(ctx context.Context, token string) error {
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	circleEmailsByID, errorValue := service.mattermostCircleEmails(ctx, token, teamRecord.ID)
	if errorValue != nil {
		return errorValue
	}
	return service.applyCircleEmailsToBlueclawPolicy(ctx, circleEmailsByID)
}

func (service *Service) mattermostCircleEmails(ctx context.Context, token string, teamID string) (map[string]map[string]bool, error) {
	circleChannels, errorValue := service.mattermostCircleChannelDefinitions(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	circleEmailsByID := map[string]map[string]bool{}
	for _, circleChannel := range circleChannels {
		channelID, errorValue := service.ensureMattermostPrivateChannel(ctx, token, teamID, circleChannel.ChannelName)
		if errorValue != nil {
			return nil, errorValue
		}
		emails, errorValue := service.mattermostChannelMemberEmails(ctx, token, channelID)
		if errorValue != nil {
			return nil, errorValue
		}
		circleEmailsByID[circleChannel.CircleID] = emails
	}
	return circleEmailsByID, nil
}

func (service *Service) mattermostChannelMemberEmails(ctx context.Context, token string, channelID string) (map[string]bool, error) {
	var members []mattermostChannelMemberRecord
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/members?per_page=200"
	if errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &members); errorValue != nil {
		return nil, errorValue
	}
	emails := map[string]bool{}
	for _, member := range members {
		userRecord, found, errorValue := service.findMattermostUserByID(ctx, token, member.UserID)
		if errorValue != nil {
			return nil, errorValue
		}
		if found && strings.TrimSpace(userRecord.Email) != "" {
			emails[strings.ToLower(strings.TrimSpace(userRecord.Email))] = true
		}
	}
	return emails, nil
}

func (service *Service) syncMattermostUserCircleMemberships(ctx context.Context, record adminUserMutation) error {
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	userID := strings.TrimSpace(record.MattermostUserID)
	if userID == "" {
		userRecord, found, errorValue := service.findMattermostUserByEmail(ctx, token, record.Email)
		if errorValue != nil {
			return errorValue
		}
		if !found {
			return nil
		}
		userID = userRecord.ID
	}
	selectedCircles := map[string]bool{}
	for _, circleID := range normalizeAdminUserCircles(record.Circles, record.Role) {
		selectedCircles[circleID] = true
	}
	circleChannels, errorValue := service.mattermostCircleChannelDefinitions(ctx)
	if errorValue != nil {
		return errorValue
	}
	for _, circleChannel := range circleChannels {
		channelID, errorValue := service.ensureMattermostPrivateChannel(ctx, token, teamRecord.ID, circleChannel.ChannelName)
		if errorValue != nil {
			return errorValue
		}
		if selectedCircles[circleChannel.CircleID] {
			if errorValue := service.ensureMattermostChannelMember(ctx, token, channelID, userID); errorValue != nil {
				return errorValue
			}
			continue
		}
		if errorValue := service.removeMattermostChannelMember(ctx, token, channelID, userID); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) removeMattermostChannelMember(ctx context.Context, token string, channelID string, userID string) error {
	if strings.TrimSpace(channelID) == "" || strings.TrimSpace(userID) == "" {
		return nil
	}
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/members/" + url.PathEscape(userID)
	if errorValue := service.mattermostRequest(ctx, http.MethodDelete, path, token, nil, nil); errorValue != nil && !isMattermostNotFound(errorValue) {
		return errorValue
	}
	return nil
}

func (service *Service) applyCircleEmailsToBlueclawPolicy(ctx context.Context, circleEmailsByID map[string]map[string]bool) error {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return errorValue
	}
	people, _ := policyDocument["people"].([]any)
	hasPolicyChange := false
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson {
			continue
		}
		syncedCircles := mattermostSyncedPersonCircles(person, circleEmailsByID)
		if mattermostCircleSetsEqual(policyStringList(person["circles"]), syncedCircles) {
			continue
		}
		person["circles"] = syncedCircles
		hasPolicyChange = true
	}
	if !hasPolicyChange {
		return nil
	}
	return service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/policy/save", policyDocument, nil)
}

func mattermostCircleSetsEqual(current []string, synced []string) bool {
	currentCopy := append([]string{}, current...)
	syncedCopy := append([]string{}, synced...)
	sort.Strings(currentCopy)
	sort.Strings(syncedCopy)
	return slices.Equal(currentCopy, syncedCopy)
}

func mattermostSyncedPersonCircles(person map[string]any, circleEmailsByID map[string]map[string]bool) []string {
	emailValues, _ := person["emails"].([]any)
	circles := []string{"staff"}
	if isAdmin, _ := person["isAdmin"].(bool); isAdmin {
		circles = append(circles, "admin")
	}
	for _, circle := range policyStringList(person["circles"]) {
		normalizedCircle := strings.ToLower(strings.TrimSpace(circle))
		if normalizedCircle == "" || normalizedCircle == "staff" || normalizedCircle == "admin" {
			continue
		}
		if _, isMattermostManaged := circleEmailsByID[normalizedCircle]; !isMattermostManaged {
			circles = append(circles, normalizedCircle)
		}
	}
	for circleID, emails := range circleEmailsByID {
		for _, value := range emailValues {
			email, isString := value.(string)
			if isString && emails[strings.ToLower(strings.TrimSpace(email))] {
				circles = append(circles, circleID)
				break
			}
		}
	}
	return uniqueMattermostCircles(circles)
}

func uniqueMattermostCircles(circles []string) []string {
	seenCircle := map[string]bool{}
	uniqueCircles := []string{}
	for _, circle := range circles {
		normalizedCircle := strings.ToLower(strings.TrimSpace(circle))
		if normalizedCircle == "" || seenCircle[normalizedCircle] {
			continue
		}
		seenCircle[normalizedCircle] = true
		uniqueCircles = append(uniqueCircles, normalizedCircle)
	}
	return uniqueCircles
}

type mattermostCircleChannelDefinition struct {
	CircleID    string
	ChannelName string
}

func (service *Service) mattermostCircleChannelDefinitions(ctx context.Context) ([]mattermostCircleChannelDefinition, error) {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return nil, errorValue
	}
	circleChannels := mattermostCircleChannelDefinitionsFromPolicy(policyDocument)
	if len(circleChannels) == 0 {
		return defaultMattermostCircleChannelDefinitions(), nil
	}
	return circleChannels, nil
}

func mattermostCircleChannelDefinitionsFromPolicy(policyDocument map[string]any) []mattermostCircleChannelDefinition {
	circleSync, _ := policyDocument["circleSync"].(map[string]any)
	channelValues, _ := circleSync["mattermostPrivateChannels"].([]any)
	circleChannels := []mattermostCircleChannelDefinition{}
	for _, value := range channelValues {
		channel, isChannel := value.(map[string]any)
		if !isChannel {
			continue
		}
		circleChannel := mattermostCircleChannelDefinition{
			CircleID:    strings.ToLower(strings.TrimSpace(mattermostPolicyString(channel["circleID"]))),
			ChannelName: strings.ToLower(strings.TrimSpace(mattermostPolicyString(channel["channelName"]))),
		}
		if circleChannel.CircleID != "" && circleChannel.ChannelName != "" {
			circleChannels = append(circleChannels, circleChannel)
		}
	}
	return circleChannels
}

func mattermostPolicyString(value any) string {
	stringValue, _ := value.(string)
	return stringValue
}

func defaultMattermostCircleChannelDefinitions() []mattermostCircleChannelDefinition {
	return []mattermostCircleChannelDefinition{
		{CircleID: "c-level", ChannelName: "circle-c-level"},
		{CircleID: "representative", ChannelName: "circle-representative"},
		{CircleID: "admin", ChannelName: "circle-admin"},
		{CircleID: "hr-compensation", ChannelName: "circle-hr-compensation"},
	}
}

func (service *Service) mattermostChannelIDByName(ctx context.Context, token string, teamID string, channelName string) (string, error) {
	var channelRecord mattermostChannelRecord
	path := "/api/v4/teams/" + url.PathEscape(teamID) + "/channels/name/" + url.PathEscape(channelName)
	errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &channelRecord)
	return channelRecord.ID, errorValue
}

func (service *Service) ensureMattermostTeam(ctx context.Context, token string) (mattermostTeamRecord, error) {
	teamName := strings.TrimSpace(service.Configuration.MattermostTeamName)
	if teamName == "" {
		teamName = "internkim"
	}
	var teamRecord mattermostTeamRecord
	errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/teams/name/"+url.PathEscape(teamName), token, nil, &teamRecord)
	if errorValue == nil && teamRecord.ID != "" {
		return teamRecord, nil
	}
	if errorValue != nil && !isMattermostNotFound(errorValue) {
		return mattermostTeamRecord{}, errorValue
	}

	body := map[string]string{"name": teamName, "display_name": "Intern Kim", "type": "I"}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/teams", token, body, &teamRecord); errorValue != nil {
		return mattermostTeamRecord{}, errorValue
	}
	if teamRecord.ID == "" {
		return mattermostTeamRecord{}, fmt.Errorf("Mattermost team %s was not created", teamName)
	}
	return teamRecord, nil
}

func (service *Service) ensureMattermostTownSquareChannel(ctx context.Context, token string, teamID string) (string, error) {
	channel, _ := service.mattermostManagedPublicChannel(mattermostdefaults.TownSquareChannelName)
	channelIDPath := filepath.Join(filepath.Dir(service.Configuration.FleetIDPath), "channel-id")
	channelID := strings.TrimSpace(readTrimmedFile(channelIDPath))
	if channelID != "" {
		var channelRecord mattermostChannelRecord
		errorValue := service.mattermostRequest(ctx, http.MethodGet, "/api/v4/channels/"+url.PathEscape(channelID), token, nil, &channelRecord)
		if errorValue == nil && channelRecord.ID != "" {
			return channelRecord.ID, service.updateMattermostManagedPublicChannelText(ctx, token, channelRecord.ID, channel)
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
		body := map[string]string{"team_id": teamID, "name": channel.Name, "display_name": channel.DisplayName, "type": "O"}
		if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels", token, body, &channelRecord); errorValue != nil {
			return "", errorValue
		}
	}
	if channelRecord.ID == "" {
		return "", fmt.Errorf("Mattermost channel town-square was not created")
	}
	if errorValue := service.updateMattermostManagedPublicChannelText(ctx, token, channelRecord.ID, channel); errorValue != nil {
		return "", errorValue
	}
	if errorValue := os.WriteFile(channelIDPath, []byte(channelRecord.ID), 0o640); errorValue != nil {
		return "", errorValue
	}
	return channelRecord.ID, nil
}

func (service *Service) ensureMattermostOffTopicChannel(ctx context.Context, token string, teamID string) (string, error) {
	channel, _ := service.mattermostManagedPublicChannel(mattermostdefaults.OffTopicChannelName)
	channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamID, channel.Name, channel.DisplayName)
	if errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.updateMattermostManagedPublicChannelText(ctx, token, channelID, channel); errorValue != nil {
		return "", errorValue
	}
	return channelID, nil
}

func (service *Service) ensureMattermostBotDirectChannel(ctx context.Context, token string, userID string) error {
	normalizedUserID := strings.TrimSpace(userID)
	if normalizedUserID == "" {
		return nil
	}
	botRecord, found, errorValue := service.findMattermostUserByUsername(ctx, token, service.Configuration.BotUsername)
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

func (service *Service) ensureMattermostBotChannelMember(ctx context.Context, token string, channelID string) error {
	botRecord, found, errorValue := service.findMattermostUserByUsername(ctx, token, service.Configuration.BotUsername)
	if errorValue != nil {
		return errorValue
	}
	if !found || botRecord.ID == "" || botRecord.DeleteAt != 0 {
		return fmt.Errorf("InternKim bot user is not available")
	}
	return service.ensureMattermostChannelMember(ctx, token, channelID, botRecord.ID)
}

func (service *Service) ensureMattermostChannelMember(ctx context.Context, token string, channelID string, userID string) error {
	if strings.TrimSpace(channelID) == "" || strings.TrimSpace(userID) == "" {
		return nil
	}
	channelMember := map[string]string{"user_id": strings.TrimSpace(userID)}
	path := "/api/v4/channels/" + url.PathEscape(channelID) + "/members"
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, path, token, channelMember, nil); errorValue != nil && !isMattermostBadRequest(errorValue) {
		return errorValue
	}
	return nil
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

func (service *Service) ensureMattermostBotEphemeralPermission(ctx context.Context, token string) error {
	botRecord, found, errorValue := service.findMattermostUserByUsername(ctx, token, service.Configuration.BotUsername)
	if errorValue != nil {
		return errorValue
	}
	if !found || botRecord.DeleteAt != 0 || strings.TrimSpace(botRecord.ID) == "" {
		return nil
	}
	if strings.Contains(" "+botRecord.Roles+" ", " system_admin ") {
		return nil
	}
	body := map[string]string{"roles": strings.TrimSpace(botRecord.Roles + " system_admin")}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/users/"+url.PathEscape(botRecord.ID)+"/roles", token, body, nil)
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

func isMattermostForbidden(errorValue error) bool {
	var apiError mattermostAPIError
	return errors.As(errorValue, &apiError) && apiError.StatusCode == http.StatusForbidden
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
