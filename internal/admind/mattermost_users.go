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
	"time"
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

	Props map[string]json.RawMessage `json:"props,omitempty"`
}

type mattermostTeamRecord struct {
	ID string `json:"id"`
}

type mattermostChannelRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Purpose     string `json:"purpose"`
	Type        string `json:"type"`
	DeleteAt    int64  `json:"delete_at"`
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

// Every login mints a Mattermost session, and Mattermost drops the oldest once
// an account holds more than it allows. Signing in per call therefore evicts the
// sessions the earlier calls are still holding, so the admin session is kept and
// reused until Mattermost refuses it.
const mattermostAdminSessionLifetime = 30 * time.Minute

type mattermostAdminSession struct {
	token    string
	issuedAt time.Time
}

func (session mattermostAdminSession) isUsableAt(moment time.Time) bool {
	return session.token != "" && moment.Sub(session.issuedAt) < mattermostAdminSessionLifetime
}

func (service *Service) mattermostAdminToken(ctx context.Context) (string, error) {
	service.mattermostAdminSessionMutex.Lock()
	defer service.mattermostAdminSessionMutex.Unlock()
	if service.mattermostAdminSession.isUsableAt(time.Now()) {
		return service.mattermostAdminSession.token, nil
	}
	token, errorValue := service.logInAsMattermostAdmin(ctx)
	if errorValue != nil {
		return "", errorValue
	}
	service.mattermostAdminSession = mattermostAdminSession{token: token, issuedAt: time.Now()}
	return token, nil
}

func (service *Service) forgetMattermostAdminToken(refused string) {
	service.mattermostAdminSessionMutex.Lock()
	defer service.mattermostAdminSessionMutex.Unlock()
	if service.mattermostAdminSession.token != refused {
		return
	}
	service.mattermostAdminSession = mattermostAdminSession{}
}

func (service *Service) logInAsMattermostAdmin(ctx context.Context) (string, error) {
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
	return channel.Header
}

func (service *Service) mattermostManagedPublicChannelPurpose(channel mattermostdefaults.PublicChannel) string {
	return channel.Purpose
}

func (service *Service) cleanupMattermostManagedChannelSystemPosts(ctx context.Context, token string, channelID string) error {
	for _, postRecord := range service.mattermostTaskPosts(ctx, token, channelID, 100) {
		if !isMattermostManagedChannelSystemPost(postRecord) || strings.TrimSpace(postRecord.ID) == "" {
			continue
		}
		if errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(postRecord.ID), token, nil, nil); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) mattermostTaskPosts(ctx context.Context, token string, channelID string, limit int) []mattermostPostRecord {
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

func isMattermostManagedChannelSystemPost(post mattermostPostRecord) bool {
	switch strings.TrimSpace(post.Type) {
	case "system_add_to_channel", "system_displayname_change", "system_header_change", "system_join_channel", "system_purpose_change":
		return true
	default:
		return false
	}
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
		currentCircles := policyStringList(person["circles"])
		syncedCircles := mattermostSyncedPersonCircles(person, circleEmailsByID)
		if mattermostCircleSetsEqual(currentCircles, syncedCircles) {
			continue
		}
		reportCircleMembershipChange(person, currentCircles, syncedCircles)
		person["circles"] = syncedCircles
		hasPolicyChange = true
	}
	if !hasPolicyChange {
		return nil
	}
	return service.deliverBlueclawPolicy(ctx, policyDocument)
}

// A circle is what a person can reach on the workspace, so taking one away names
// which one and whose.
func reportCircleMembershipChange(person map[string]any, current []string, synced []string) {
	kept := map[string]bool{}
	for _, circle := range synced {
		kept[circle] = true
	}
	removed := []string{}
	for _, circle := range current {
		if !kept[circle] {
			removed = append(removed, circle)
		}
	}
	if len(removed) == 0 {
		return
	}
	log.Printf("Mattermost circle sync removes %s from %s", strings.Join(removed, ","), policyPersonLabel(person))
}

func policyPersonLabel(person map[string]any) string {
	if emails := policyStringList(person["emails"]); len(emails) > 0 {
		return emails[0]
	}
	personID, _ := person["personID"].(string)
	return personID
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
	circles := []string{"member"}
	if isAdmin, _ := person["isAdmin"].(bool); isAdmin {
		circles = append(circles, "admin")
	}
	for _, circle := range policyStringList(person["circles"]) {
		normalizedCircle := strings.ToLower(strings.TrimSpace(circle))
		if normalizedCircle == "" || isTheCircleEveryoneIsIn(normalizedCircle) || normalizedCircle == "admin" {
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
		{CircleID: "hr", ChannelName: "circle-hr"},
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
	if response.StatusCode == http.StatusUnauthorized {
		service.forgetMattermostAdminToken(token)
	}
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
