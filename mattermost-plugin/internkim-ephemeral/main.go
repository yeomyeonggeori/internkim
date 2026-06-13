package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/plugin"
)

const pluginID = "com.internkim.ephemeral"

type configuration struct {
	Secret string
}

type Plugin struct {
	plugin.MattermostPlugin
	deleter             ephemeralPostDeleter
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
	pluginValue.configurationLock.RLock()
	defer pluginValue.configurationLock.RUnlock()
	if pluginValue.activeConfiguration == nil {
		return ""
	}
	return strings.TrimSpace(pluginValue.activeConfiguration.Secret)
}

func (pluginValue *Plugin) ephemeralPostDeleter() ephemeralPostDeleter {
	if pluginValue.deleter != nil {
		return pluginValue.deleter
	}
	return pluginAPIDeleter{api: pluginValue.API}
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
