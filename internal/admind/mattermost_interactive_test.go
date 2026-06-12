package admind

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMattermostAskActionAcknowledgesWithResolvedUpdateAndForwardsEvent(t *testing.T) {
	forwardedRequests := make(chan map[string]any, 1)
	deletedPosts := make(chan string, 1)
	service := &Service{Configuration: DefaultConfiguration()}
	service.Configuration.StateDirectory = t.TempDir()
	service.Configuration.BlueclawBaseURL = "http://blueclaw.test"
	service.Configuration.MattermostBaseURL = "http://mattermost.test"
	service.Configuration.MattermostBotTokenPath = writeTestFile(t, "bot-token")
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() == "http://mattermost.test/api/v4/posts/post-1" && request.Method == http.MethodDelete {
			deletedPosts <- "post-1"
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
	assertMattermostAskResolvedUpdate(t, response.Update, "post-1", "channel-1", "취소")
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
	case postID := <-deletedPosts:
		if postID != "post-1" {
			t.Fatalf("deleted post = %q", postID)
		}
	default:
		t.Fatal("expected ask control post to be deleted")
	}
}

func TestMattermostAskActionResolvedUpdates(t *testing.T) {
	testCases := []struct {
		name            string
		contextDocument string
		selectedOption  string
		expectedMessage string
	}{
		{
			name:            "confirm",
			contextDocument: `"action":"ask.confirm"`,
			expectedMessage: "확인",
		},
		{
			name:            "cancel",
			contextDocument: `"action":"ask.cancel"`,
			expectedMessage: "취소",
		},
		{
			name:            "choice label",
			contextDocument: `"action":"ask.choice","choiceKey":"A","choiceLabel":"첫 번째 선택지"`,
			expectedMessage: "첫 번째 선택지",
		},
		{
			name:            "choice selected option label",
			contextDocument: `"action":"ask.choice"`,
			expectedMessage: "둘째",
			selectedOption:  `{"key":"B","label":"둘째"}`,
		},
		{
			name:            "choice key",
			contextDocument: `"action":"ask.choice","choiceKey":"B"`,
			expectedMessage: "B",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			service := &Service{Configuration: DefaultConfiguration()}
			service.Configuration.StateDirectory = t.TempDir()
			service.Configuration.BlueclawBaseURL = "http://blueclaw.test"
			service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return jsonResponse(http.StatusOK, `{"handled":true}`, nil), nil
			})}
			token := service.ensureMattermostInteractiveActionToken()
			selectedOptionDocument := ""
			if testCase.selectedOption != "" {
				selectedOptionDocument = `,"selected_option":` + strconv.Quote(testCase.selectedOption)
			}
			requestBody := `{"user_id":"user-1","post_id":"post-1","channel_id":"channel-1"` + selectedOptionDocument + `,"context":{` + testCase.contextDocument + `,"token":"` + token + `","interactionID":"interaction-1","taskRunID":"task-1","conversationID":"channel-1","replyTargetID":"target-1","targetUserID":"user-1"}}`
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
			assertMattermostAskResolvedUpdate(t, response.Update, "post-1", "channel-1", testCase.expectedMessage)
		})
	}
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
	if response.Error != nil {
		t.Fatalf("expected configured token to pass validation, got %+v", response.Error)
	}
	assertMattermostAskResolvedUpdate(t, response.Update, "post-1", "channel-1", "확인")
}

func assertMattermostAskResolvedUpdate(t *testing.T, update any, postID string, channelID string, expectedMessage string) {
	t.Helper()
	updateDocument, isMap := update.(map[string]any)
	if !isMap {
		t.Fatalf("expected resolved post update, got %+v", update)
	}
	if updateDocument["id"] != postID || updateDocument["channel_id"] != channelID {
		t.Fatalf("expected post update target, got %+v", updateDocument)
	}
	if updateDocument["message"] != expectedMessage {
		t.Fatalf("expected resolved update message %q, got %+v", expectedMessage, updateDocument)
	}
	if _, isFound := updateDocument["delete_at"]; isFound {
		t.Fatalf("expected no delete_at in update, got %+v", updateDocument)
	}
	properties, isMap := updateDocument["props"].(map[string]any)
	if !isMap {
		t.Fatalf("expected props in update, got %+v", updateDocument)
	}
	attachments, isSlice := properties["attachments"].([]any)
	if !isSlice || len(attachments) != 0 {
		t.Fatalf("expected attachments to be cleared, got %+v", properties)
	}
}
