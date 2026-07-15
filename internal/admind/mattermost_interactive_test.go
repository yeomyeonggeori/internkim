package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMattermostInteractiveActionURLUsesConfiguredPublicBaseURL(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.ListenAddress = "0.0.0.0:18080"
	configuration.MattermostInteractiveBaseURL = " https://poc0-t15.example.test/ "
	service := &Service{Configuration: configuration}

	actualURL := service.mattermostInteractiveActionURL()
	if actualURL != "https://poc0-t15.example.test/_internkim/mattermost/actions" {
		t.Fatalf("interactive action URL = %q", actualURL)
	}
}

func TestMattermostAskActionAcknowledgesWithResolvedUpdateAndForwardsEvent(t *testing.T) {
	forwardedRequests := make(chan map[string]any, 1)
	deletedPosts := make(chan mattermostEphemeralPluginDeleteRequest, 1)
	service := &Service{Configuration: DefaultConfiguration()}
	service.Configuration.StateDirectory = t.TempDir()
	service.Configuration.BlueclawBaseURL = "http://blueclaw.test"
	service.Configuration.MattermostBaseURL = "http://mattermost.test"
	writeFile(t, service.mattermostEphemeralPluginSecretPath(), "shared-secret")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://mattermost.test/plugins/com.internkim.ephemeral/api/v1/delete-ephemeral" && request.Method == http.MethodPost {
			var payload mattermostEphemeralPluginDeleteRequest
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected delete payload to decode: %v", errorValue)
			}
			deletedPosts <- payload
			return jsonResponse(http.StatusOK, `{}`, nil), nil
		}
		if request.URL.String() != "http://blueclaw.test/connectors/mattermost/events" || request.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		var payload map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatalf("expected forward payload to decode: %v", errorValue)
		}
		forwardedRequests <- payload
		return jsonResponse(http.StatusOK, `{"handled":true}`, nil), nil
	})}
	token := service.ensureMattermostInteractiveActionToken()
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/actions", strings.NewReader(`{"user_id":"user-1","post_id":"post-1","channel_id":"channel-1","context":{"action":"ask.cancel","token":"`+token+`","interactionID":"interaction-1","taskRunID":"task-1","conversationID":"channel-1","replyTargetID":"target-1","targetUserID":"user-1"}}`))
	responseRecorder := httptest.NewRecorder()

	service.handleMattermostInteractiveAction(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected success response, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response mattermostInteractiveResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	assertMattermostAskAcknowledged(t, response)
	select {
	case payload := <-forwardedRequests:
		eventDocument, isMap := payload["event"].(map[string]any)
		if !isMap {
			t.Fatalf("expected normalized event envelope, got %+v", payload)
		}
		if eventDocument["prompt"] != "rejected" || eventDocument["replyTargetID"] != "target-1" {
			t.Fatalf("expected normalized ask prompt and target, got %+v", eventDocument)
		}
		legacyFields, isMap := eventDocument["legacyFields"].(map[string]any)
		if !isMap {
			t.Fatalf("expected legacy fields, got %+v", eventDocument)
		}
		if legacyFields["askAction"] != "cancel" || legacyFields["taskRunID"] != "task-1" || legacyFields["postID"] != "post-1" || legacyFields["ephemeralAsk"] != true {
			t.Fatalf("expected ask action to be forwarded, got %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("expected ask action to be forwarded")
	}
	select {
	case payload := <-deletedPosts:
		if payload.PostID != "post-1" || payload.UserID != "user-1" {
			t.Fatalf("deleted ephemeral post = %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("expected ask control post to be deleted")
	}
}

func TestMattermostAskActionKeepsControlWhenForwardFails(t *testing.T) {
	deleteRequests := make(chan struct{}, 1)
	service := &Service{Configuration: DefaultConfiguration()}
	service.Configuration.StateDirectory = t.TempDir()
	service.Configuration.BlueclawBaseURL = "http://blueclaw.test"
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.Contains(request.URL.String(), "delete-ephemeral") {
			deleteRequests <- struct{}{}
		}
		return jsonResponse(http.StatusServiceUnavailable, `{}`, nil), nil
	})}
	token := service.ensureMattermostInteractiveActionToken()
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/actions", strings.NewReader(`{"user_id":"user-1","post_id":"post-1","channel_id":"channel-1","context":{"action":"ask.confirm","token":"`+token+`","interactionID":"interaction-1","taskRunID":"task-1","conversationID":"channel-1","replyTargetID":"target-1","targetUserID":"user-1"}}`))
	responseRecorder := httptest.NewRecorder()

	service.handleMattermostInteractiveAction(responseRecorder, request)

	var response mattermostInteractiveResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.EphemeralText != "요청 전달에 실패했습니다. 버튼을 다시 눌러 주세요." || response.Update != nil {
		t.Fatalf("expected forward failure response, got %+v", response)
	}
	select {
	case <-deleteRequests:
		t.Fatal("expected failed forward to preserve the ask control")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMattermostAskActionAcknowledgesWithoutPostUpdate(t *testing.T) {
	service := &Service{Configuration: DefaultConfiguration()}
	service.Configuration.StateDirectory = t.TempDir()
	service.Configuration.BlueclawBaseURL = "http://blueclaw.test"
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"handled":true}`, nil), nil
	})}
	token := service.ensureMattermostInteractiveActionToken()
	requestBody := `{"user_id":"user-1","post_id":"post-1","channel_id":"channel-1","context":{"action":"ask.confirm","token":"` + token + `","interactionID":"interaction-1","taskRunID":"task-1","conversationID":"channel-1","replyTargetID":"target-1","targetUserID":"user-1"}}`
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/actions", strings.NewReader(requestBody))
	responseRecorder := httptest.NewRecorder()

	service.handleMattermostInteractiveAction(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected success response, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response mattermostInteractiveResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	assertMattermostAskAcknowledged(t, response)
}

func TestMattermostAskConfirmRequiresTargetUser(t *testing.T) {
	forwardedRequests := make(chan map[string]any, 1)
	service := &Service{Configuration: DefaultConfiguration()}
	service.Configuration.StateDirectory = t.TempDir()
	service.Configuration.BlueclawBaseURL = "http://blueclaw.test"
	service.Configuration.MattermostBaseURL = "http://mattermost.test"
	service.Configuration.MattermostTokenPath = writeTestFile(t, "bot-token")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://blueclaw.test/connectors/mattermost/events" || request.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
		}
		var payload map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatalf("expected forward payload to decode: %v", errorValue)
		}
		forwardedRequests <- payload
		return jsonResponse(http.StatusOK, `{"handled":true}`, nil), nil
	})}
	token := service.ensureMattermostInteractiveActionToken()
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/actions", strings.NewReader(`{"user_id":"user-2","post_id":"post-1","channel_id":"channel-1","context":{"action":"ask.confirm","token":"`+token+`","interactionID":"interaction-1","taskRunID":"task-1","conversationID":"channel-1","replyTargetID":"target-1","targetUserID":"user-1"}}`))
	responseRecorder := httptest.NewRecorder()

	service.handleMattermostInteractiveAction(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected success response, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response mattermostInteractiveResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.EphemeralText == "" {
		t.Fatalf("expected target mismatch ephemeral notice, got %+v", response)
	}
	select {
	case payload := <-forwardedRequests:
		t.Fatalf("expected mismatched target not to forward, got %+v", payload)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestMattermostAskActionAcceptsConfiguredInteractiveTokenPath(t *testing.T) {
	stateDirectory := t.TempDir()
	tokenPath := writeTestFile(t, "configured-token")
	service := &Service{Configuration: DefaultConfiguration()}
	service.Configuration.StateDirectory = stateDirectory
	service.Configuration.MattermostInteractiveTokenPath = tokenPath
	service.Configuration.BlueclawBaseURL = "http://blueclaw.test"
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{"handled":true}`, nil), nil
	})}
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/actions", strings.NewReader(`{"user_id":"user-1","post_id":"post-1","channel_id":"channel-1","context":{"action":"ask.confirm","token":"configured-token","interactionID":"interaction-1","taskRunID":"task-1","conversationID":"channel-1","replyTargetID":"target-1","targetUserID":"user-1"}}`))
	responseRecorder := httptest.NewRecorder()

	service.handleMattermostInteractiveAction(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected configured token to be accepted, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response mattermostInteractiveResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.EphemeralText != "" {
		t.Fatalf("expected configured token to pass validation, got %q", response.EphemeralText)
	}
	assertMattermostAskAcknowledged(t, response)
}

func assertMattermostAskAcknowledged(t *testing.T, response mattermostInteractiveResponse) {
	t.Helper()
	if response.EphemeralText != "" {
		t.Fatalf("expected acknowledged ask without ephemeral error, got %q", response.EphemeralText)
	}
	if response.Update != nil {
		t.Fatalf("expected no immediate post update for ask acknowledgement, got %+v", response.Update)
	}
}
