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
	"strconv"
	"strings"
	"unicode"

	"gitlab.com/eastriver/internkim/internal/identity"
	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

type adminUserMutation struct {
	UserID                 string `json:"userID,omitempty"`
	Handle                 string `json:"handle,omitempty"`
	Name                   string `json:"name,omitempty"`
	Email                  string `json:"email"`
	HireDate               string `json:"hireDate,omitempty"`
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
	LastName    string `json:"last_name"`
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

type mattermostChannelMemberRecord struct {
	UserID string `json:"user_id"`
}

type mattermostPostRecord struct {
	ID      string         `json:"id"`
	UserID  string         `json:"user_id"`
	Message string         `json:"message"`
	Type    string         `json:"type"`
	Props   map[string]any `json:"props"`
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
	userMembershipError := service.ensureMattermostDefaultChannelMemberships(ctx, adminToken, teamRecord.ID, channelIDs)
	connectCommandError := service.ensureMattermostConnectCommand(ctx, adminToken)
	return errors.Join(channelError, adminMembershipError, userMembershipError, connectCommandError)
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
			if errorValue := service.deleteMattermostPost(ctx, token, trimmedPostID); errorValue != nil && !isMattermostNotFound(errorValue) {
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
	return service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(postID), token, nil, nil)
}

func (service *Service) deleteBlueclawDirectConversationHistory(ctx context.Context, channelID string) error {
	normalizedChannelID := strings.TrimSpace(channelID)
	if normalizedChannelID == "" {
		return nil
	}
	sql := blueclawDirectConversationResetSQL(normalizedChannelID)
	command := "su -s /bin/bash postgres -c " + quoteShellValue("psql -d blueclaw -v ON_ERROR_STOP=1 <<'SQL'\n"+sql+"\nSQL")
	_, errorValue := service.runCommand(ctx, "sh", "-c", command)
	return errorValue
}

func blueclawDirectConversationResetSQL(channelID string) string {
	directConversationID := "dm:" + channelID
	threadConversationPattern := "thread:" + channelID + ":%"
	return fmt.Sprintf(`DO $$
DECLARE
  direct_conversation_id text := %s;
  thread_conversation_pattern text := %s;
BEGIN
  IF to_regclass('public.task_run') IS NOT NULL THEN
    DELETE FROM task_run
    WHERE origin_conversation_id = direct_conversation_id
       OR origin_conversation_id LIKE thread_conversation_pattern;
  END IF;

  IF to_regclass('public.raw_event') IS NOT NULL THEN
    DELETE FROM raw_event
    WHERE conversation_id IN (
      SELECT conversation_id FROM conversation
      WHERE platform = 'mattermost'
        AND (external_conversation_id = direct_conversation_id
          OR external_conversation_id LIKE thread_conversation_pattern)
    );
  END IF;

  IF to_regclass('public.graphiti_episode') IS NOT NULL THEN
    DELETE FROM graphiti_episode
    WHERE conversation_id = direct_conversation_id
       OR conversation_id LIKE thread_conversation_pattern;
  END IF;

  IF to_regclass('public.graphiti_namespace') IS NOT NULL THEN
    DELETE FROM graphiti_namespace
    WHERE scope_conversation_id = direct_conversation_id
       OR scope_conversation_id LIKE thread_conversation_pattern;
  END IF;

  IF to_regclass('public.conversation') IS NOT NULL THEN
    DELETE FROM conversation
    WHERE platform = 'mattermost'
      AND (external_conversation_id = direct_conversation_id
        OR external_conversation_id LIKE thread_conversation_pattern);
  END IF;
END $$;`, quoteSQLLiteral(directConversationID), quoteSQLLiteral(threadConversationPattern))
}

func quoteSQLLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
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
		"EnableBotAccountCreation": true,
		"EnableUserAccessTokens":   true,
		"ManagedResourcePaths":     mattermostdefaults.ManagedResourcePathSetting(),
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
	users, errorValue := service.allowedMattermostUsers(ctx, token)
	if errorValue != nil {
		return errorValue
	}
	var membershipErrors []error
	for _, user := range users {
		membershipErrors = append(membershipErrors, service.ensureMattermostDefaultChannelMembership(ctx, token, teamID, user.ID, channelIDs))
	}
	return errors.Join(membershipErrors...)
}

