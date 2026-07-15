package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const pluginID = "com.internkim.ephemeral"
const defaultRuntimeHealthURL = "http://127.0.0.1:8080/admin/api/health"
const defaultBotUsername = "internkim"
const unavailableNoticeProperty = "internkim_unavailable_notice"
const managedChannelPostRejectionMessage = "This channel is managed by InternKim. Use Flow or Calendar to make changes."

type configuration struct {
	Secret           string `json:"secret"`
	RuntimeHealthURL string `json:"runtimeHealthURL"`
	BotUsername      string `json:"botUsername"`
}

type Plugin struct {
	plugin.MattermostPlugin
	deleter             ephemeralPostDeleter
	healthClient        runtimeHealthHTTPClient
	configurationLock   sync.RWMutex
	activeConfiguration *configuration
}

func (pluginValue *Plugin) OnConfigurationChange() error {
	loadedConfiguration := new(configuration)
	if errorValue := pluginValue.API.LoadPluginConfiguration(loadedConfiguration); errorValue != nil {
		return errorValue
	}
	pluginValue.configurationLock.Lock()
	defer pluginValue.configurationLock.Unlock()
	pluginValue.activeConfiguration = loadedConfiguration
	return nil
}

type ephemeralPostDeleter interface {
	deleteEphemeralPost(userID string, postID string)
}

type runtimeHealthHTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

type pluginAPIDeleter struct {
	api plugin.API
}

type deleteEphemeralRequest struct {
	UserID string `json:"userID"`
	PostID string `json:"postID"`
}

func main() {
	plugin.ClientMain(&Plugin{})
}

func (pluginValue *Plugin) MessageWillBePosted(_ *plugin.Context, post *model.Post) (*model.Post, string) {
	if !pluginValue.shouldRejectManagedChannelPost(post) {
		return nil, ""
	}
	return nil, managedChannelPostRejectionMessage
}

func (pluginValue *Plugin) MessageHasBeenPosted(_ *plugin.Context, post *model.Post) {
	if !pluginValue.shouldHandlePost(post) {
		return
	}
	botUser := pluginValue.botUser()
	if botUser == nil || post.UserId == botUser.Id {
		return
	}
	if !pluginValue.isPostDirectedAtBot(post, botUser) {
		return
	}
	if pluginValue.isRuntimeHealthy() {
		return
	}
	pluginValue.sendRuntimeUnavailableNotice(post, botUser.Id)
}

func (pluginValue *Plugin) ServeHTTP(_ *plugin.Context, responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/api/v1/delete-ephemeral" {
		http.NotFound(responseWriter, request)
		return
	}
	if !pluginValue.isValidToken(request.Header.Get("X-InternKim-Token")) {
		pluginValue.API.LogWarn("delete-ephemeral rejected: token mismatch", "hasConfiguredSecret", pluginValue.sharedSecret() != "")
		http.Error(responseWriter, "invalid token", http.StatusUnauthorized)
		return
	}
	payload, errorValue := decodeDeleteEphemeralRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	pluginValue.ephemeralPostDeleter().deleteEphemeralPost(payload.UserID, payload.PostID)
	responseWriter.WriteHeader(http.StatusOK)
}

func (pluginValue *Plugin) isValidToken(token string) bool {
	expectedToken := strings.TrimSpace(pluginValue.sharedSecret())
	return expectedToken != "" && strings.TrimSpace(token) == expectedToken
}

func (pluginValue *Plugin) sharedSecret() string {
	return strings.TrimSpace(pluginValue.configurationSnapshot().Secret)
}

func (pluginValue *Plugin) runtimeHealthURL() string {
	return firstNonEmptyString(pluginValue.configurationSnapshot().RuntimeHealthURL, defaultRuntimeHealthURL)
}

func (pluginValue *Plugin) botUsername() string {
	return firstNonEmptyString(pluginValue.configurationSnapshot().BotUsername, defaultBotUsername)
}

func (pluginValue *Plugin) configurationSnapshot() configuration {
	pluginValue.configurationLock.RLock()
	defer pluginValue.configurationLock.RUnlock()
	if pluginValue.activeConfiguration == nil {
		return configuration{}
	}
	return *pluginValue.activeConfiguration
}

func (pluginValue *Plugin) ephemeralPostDeleter() ephemeralPostDeleter {
	if pluginValue.deleter != nil {
		return pluginValue.deleter
	}
	return pluginAPIDeleter{api: pluginValue.API}
}

func (pluginValue *Plugin) runtimeHealthHTTPClient() runtimeHealthHTTPClient {
	if pluginValue.healthClient != nil {
		return pluginValue.healthClient
	}
	return http.DefaultClient
}

func (pluginValue *Plugin) shouldRejectManagedChannelPost(post *model.Post) bool {
	if !pluginValue.shouldHandlePost(post) {
		return false
	}
	botUser := pluginValue.botUser()
	if botUser != nil && post.UserId == botUser.Id {
		return false
	}
	channel, appError := pluginValue.API.GetChannel(post.ChannelId)
	if appError != nil || channel == nil {
		return false
	}
	return isManagedChannelName(channel.Name)
}

