package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

type testEphemeralPostDeleter struct {
	userID string
	postID string
}

func TestServeHTTPDeletesEphemeralPost(t *testing.T) {
	deleter := &testEphemeralPostDeleter{}
	pluginValue := &Plugin{deleter: deleter, activeConfiguration: &configuration{Secret: "shared-secret"}}
	pluginValue.API = testPluginAPI{secret: "shared-secret"}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/delete-ephemeral", strings.NewReader(`{"userID":"user-1","postID":"post-1"}`))
	request.Header.Set("X-InternKim-Token", "shared-secret")
	responseRecorder := httptest.NewRecorder()

	pluginValue.ServeHTTP(nil, responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if deleter.userID != "user-1" || deleter.postID != "post-1" {
		t.Fatalf("deleter = %+v", deleter)
	}
}

func TestServeHTTPRejectsInvalidSecret(t *testing.T) {
	pluginValue := &Plugin{deleter: &testEphemeralPostDeleter{}, activeConfiguration: &configuration{Secret: "shared-secret"}}
	pluginValue.API = testPluginAPI{secret: "shared-secret"}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/delete-ephemeral", strings.NewReader(`{"userID":"user-1","postID":"post-1"}`))
	request.Header.Set("X-InternKim-Token", "wrong")
	responseRecorder := httptest.NewRecorder()

	pluginValue.ServeHTTP(nil, responseRecorder, request)

	if responseRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func TestServeHTTPRejectsInvalidInput(t *testing.T) {
	pluginValue := &Plugin{deleter: &testEphemeralPostDeleter{}, activeConfiguration: &configuration{Secret: "shared-secret"}}
	pluginValue.API = testPluginAPI{secret: "shared-secret"}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/delete-ephemeral", strings.NewReader(`{"userID":"user-1"}`))
	request.Header.Set("X-InternKim-Token", "shared-secret")
	responseRecorder := httptest.NewRecorder()

	pluginValue.ServeHTTP(nil, responseRecorder, request)

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func (deleter *testEphemeralPostDeleter) deleteEphemeralPost(userID string, postID string) {
	deleter.userID = userID
	deleter.postID = postID
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