func (service *Service) allowedMattermostUsers(ctx context.Context, token string) ([]mattermostUserRecord, error) {
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
	channelMember := map[string]string{"user_id": userID}
	var membershipErrors []error
	for _, channelID := range channelIDs {
		if channelID == "" {
			continue
		}
		if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels/"+url.PathEscape(channelID)+"/members", token, channelMember, nil); errorValue != nil && !isMattermostBadRequest(errorValue) {
			membershipErrors = append(membershipErrors, errorValue)
		}
	}
	if flowChannelID := strings.TrimSpace(readTrimmedFile(service.mattermostFlowChannelIDPath())); flowChannelID != "" {
		_ = service.cleanupMattermostFlowSystemPosts(ctx, token, flowChannelID)
	}
	return errors.Join(membershipErrors...)
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
	return uniqueNonEmpty(channelIDs), errors.Join(channelErrors...)
}

func (service *Service) mattermostDefaultChannelProvisions() []mattermostDefaultChannelProvision {
	return []mattermostDefaultChannelProvision{
		{Name: mattermostdefaults.TownSquareChannelName, Ensure: service.ensureMattermostTownSquareChannel},
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
	for _, channelName := range []string{"circle-c-level", "circle-representative", "circle-admin"} {
		if _, errorValue := service.ensureMattermostPrivateChannel(ctx, token, teamRecord.ID, channelName); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) ensureMattermostPrivateChannel(ctx context.Context, token string, teamID string, channelName string) (string, error) {
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
		"display_name": channelDisplayName(channelName),
		"type":         "P",
	}
	var channelRecord mattermostChannelRecord
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/channels", token, body, &channelRecord); errorValue != nil {
		return "", errorValue
	}
	return channelRecord.ID, nil
}

func (service *Service) ensureMattermostFlowChannel(ctx context.Context, token string, teamID string) (string, error) {
	channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamID, mattermostFlowChannelName, mattermostFlowChannelDisplayName)
	if errorValue != nil {
		return "", errorValue
	}
	service.saveMattermostFlowChannelID(channelID)
	if errorValue := service.updateMattermostFlowChannelText(ctx, token, channelID); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.ensureMattermostFlowChannelReadOnly(ctx, token, channelID); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.deleteMattermostFlowEntryPost(ctx, token, channelID); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.cleanupMattermostFlowSystemPosts(ctx, token, channelID); errorValue != nil {
		return "", errorValue
	}
	return channelID, nil
}