func isManagedChannelName(channelName string) bool {
	switch strings.ToLower(strings.TrimSpace(channelName)) {
	case "flow", "calendar":
		return true
	default:
		return false
	}
}

func (pluginValue *Plugin) shouldHandlePost(post *model.Post) bool {
	if post == nil || post.UserId == "" || post.ChannelId == "" {
		return false
	}
	if post.DeleteAt != 0 || strings.HasPrefix(post.Type, "system_") {
		return false
	}
	if sentByPlugin, _ := post.GetProp("sent_by_plugin").(bool); sentByPlugin {
		return false
	}
	notice, _ := post.GetProp(unavailableNoticeProperty).(bool)
	return !notice
}

func (pluginValue *Plugin) botUser() *model.User {
	botUser, appError := pluginValue.API.GetUserByUsername(pluginValue.botUsername())
	if appError != nil || botUser == nil || !botUser.IsBot {
		pluginValue.API.LogWarn("runtime fallback skipped: bot user lookup failed", "botUsername", pluginValue.botUsername())
		return nil
	}
	return botUser
}

func (pluginValue *Plugin) isPostDirectedAtBot(post *model.Post, botUser *model.User) bool {
	channel, appError := pluginValue.API.GetChannel(post.ChannelId)
	if appError != nil || channel == nil {
		pluginValue.API.LogWarn("runtime fallback skipped: channel lookup failed", "channelID", post.ChannelId)
		return false
	}
	if channel.Type == model.ChannelTypeDirect {
		return pluginValue.directChannelContainsUser(channel.Id, botUser.Id)
	}
	return messageMentionsUsername(post.Message, botUser.Username)
}

func (pluginValue *Plugin) directChannelContainsUser(channelID string, userID string) bool {
	users, appError := pluginValue.API.GetUsersInChannel(channelID, "", 0, 20)
	if appError != nil {
		pluginValue.API.LogWarn("runtime fallback skipped: direct channel member lookup failed", "channelID", channelID)
		return false
	}
	for _, user := range users {
		if user != nil && user.Id == userID {
			return true
		}
	}
	return false
}

func (pluginValue *Plugin) isRuntimeHealthy() bool {
	requestContext, cancel := context.WithTimeout(context.Background(), 900*time.Millisecond)
	defer cancel()
	request, errorValue := http.NewRequestWithContext(requestContext, http.MethodGet, pluginValue.runtimeHealthURL(), nil)
	if errorValue != nil {
		return false
	}
	response, errorValue := pluginValue.runtimeHealthHTTPClient().Do(request)
	if errorValue != nil {
		return false
	}
	if response.Body != nil {
		defer response.Body.Close()
	}
	return response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices
}

func (pluginValue *Plugin) sendRuntimeUnavailableNotice(post *model.Post, botUserID string) {
	pluginValue.API.SendEphemeralPost(post.UserId, &model.Post{
		UserId:    botUserID,
		ChannelId: post.ChannelId,
		RootId:    post.RootId,
		Message:   localizedRuntimeUnavailableMessage(pluginValue.userLocale(post.UserId)),
		Props: model.StringInterface{
			"sent_by_plugin":          true,
			unavailableNoticeProperty: true,
		},
	})
}

func (pluginValue *Plugin) userLocale(userID string) string {
	user, appError := pluginValue.API.GetUser(userID)
	if appError != nil || user == nil {
		return ""
	}
	return user.Locale
}

func decodeDeleteEphemeralRequest(request *http.Request) (deleteEphemeralRequest, error) {
	var payload deleteEphemeralRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		return deleteEphemeralRequest{}, errorValue
	}
	payload.UserID = strings.TrimSpace(payload.UserID)
	payload.PostID = strings.TrimSpace(payload.PostID)
	if payload.UserID == "" || payload.PostID == "" {
		return deleteEphemeralRequest{}, errors.New("userID and postID are required")
	}
	return payload, nil
}

func (deleter pluginAPIDeleter) deleteEphemeralPost(userID string, postID string) {
	deleter.api.DeleteEphemeralPost(userID, postID)
}

func messageMentionsUsername(message string, username string) bool {
	normalizedUsername := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(username)), "@")
	if normalizedUsername == "" {
		return false
	}
	target := "@" + normalizedUsername
	remainingMessage := strings.ToLower(message)
	for {
		index := strings.Index(remainingMessage, target)
		if index < 0 {
			return false
		}
		afterIndex := index + len(target)
		if afterIndex == len(remainingMessage) || !isUsernameRune(rune(remainingMessage[afterIndex])) {
			return true
		}
		remainingMessage = remainingMessage[afterIndex:]
	}
}

func isUsernameRune(value rune) bool {
	return value >= 'a' && value <= 'z' ||
		value >= '0' && value <= '9' ||
		value == '_' ||
		value == '-' ||
		value == '.'
}

func localizedRuntimeUnavailableMessage(locale string) string {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(locale)), "ko") {
		return "지금 김인턴이 잠시 응답할 수 없습니다. 잠시 후 다시 시도해 주세요."
	}
	return "Kim Intern is temporarily unavailable. Please try again in a moment."
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmedValue := strings.TrimSpace(value); trimmedValue != "" {
			return trimmedValue
		}
	}
	return ""
}
