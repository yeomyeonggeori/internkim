package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const pluginID = "com.internkim.ephemeral"

type configuration struct {
	Secret string
}

type Plugin struct {
	plugin.MattermostPlugin
	updater             ephemeralPostUpdater
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

type ephemeralPostUpdater interface {
	updateEphemeralPost(userID string, post *model.Post) error
}

type pluginAPIUpdater struct {
	api plugin.API
}

type updateEphemeralRequest struct {
	UserID    string `json:"userID"`
	PostID    string `json:"postID"`
	ChannelID string `json:"channelID"`
	RootID    string `json:"rootID"`
	Message   string `json:"message"`
}

func main() {
	plugin.ClientMain(&Plugin{})
}

func (pluginValue *Plugin) ServeHTTP(_ *plugin.Context, responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost || request.URL.Path != "/api/v1/update-ephemeral" {
		http.NotFound(responseWriter, request)
		return
	}
	if !pluginValue.isValidToken(request.Header.Get("X-InternKim-Token")) {
		pluginValue.API.LogWarn("update-ephemeral rejected: token mismatch", "hasConfiguredSecret", pluginValue.sharedSecret() != "")
		http.Error(responseWriter, "invalid token", http.StatusUnauthorized)
		return
	}
	payload, errorValue := decodeUpdateEphemeralRequest(request)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := pluginValue.ephemeralPostUpdater().updateEphemeralPost(payload.UserID, payload.post()); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	responseWriter.WriteHeader(http.StatusOK)
}

func (pluginValue *Plugin) isValidToken(token string) bool {
	expectedToken := strings.TrimSpace(pluginValue.sharedSecret())
	return expectedToken != "" && strings.TrimSpace(token) == expectedToken
}

func (pluginValue *Plugin) sharedSecret() string {
	pluginValue.configurationLock.RLock()
	defer pluginValue.configurationLock.RUnlock()
	if pluginValue.activeConfiguration == nil {
		return ""
	}
	return strings.TrimSpace(pluginValue.activeConfiguration.Secret)
}

func (pluginValue *Plugin) ephemeralPostUpdater() ephemeralPostUpdater {
	if pluginValue.updater != nil {
		return pluginValue.updater
	}
	return pluginAPIUpdater{api: pluginValue.API}
}

func decodeUpdateEphemeralRequest(request *http.Request) (updateEphemeralRequest, error) {
	var payload updateEphemeralRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		return updateEphemeralRequest{}, errorValue
	}
	payload.UserID = strings.TrimSpace(payload.UserID)
	payload.PostID = strings.TrimSpace(payload.PostID)
	payload.ChannelID = strings.TrimSpace(payload.ChannelID)
	payload.RootID = strings.TrimSpace(payload.RootID)
	payload.Message = strings.TrimSpace(payload.Message)
	if payload.UserID == "" || payload.PostID == "" || payload.ChannelID == "" || payload.Message == "" {
		return updateEphemeralRequest{}, errors.New("userID, postID, channelID, and message are required")
	}
	return payload, nil
}

func (payload updateEphemeralRequest) post() *model.Post {
	return &model.Post{
		Id:        payload.PostID,
		ChannelId: payload.ChannelID,
		RootId:    payload.RootID,
		Message:   payload.Message,
		Props:     model.StringInterface{"attachments": []interface{}{}},
	}
}

func (updater pluginAPIUpdater) updateEphemeralPost(userID string, post *model.Post) error {
	updater.api.UpdateEphemeralPost(userID, post)
	return nil
}