func (service *Service) ensureMattermostCalendarChannel(ctx context.Context, token string, teamID string) (string, error) {
	channelID, errorValue := service.ensureMattermostPublicChannel(ctx, token, teamID, mattermostCalendarChannelName, mattermostCalendarChannelDisplayName)
	if errorValue != nil {
		return "", errorValue
	}
	service.saveMattermostCalendarChannelID(channelID)
	if errorValue := service.updateMattermostCalendarChannelText(ctx, token, channelID); errorValue != nil {
		return "", errorValue
	}
	if errorValue := service.ensureMattermostFlowChannelReadOnly(ctx, token, channelID); errorValue != nil {
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
	flowChannelLink := service.mattermostFlowLink("")
	body := map[string]string{
		"display_name": mattermostFlowChannelDisplayName,
		"header":       flowChannelLink,
		"purpose":      "",
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", token, body, nil)
}

func (service *Service) updateMattermostCalendarChannelText(ctx context.Context, token string, channelID string) error {
	calendarChannelLink := service.mattermostCalendarLink("")
	body := map[string]string{
		"display_name": mattermostCalendarChannelDisplayName,
		"header":       calendarChannelLink,
		"purpose":      "",
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/patch", token, body, nil)
}

func (service *Service) ensureMattermostFlowChannelReadOnly(ctx context.Context, token string, channelID string) error {
	body := []map[string]any{
		mattermostChannelModerationPatch("create_post", map[string]bool{"members": false, "guests": false}),
		mattermostChannelModerationPatch("create_reactions", map[string]bool{"members": false, "guests": false}),
		mattermostChannelModerationPatch("manage_members", map[string]bool{"members": false}),
		mattermostChannelModerationPatch("use_channel_mentions", map[string]bool{"members": false, "guests": false}),
	}
	return service.mattermostRequest(ctx, http.MethodPut, "/api/v4/channels/"+url.PathEscape(channelID)+"/moderations/patch", token, body, nil)
}

func mattermostChannelModerationPatch(name string, roles map[string]bool) map[string]any {
	return map[string]any{
		"name":  name,
		"roles": roles,
	}
}

func (service *Service) deleteMattermostFlowEntryPost(ctx context.Context, token string, channelID string) error {
	postRecord, found := service.mattermostFlowEntryPost(ctx, token, channelID)
	if !found || strings.TrimSpace(postRecord.ID) == "" {
		return nil
	}
	return service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(postRecord.ID), token, nil, nil)
}

func (service *Service) cleanupMattermostFlowSystemPosts(ctx context.Context, token string, channelID string) error {
	for _, postRecord := range service.mattermostFlowPosts(ctx, token, channelID, 100) {
		if !isMattermostFlowSystemPost(postRecord) || strings.TrimSpace(postRecord.ID) == "" {
			continue
		}
		if errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(postRecord.ID), token, nil, nil); errorValue != nil {
			return errorValue
		}
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

func isMattermostFlowSystemPost(post mattermostPostRecord) bool {
	switch strings.TrimSpace(post.Type) {
	case "system_add_to_channel", "system_join_channel", "system_purpose_change":
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
	circleEmailsByID := map[string]map[string]bool{}
	for circleID, channelName := range defaultMattermostCircleChannels() {
		channelID, errorValue := service.ensureMattermostPrivateChannel(ctx, token, teamID, channelName)
		if errorValue != nil {
			return nil, errorValue
		}
		emails, errorValue := service.mattermostChannelMemberEmails(ctx, token, channelID)
		if errorValue != nil {
			return nil, errorValue
		}
		circleEmailsByID[circleID] = emails
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

func (service *Service) applyCircleEmailsToBlueclawPolicy(ctx context.Context, circleEmailsByID map[string]map[string]bool) error {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return errorValue
	}
	people, _ := policyDocument["people"].([]any)
	for _, value := range people {
		person, isPerson := value.(map[string]any)
		if !isPerson {
			continue
		}
		person["circles"] = mattermostSyncedPersonCircles(person, circleEmailsByID)
	}
	return service.blueclawJSONRequest(ctx, http.MethodPost, "/admin/api/policy/save", policyDocument, nil)
}

func mattermostSyncedPersonCircles(person map[string]any, circleEmailsByID map[string]map[string]bool) []string {
	emailValues, _ := person["emails"].([]any)
	circles := []string{"staff"}
	if isAdmin, _ := person["isAdmin"].(bool); isAdmin {
		circles = append(circles, "admin")
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

func defaultMattermostCircleChannels() map[string]string {
	return map[string]string{
		"admin":          "circle-admin",
		"c-level":        "circle-c-level",
		"representative": "circle-representative",
	}
}

func (service *Service) mattermostChannelIDByName(ctx context.Context, token string, teamID string, channelName string) (string, error) {
	var channelRecord mattermostChannelRecord
	path := "/api/v4/teams/" + url.PathEscape(teamID) + "/channels/name/" + url.PathEscape(channelName)
	errorValue := service.mattermostRequest(ctx, http.MethodGet, path, token, nil, &channelRecord)
	return channelRecord.ID, errorValue
}

func channelDisplayName(channelName string) string {
	value := strings.TrimPrefix(strings.TrimSpace(channelName), "circle-")
	value = strings.ReplaceAll(value, "-", " ")
	if value == "" {
		return channelName
	}
	return "Circle " + value
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
	channelIDPath := filepath.Join(filepath.Dir(service.Configuration.FleetIDPath), "channel-id")
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
