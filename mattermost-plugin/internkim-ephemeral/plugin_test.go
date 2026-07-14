package main

import (
	"errors"
	"io"
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
	pluginValue.API = &testPluginAPI{secret: "shared-secret"}
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
	pluginValue.API = &testPluginAPI{secret: "shared-secret"}
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
	pluginValue.API = &testPluginAPI{secret: "shared-secret"}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/delete-ephemeral", strings.NewReader(`{"userID":"user-1"}`))
	request.Header.Set("X-InternKim-Token", "shared-secret")
	responseRecorder := httptest.NewRecorder()

	pluginValue.ServeHTTP(nil, responseRecorder, request)

	if responseRecorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body = %s", responseRecorder.Code, responseRecorder.Body.String())
	}
}

func TestMessageWillBePostedRejectsHumanPostInManagedChannels(t *testing.T) {
	for _, channel := range []*model.Channel{
		{Id: "flow-channel", Name: "flow"},
		{Id: "calendar-channel", Name: "calendar"},
	} {
		api := &testPluginAPI{
			channel: channel,
			botUser: &model.User{Id: "bot-1", Username: "internkim"},
		}
		pluginValue := &Plugin{activeConfiguration: &configuration{BotUsername: "internkim"}}
		pluginValue.API = api

		replacementPost, rejectionMessage := pluginValue.MessageWillBePosted(nil, &model.Post{UserId: "user-1", ChannelId: channel.Id, Message: "blocked"})

		if replacementPost != nil {
			t.Fatalf("replacement post = %+v", replacementPost)
		}
		if rejectionMessage != managedChannelPostRejectionMessage {
			t.Fatalf("rejection message = %q", rejectionMessage)
		}
	}
}

func TestMessageWillBePostedAllowsConfiguredBotAndSystemPosts(t *testing.T) {
	api := &testPluginAPI{
		channel: &model.Channel{Id: "calendar-channel", Name: "calendar"},
		botUser: &model.User{Id: "bot-1", Username: "internkim"},
	}
	pluginValue := &Plugin{activeConfiguration: &configuration{BotUsername: "internkim"}}
	pluginValue.API = api

	for _, post := range []*model.Post{
		{UserId: "bot-1", ChannelId: "calendar-channel", Message: "allowed"},
		{UserId: "user-1", ChannelId: "calendar-channel", Type: "system_join_channel", Message: "allowed"},
	} {
		replacementPost, rejectionMessage := pluginValue.MessageWillBePosted(nil, post)
		if replacementPost != nil || rejectionMessage != "" {
			t.Fatalf("post %+v was rejected: replacement=%+v rejection=%q", post, replacementPost, rejectionMessage)
		}
	}
}

func TestMessageHasBeenPostedSendsKoreanRuntimeUnavailableNoticeForBotDM(t *testing.T) {
	api := &testPluginAPI{
		channel:      &model.Channel{Id: "channel-1", Type: model.ChannelTypeDirect},
		botUser:      &model.User{Id: "bot-1", Username: "internkim"},
		channelUsers: []*model.User{{Id: "user-1"}, {Id: "bot-1"}},
		usersByID:    map[string]*model.User{"user-1": &model.User{Id: "user-1", Locale: "ko"}},
	}
	pluginValue := &Plugin{
		activeConfiguration: &configuration{RuntimeHealthURL: "http://runtime.local/health", BotUsername: "internkim"},
		healthClient: runtimeHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.String() != "http://runtime.local/health" {
				t.Fatalf("runtime health URL = %s", request.URL.String())
			}
			return nil, errors.New("runtime unavailable")
		}),
	}
	pluginValue.API = api

	pluginValue.MessageHasBeenPosted(nil, &model.Post{UserId: "user-1", ChannelId: "channel-1", Message: "도와줘"})

	if api.ephemeralUserID != "user-1" || api.ephemeralPost == nil {
		t.Fatalf("expected ephemeral notice, got user=%q post=%+v", api.ephemeralUserID, api.ephemeralPost)
	}
	if api.ephemeralPost.ChannelId != "channel-1" {
		t.Fatalf("ephemeral channel = %q", api.ephemeralPost.ChannelId)
	}
	if api.ephemeralPost.UserId != "bot-1" {
		t.Fatalf("ephemeral author = %q", api.ephemeralPost.UserId)
	}
	if !strings.Contains(api.ephemeralPost.Message, "잠시 후 다시 시도") {
		t.Fatalf("expected Korean unavailable copy, got %q", api.ephemeralPost.Message)
	}
}

