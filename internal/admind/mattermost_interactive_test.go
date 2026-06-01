package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMattermostAskActionClearsButtonsAndForwardsEvent(t *testing.T) {
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
	update, isMap := response.Update.(map[string]any)
	if !isMap {
		t.Fatalf("expected immediate response to update original post, got %+v", response.Update)
	}
	props, isMap := update["props"].(map[string]any)
	if !isMap {
		t.Fatalf("expected update props, got %+v", update)
	}
	attachments, isArray := props["attachments"].([]any)
	if !isArray || len(attachments) != 0 {
		t.Fatalf("expected ask action to clear buttons, got %+v", update)
	}
	select {
	case payload := <-forwardedRequests:
		contextDocument := payload["context"].(map[string]any)
		if contextDocument["action"] != "ask.cancel" || contextDocument["taskRunID"] != "task-1" {
			t.Fatalf("expected ask action to be forwarded, got %+v", payload)
		}
	case <-time.After(time.Second):
		t.Fatal("expected ask action to be forwarded")
	}
}
