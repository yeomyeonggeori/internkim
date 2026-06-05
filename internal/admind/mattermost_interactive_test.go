package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMattermostAskActionAcknowledgesWithoutUpdatingPostAndForwardsEvent(t *testing.T) {
	forwardedRequests := make(chan map[string]any, 1)
	service := &Service{Configuration: DefaultConfiguration()}
	service.Configuration.StateDirectory = t.TempDir()
	service.Configuration.BlueclawBaseURL = "http://blueclaw.test"
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://blueclaw.test/connectors/mattermost/events" || request.Method != http.MethodPost {
			t.Fatalf("unexpected forward request: %s %s", request.Method, request.URL.String())
		}
		var payload map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatalf("expected forward payload to decode: %v", errorValue)
		}
		forwardedRequests <- payload
		return jsonResponse(http.StatusOK, `{"handled":true}`, nil), nil
	})}
	token := service.ensureMattermostInteractiveActionToken()
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/actions", strings.NewReader(`{"user_id":"user-1","post_id":"post-1","channel_id":"channel-1","context":{"action":"ask.cancel","token":"`+token+`","interactionID":"interaction-1","taskRunID":"task-1","conversationID":"channel-1","replyTargetID":"target-1"}}`))
	responseRecorder := httptest.NewRecorder()

	service.handleMattermostInteractiveAction(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected success response, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response mattermostInteractiveResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.Update != nil {
		t.Fatalf("expected immediate response not to update original post, got %+v", response.Update)
	}
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
		if legacyFields["askAction"] != "cancel" || legacyFields["taskRunID"] != "task-1" || legacyFields["postID"] != "post-1" {
			t.Fatalf("expected ask action to be forwarded, got %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("expected ask action to be forwarded")
	}
}

func TestMattermostAskConfirmAcknowledgesWithoutUpdatingPost(t *testing.T) {
	forwardedRequests := make(chan map[string]any, 1)
	service := &Service{Configuration: DefaultConfiguration()}
	service.Configuration.StateDirectory = t.TempDir()
	service.Configuration.BlueclawBaseURL = "http://blueclaw.test"
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://blueclaw.test/connectors/mattermost/events" || request.Method != http.MethodPost {
			t.Fatalf("unexpected forward request: %s %s", request.Method, request.URL.String())
		}
		var payload map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatalf("expected forward payload to decode: %v", errorValue)
		}
		forwardedRequests <- payload
		return jsonResponse(http.StatusOK, `{"handled":true}`, nil), nil
	})}
	token := service.ensureMattermostInteractiveActionToken()
	request := httptest.NewRequest(http.MethodPost, "/_internkim/mattermost/actions", strings.NewReader(`{"user_id":"user-1","post_id":"post-1","channel_id":"channel-1","context":{"action":"ask.confirm","token":"`+token+`","interactionID":"interaction-1","taskRunID":"task-1","conversationID":"channel-1","replyTargetID":"target-1"}}`))
	responseRecorder := httptest.NewRecorder()

	service.handleMattermostInteractiveAction(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected success response, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response mattermostInteractiveResponse
	if errorValue := json.Unmarshal(responseRecorder.Body.Bytes(), &response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.Update != nil {
		t.Fatalf("expected confirm response not to update original post, got %+v", response.Update)
	}
	select {
	case payload := <-forwardedRequests:
		eventDocument, isMap := payload["event"].(map[string]any)
		if !isMap || eventDocument["prompt"] != "approved" {
			t.Fatalf("expected confirm event to be forwarded, got %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("expected confirm action to be forwarded")
	}
}
