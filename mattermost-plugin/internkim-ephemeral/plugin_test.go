package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type testEphemeralPostUpdater struct {
	userID string
	post   *model.Post
}

func TestServeHTTPUpdatesEphemeralPost(t *testing.T) {
	updater := &testEphemeralPostUpdater{}
	pluginValue := &Plugin{updater: updater, activeConfiguration: &configuration{Secret: "shared-secret"}}
	pluginValue.API = testPluginAPI{secret: "shared-secret"}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/update-ephemeral", strings.NewReader(`{"userID":"user-1","postID":"post-1","channelID":"channel-1","rootID":"root-1","message":"확인"}`))
	request.Header.Set("X-InternKim-Token", "shared-secret")
	responseRecorder := httptest.NewRecorder()

	pluginValue.ServeHTTP(nil, responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if updater.userID != "user-1" {
		t.Fatalf("userID = %q", updater.userID)
	}
	if updater.post.Id != "post-1" || updater.post.ChannelId != "channel-1" || updater.post.RootId != "root-1" || updater.post.Message != "확인" {
		t.Fatalf("post = %+v", updater.post)
	}
	if attachments, ok := updater.post.Props["attachments"].([]interface{}); !ok || len(attachments) != 0 {
		t.Fatalf("attachments = %#v", updater.post.Props["attachments"])
	}
}

func TestServeHTTPRejectsInvalidSecret(t *testing.T) {
	pluginValue := &Plugin{updater: &testEphemeralPostUpdater{}, activeConfiguration: &configuration{Secret: "shared-secret"}}
	pluginValue.API = testPluginAPI{secret: "shared-secret"}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/update-ephemeral", strings.NewReader(`{"userID":"user-1","postID":"post-1","channelID":"channel-1","message":"확인"}`))
	request.Header.Set("X-InternKim-Token", "wrong")
	responseRecorder := httptest.NewRecorder()

	pluginValue.ServeHTTP(nil, responseRecorder, request)

	if responseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func TestServeHTTPRejectsInvalidInput(t *testing.T) {
	pluginValue := &Plugin{updater: &testEphemeralPostUpdater{}, activeConfiguration: &configuration{Secret: "shared-secret"}}
	pluginValue.API = testPluginAPI{secret: "shared-secret"}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/update-ephemeral", strings.NewReader(`{"userID":"user-1","channelID":"channel-1","message":"확인"}`))
	request.Header.Set("X-InternKim-Token", "shared-secret")
	responseRecorder := httptest.NewRecorder()

	pluginValue.ServeHTTP(nil, responseRecorder, request)

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func (updater *testEphemeralPostUpdater) updateEphemeralPost(userID string, post *model.Post) error {
	updater.userID = userID
	updater.post = post
	return nil
}

type testPluginAPI struct {
	plugin.API
	secret string
}

func (api testPluginAPI) GetConfig() *model.Config {
	configuration := &model.Config{}
	configuration.PluginSettings.Plugins = map[string]map[string]interface{}{
		pluginID: {"secret": api.secret},
	}
	return configuration
}

func (api testPluginAPI) LogWarn(message string, keyValuePairs ...interface{}) {
}