func TestMessageHasBeenPostedSkipsNoticeWhenRuntimeIsHealthy(t *testing.T) {
	api := &testPluginAPI{
		channel:   &model.Channel{Id: "channel-1", Type: model.ChannelTypeOpen},
		botUser:   &model.User{Id: "bot-1", Username: "internkim"},
		usersByID: map[string]*model.User{"user-1": &model.User{Id: "user-1", Locale: "en"}},
	}
	pluginValue := &Plugin{
		activeConfiguration: &configuration{RuntimeHealthURL: "http://runtime.local/health", BotUsername: "internkim"},
		healthClient: runtimeHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"status":"ok"}`))}, nil
		}),
	}
	pluginValue.API = api

	pluginValue.MessageHasBeenPosted(nil, &model.Post{UserId: "user-1", ChannelId: "channel-1", Message: "@internkim ping"})

	if api.ephemeralPost != nil {
		t.Fatalf("expected no ephemeral notice when runtime is healthy, got %+v", api.ephemeralPost)
	}
}

func TestMessageHasBeenPostedRequiresMentionOutsideDM(t *testing.T) {
	api := &testPluginAPI{
		channel:   &model.Channel{Id: "channel-1", Type: model.ChannelTypeOpen},
		botUser:   &model.User{Id: "bot-1", Username: "internkim"},
		usersByID: map[string]*model.User{"user-1": &model.User{Id: "user-1", Locale: "en"}},
	}
	pluginValue := &Plugin{
		activeConfiguration: &configuration{RuntimeHealthURL: "http://runtime.local/health", BotUsername: "internkim"},
		healthClient: runtimeHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, errors.New("runtime unavailable")
		}),
	}
	pluginValue.API = api

	pluginValue.MessageHasBeenPosted(nil, &model.Post{UserId: "user-1", ChannelId: "channel-1", Message: "general ping"})

	if api.ephemeralPost != nil {
		t.Fatalf("expected no ephemeral notice without mention, got %+v", api.ephemeralPost)
	}
}

func TestLocalizedRuntimeUnavailableMessageUsesEnglishFallback(t *testing.T) {
	message := localizedRuntimeUnavailableMessage("en")
	if !strings.Contains(message, "Please try again") {
		t.Fatalf("expected English fallback copy, got %q", message)
	}
}

func (deleter *testEphemeralPostDeleter) deleteEphemeralPost(userID string, postID string) {
	deleter.userID = userID
	deleter.postID = postID
}

type testPluginAPI struct {
	plugin.API
	secret          string
	channel         *model.Channel
	botUser         *model.User
	channelUsers    []*model.User
	usersByID       map[string]*model.User
	ephemeralUserID string
	ephemeralPost   *model.Post
}

func (api *testPluginAPI) GetConfig() *model.Config {
	configuration := &model.Config{}
	configuration.PluginSettings.Plugins = map[string]map[string]interface{}{
		pluginID: {"secret": api.secret},
	}
	return configuration
}

func (api *testPluginAPI) GetChannel(channelID string) (*model.Channel, *model.AppError) {
	if api.channel == nil || api.channel.Id != channelID {
		return nil, nil
	}
	return api.channel, nil
}

func (api *testPluginAPI) GetUserByUsername(username string) (*model.User, *model.AppError) {
	if api.botUser == nil || api.botUser.Username != username {
		return nil, nil
	}
	return api.botUser, nil
}

func (api *testPluginAPI) GetUsersInChannel(channelID, sortBy string, page, perPage int) ([]*model.User, *model.AppError) {
	_ = sortBy
	_ = page
	_ = perPage
	if api.channel == nil || api.channel.Id != channelID {
		return nil, nil
	}
	return api.channelUsers, nil
}

func (api *testPluginAPI) GetUser(userID string) (*model.User, *model.AppError) {
	if api.usersByID == nil {
		return nil, nil
	}
	return api.usersByID[userID], nil
}

func (api *testPluginAPI) SendEphemeralPost(userID string, post *model.Post) *model.Post {
	api.ephemeralUserID = userID
	api.ephemeralPost = post
	return post
}

func (api *testPluginAPI) LogWarn(message string, keyValuePairs ...interface{}) {
	_ = message
	_ = keyValuePairs
}

type runtimeHealthRoundTripFunc func(request *http.Request) (*http.Response, error)

func (roundTrip runtimeHealthRoundTripFunc) Do(request *http.Request) (*http.Response, error) {
	return roundTrip(request)
}
