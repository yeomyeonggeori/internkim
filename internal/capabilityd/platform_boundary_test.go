package capabilityd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMattermostNormalizeSkipsSelfAndBuildsMinimalThreadEvent(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "hello",
		CreateAt:  1700000000000,
	}, "bot-1", "O", "town-square", false)
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected user post to become an event")
	}
	if event.ConversationID != "thread:channel-1:post-1" {
		t.Fatalf("expected top-level channel post to become thread conversation, got %q", event.ConversationID)
	}
	if event.SenderID != "user-1" {
		t.Fatalf("expected senderID, got %q", event.SenderID)
	}
	if event.Prompt != "hello" {
		t.Fatalf("expected prompt, got %q", event.Prompt)
	}
	if event.Context.HistoryCursor == "" {
		t.Fatal("expected history cursor")
	}
	if event.Context.ConversationType != "O" || event.Context.ChannelID != "channel-1" || event.Context.ChannelName != "town-square" {
		t.Fatalf("expected channel metadata in context, got %+v", event.Context)
	}

	replyHandle, errorValue := decodePlatformHandle(event.ReplyTargetID)
	if errorValue != nil {
		t.Fatalf("expected reply target to decode: %v", errorValue)
	}
	if replyHandle.RootID != "post-1" {
		t.Fatalf("expected channel reply root to be post-1, got %q", replyHandle.RootID)
	}

	_, hasSelfEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-2",
		UserID:    "bot-1",
		ChannelID: "channel-1",
		Message:   "self",
	}, "bot-1", "O", "town-square", false)
	if errorValue != nil {
		t.Fatalf("expected self normalization to be harmless: %v", errorValue)
	}
	if hasSelfEvent {
		t.Fatal("expected self message to be suppressed")
	}
}

func TestMattermostCompanionConnectCreatesOwnerPairingAndRepliesInDM(t *testing.T) {
	var pairingRequest companionConnectPairingRequest
	var postedMessage string
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://127.0.0.1:18080/_internkim/companion/pairing-codes":
			if errorValue := json.NewDecoder(request.Body).Decode(&pairingRequest); errorValue != nil {
				t.Fatal(errorValue)
			}
			return testJSONResponse(http.StatusOK, companionConnectPairingResponse{
				Code:      "ABCD-1234",
				ExpiresAt: time.Date(2026, 5, 6, 12, 30, 0, 0, time.UTC),
				DeepLink:  "internkim://pair?device_url=https%3A%2F%2Fdevice.example.com&code=ABCD-1234",
			}), nil
		case "https://mattermost.test/api/v4/posts":
			var body map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
				t.Fatal(errorValue)
			}
			postedMessage, _ = body["message"].(string)
			if body["channel_id"] != "dm-1" {
				t.Fatalf("expected DM channel post, got %+v", body)
			}
			return testJSONResponse(http.StatusCreated, map[string]string{"id": "reply-1"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{
			AdmindBaseURL:       "http://127.0.0.1:18080",
			MattermostBaseURL:   "https://mattermost.test",
			MattermostTokenPath: writePlatformTestFile(t, "mattermost-token"),
			DeviceIDPath:        writePlatformTestFile(t, "dc719d8e"),
		},
		HTTPClient: httpClient,
	}

	handled, errorValue := service.handleMattermostCompanionConnectCommand(context.Background(), platformInboundEvent{
		ConversationID: "dm:dm-1",
		SenderID:       "user-1",
		Prompt:         "connect",
		Context: platformEventContext{
			ConversationType: "D",
			ChannelID:        "dm-1",
			Sender: platformContextSender{
				Platform: "mattermost",
				UserID:   "user-1",
				Email:    "Alice@Example.com",
				Name:     "Alice",
			},
		},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !handled {
		t.Fatal("expected connect command to be handled")
	}
	if pairingRequest.OwnerPlatform != "mattermost" || pairingRequest.OwnerPlatformUserID != "user-1" || pairingRequest.OwnerEmail != "alice@example.com" || pairingRequest.OwnerName != "Alice" {
		t.Fatalf("unexpected pairing request: %+v", pairingRequest)
	}
	if pairingRequest.DeviceURL != "https://dc719d8e.example.test" {
		t.Fatalf("expected public device url, got %q", pairingRequest.DeviceURL)
	}
	if !strings.Contains(postedMessage, "ABCD-1234") || !strings.Contains(postedMessage, "[Companion 앱 열기](internkim://pair") {
		t.Fatalf("unexpected connect reply: %q", postedMessage)
	}
}

func TestMattermostCompanionConnectRecognizesSlashConnect(t *testing.T) {
	event := platformInboundEvent{
		Prompt: "/connect",
		Context: platformEventContext{
			ConversationType: "O",
		},
	}
	if !isMattermostCompanionConnectCommand(event) {
		t.Fatal("expected /connect to be recognized")
	}
}

func TestMattermostDirectMessageDoesNotUseThreadRoot(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "dm-1",
		Message:   "hello",
	}, "bot-1", "D", "", false)
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected direct message event")
	}
	if event.ConversationID != "dm:dm-1" {
		t.Fatalf("expected dm conversation, got %q", event.ConversationID)
	}
	if event.Context.ConversationType != "D" || event.Context.ChannelID != "dm-1" {
		t.Fatalf("expected direct message metadata in context, got %+v", event.Context)
	}
	replyHandle, errorValue := decodePlatformHandle(event.ReplyTargetID)
	if errorValue != nil {
		t.Fatalf("expected reply target to decode: %v", errorValue)
	}
	if replyHandle.RootID != "" {
		t.Fatalf("expected DM reply root to be empty, got %q", replyHandle.RootID)
	}
}

func TestMattermostDirectMessageThreadKeepsThreadRoot(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "reply-1",
		UserID:    "user-1",
		ChannelID: "dm-1",
		Message:   "thread reply",
		RootID:    "root-9",
	}, "bot-1", "D", "", false)
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected dm thread event")
	}
	if event.ConversationID != "thread:dm-1:root-9" {
		t.Fatalf("expected dm thread conversation, got %q", event.ConversationID)
	}
	replyHandle, errorValue := decodePlatformHandle(event.ReplyTargetID)
	if errorValue != nil {
		t.Fatalf("expected reply target to decode: %v", errorValue)
	}
	if replyHandle.RootID != "root-9" {
		t.Fatalf("expected dm thread reply root to be root-9, got %q", replyHandle.RootID)
	}
}

func TestMattermostChannelMessageWithoutMentionIsSkipped(t *testing.T) {
	_, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "casual chatter",
	}, "bot-1", "O", "random-chat", false)
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if hasEvent {
		t.Fatal("expected non-default channel without mention to be skipped")
	}
}

func TestMattermostChannelMessageWithMentionIsForwarded(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "@internkim hi",
	}, "bot-1", "O", "random-chat", true)
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected channel mention to produce an event")
	}
	if event.ConversationID != "thread:channel-1:post-1" {
		t.Fatalf("expected mention to start a thread conversation, got %q", event.ConversationID)
	}
}

func TestMattermostTownSquareForwardsWithoutMention(t *testing.T) {
	_, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "town square chatter",
	}, "bot-1", "O", "town-square", false)
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected town-square message to be forwarded without mention")
	}
}

func TestMattermostGroupMessageRequiresMention(t *testing.T) {
	_, hasEventWithoutMention, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "group-1",
		Message:   "hi all",
	}, "bot-1", "G", "", false)
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if hasEventWithoutMention {
		t.Fatal("expected group message without mention to be skipped")
	}

	_, hasEventWithMention, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-2",
		UserID:    "user-1",
		ChannelID: "group-1",
		Message:   "@internkim help",
	}, "bot-1", "G", "", true)
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEventWithMention {
		t.Fatal("expected group message with mention to be forwarded")
	}
}

func TestMattermostWebSocketPayloadGatesByMention(t *testing.T) {
	buildPayload := func(channelType string, channelName string, mentions string, message string) []byte {
		postDocument, _ := json.Marshal(mattermostPost{
			ID:        "post-1",
			UserID:    "user-1",
			ChannelID: "channel-1",
			Message:   message,
		})
		envelope := map[string]any{
			"event": "posted",
			"data": map[string]any{
				"channel_type": channelType,
				"channel_name": channelName,
				"mentions":     mentions,
				"post":         string(postDocument),
			},
		}
		document, _ := json.Marshal(envelope)
		return document
	}

	_, hasEvent, errorValue := normalizeMattermostWebSocketPayload(buildPayload("O", "random-chat", "", "no mention"), "bot-1")
	if errorValue != nil {
		t.Fatalf("expected payload normalization to succeed: %v", errorValue)
	}
	if hasEvent {
		t.Fatal("expected channel post without mention to be skipped")
	}

	_, hasEvent, errorValue = normalizeMattermostWebSocketPayload(buildPayload("O", "random-chat", `["bot-1"]`, "@internkim hi"), "bot-1")
	if errorValue != nil {
		t.Fatalf("expected mention payload normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected channel mention payload to be forwarded")
	}
}

func TestMattermostContextUsesSingleNameForHistorySpeakers(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/channels/channel-1/posts":
			return testJSONResponse(http.StatusOK, struct {
				Order []string                         `json:"order"`
				Posts map[string]mattermostHistoryPost `json:"posts"`
			}{
				Order: []string{"post-1", "post-2"},
				Posts: map[string]mattermostHistoryPost{
					"post-1": {ID: "post-1", UserID: "user-1", Message: "previous", CreateAt: 1000},
					"post-2": {ID: "post-2", UserID: "user-2", Message: "current", CreateAt: 2000},
				},
			}), nil
		case "/api/v4/users/user-1":
			return testJSONResponse(http.StatusOK, map[string]string{
				"id":         "user-1",
				"username":   "lee",
				"first_name": "서연",
				"last_name":  "이",
				"nickname":   "이서연",
			}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s?%s", request.URL.Path, request.URL.RawQuery)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	service := Service{
		Configuration: Configuration{MattermostBaseURL: "http://mattermost.test", MattermostTokenPath: tokenPath},
		HTTPClient:    httpClient,
	}
	contextValue := service.mattermostContext(context.Background(), platformHandle{ChannelID: "channel-1", MessageID: "post-2"}, 20)
	if len(contextValue.Messages) != 1 {
		t.Fatalf("expected one history message, got %+v", contextValue.Messages)
	}
	if contextValue.Messages[0].Speaker != "이서연" {
		t.Fatalf("expected single name speaker, got %q", contextValue.Messages[0].Speaker)
	}
}

func TestMattermostSenderFallsBackToSenderIDWhenProfileLookupFails(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v4/users/user-1" {
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
		}
		return testJSONResponse(http.StatusNotFound, map[string]string{"message": "not found"}), nil
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	service := Service{
		Configuration: Configuration{MattermostBaseURL: "http://mattermost.test", MattermostTokenPath: tokenPath},
		HTTPClient:    httpClient,
	}
	sender := service.mattermostSender(context.Background(), "user-1")
	if sender.Name != "user-1" || sender.UserID != "user-1" || sender.Platform != "mattermost" {
		t.Fatalf("expected sender fallback, got %+v", sender)
	}
}

func TestMattermostPollerSeedsWatermarkWithoutReplayingDirectMessage(t *testing.T) {
	forwardedCount := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "blueclaw.test" {
			forwardedCount++
			return handleTestBlueclawForward(t, request)
		}
		if request.URL.Host != "mattermost.test" {
			t.Fatalf("unexpected request host: %s", request.URL.Host)
		}
		return handleTestMattermostPoll(t, request), nil
	})}

	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}

	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	configuration.BlueclawBaseURL = "http://blueclaw.test"
	service := Service{Configuration: configuration, HTTPClient: httpClient}
	lastSeenByChannel := map[string]int64{}

	if errorValue := service.pollMattermost(context.Background(), lastSeenByChannel); errorValue != nil {
		t.Fatalf("expected first poll to seed watermark: %v", errorValue)
	}
	if forwardedCount != 0 {
		t.Fatalf("expected first poll not to replay old dm, got %d forwards", forwardedCount)
	}
	if lastSeenByChannel["dm-1"] != 1000 {
		t.Fatalf("expected dm watermark to seed at latest existing post, got %d", lastSeenByChannel["dm-1"])
	}

	if errorValue := service.pollMattermost(context.Background(), lastSeenByChannel); errorValue != nil {
		t.Fatalf("expected second poll to forward new message: %v", errorValue)
	}
	if forwardedCount != 1 {
		t.Fatalf("expected only new dm to be forwarded once, got %d forwards", forwardedCount)
	}
	if lastSeenByChannel["dm-1"] != 2000 {
		t.Fatalf("expected dm watermark to advance, got %d", lastSeenByChannel["dm-1"])
	}
}

func TestMattermostPollerDoesNotAdvanceWatermarkWhenBlueclawForwardFails(t *testing.T) {
	forwardedCount := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "blueclaw.test" {
			forwardedCount++
			return testJSONResponse(http.StatusServiceUnavailable, map[string]string{"error": "blueclaw down"}), nil
		}
		if request.URL.Host != "mattermost.test" {
			t.Fatalf("unexpected request host: %s", request.URL.Host)
		}
		return handleTestMattermostPoll(t, request), nil
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	configuration.BlueclawBaseURL = "http://blueclaw.test"
	service := Service{Configuration: configuration, HTTPClient: httpClient}
	lastSeenByChannel := map[string]int64{"dm-1": 1000}

	if errorValue := service.pollMattermost(context.Background(), lastSeenByChannel); errorValue != nil {
		t.Fatalf("expected failed forward to stay in poll loop: %v", errorValue)
	}
	if forwardedCount != 1 {
		t.Fatalf("expected one forward attempt, got %d", forwardedCount)
	}
	if lastSeenByChannel["dm-1"] != 1000 {
		t.Fatalf("expected watermark to remain at failed event, got %d", lastSeenByChannel["dm-1"])
	}
}

func TestMattermostProgressStartPublishesTypingUntilStopped(t *testing.T) {
	typingRequests := make(chan map[string]string, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]string{"id": "bot-1"}), nil
		case "/api/v4/users/bot-1/typing":
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected typing request to decode: %v", errorValue)
			}
			typingRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}

	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{
		Platform:  "mattermost",
		ChannelID: "channel-1",
		RootID:    "root-1",
	})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}

	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	service := Service{
		Configuration:   configuration,
		HTTPClient:      httpClient,
		ProgressManager: newPlatformProgressManager(),
	}

	requestBody := strings.NewReader(`{"replyTargetID":"` + replyTargetID + `"}`)
	_, errorValue = service.mattermostStartProgressFromRequest(context.Background(), requestBody)
	if errorValue != nil {
		t.Fatalf("expected progress start to succeed: %v", errorValue)
	}
	defer service.progressManager().Stop("mattermost:" + replyTargetID)

	select {
	case payload := <-typingRequests:
		if payload["channel_id"] != "channel-1" || payload["parent_id"] != "root-1" {
			t.Fatalf("unexpected typing payload: %+v", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected typing request")
	}

	_, errorValue = service.mattermostStopProgressFromRequest(context.Background(), strings.NewReader(`{"replyTargetID":"`+replyTargetID+`"}`))
	if errorValue != nil {
		t.Fatalf("expected progress stop to succeed: %v", errorValue)
	}
}

func TestMattermostReplyStopsProgressBeforeSendingPost(t *testing.T) {
	typingRequests := make(chan map[string]string, 1)
	postRequests := make(chan map[string]any, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]string{"id": "bot-1"}), nil
		case "/api/v4/users/bot-1/typing":
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected typing request to decode: %v", errorValue)
			}
			typingRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{}), nil
		case "/api/v4/posts":
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected post request to decode: %v", errorValue)
			}
			postRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{"id": "post-1"}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}

	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{
		Platform:  "mattermost",
		ChannelID: "channel-1",
		RootID:    "root-1",
	})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}

	progressManager := newPlatformProgressManager()
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	service := Service{
		Configuration:   configuration,
		HTTPClient:      httpClient,
		ProgressManager: progressManager,
	}

	_, errorValue = service.mattermostStartProgressFromRequest(context.Background(), strings.NewReader(`{"replyTargetID":"`+replyTargetID+`"}`))
	if errorValue != nil {
		t.Fatalf("expected progress start to succeed: %v", errorValue)
	}
	defer service.progressManager().Stop("mattermost:" + replyTargetID)

	select {
	case <-typingRequests:
	case <-time.After(2 * time.Second):
		t.Fatal("expected typing request")
	}

	_, errorValue = service.mattermostReply(context.Background(), json.RawMessage(`{"replyTargetID":"`+replyTargetID+`","message":"done","rawEventID":"raw-event-1","outboxID":"outbox-1"}`))
	if errorValue != nil {
		t.Fatalf("expected reply to succeed: %v", errorValue)
	}

	select {
	case payload := <-postRequests:
		if payload["channel_id"] != "channel-1" || payload["root_id"] != "root-1" || payload["message"] != "done" {
			t.Fatalf("unexpected post payload: %+v", payload)
		}
		props, isMap := payload["props"].(map[string]any)
		if !isMap || props["internkim_raw_event_id"] != "raw-event-1" || props["internkim_outbox_id"] != "outbox-1" {
			t.Fatalf("expected connector metadata props, got %+v", payload)
		}
	default:
		t.Fatal("expected post request")
	}
	progressManager.mutex.Lock()
	_, isActive := progressManager.leaseByKey["mattermost:"+replyTargetID]
	progressManager.mutex.Unlock()
	if isActive {
		t.Fatal("expected reply send to stop Mattermost progress")
	}
}

func TestMattermostReplyRequiresConnectorOutboxMetadata(t *testing.T) {
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ChannelID: "channel-1"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.MattermostTokenPath = t.TempDir() + "/missing-token"
	service := Service{Configuration: configuration}

	_, errorValue = service.mattermostReply(context.Background(), json.RawMessage(`{"replyTargetID":"`+replyTargetID+`","message":"done"}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "connector outbox metadata") {
		t.Fatalf("expected connector metadata error, got %v", errorValue)
	}
}

func TestMattermostProgressStopBeforeDebounceSuppressesTyping(t *testing.T) {
	typingRequests := make(chan map[string]string, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]string{"id": "bot-1"}), nil
		case "/api/v4/users/bot-1/typing":
			typingRequests <- map[string]string{}
			return testJSONResponse(http.StatusOK, map[string]string{}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{
		Platform:  "mattermost",
		ChannelID: "channel-1",
	})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	service := Service{
		Configuration: Configuration{
			MattermostBaseURL:   "http://mattermost.test",
			MattermostTokenPath: tokenPath,
		},
		HTTPClient:      httpClient,
		ProgressManager: newPlatformProgressManager(),
	}

	_, errorValue = service.mattermostStartProgressFromRequest(context.Background(), strings.NewReader(`{"replyTargetID":"`+replyTargetID+`"}`))
	if errorValue != nil {
		t.Fatalf("expected progress start to succeed: %v", errorValue)
	}
	_, errorValue = service.mattermostStopProgressFromRequest(context.Background(), strings.NewReader(`{"replyTargetID":"`+replyTargetID+`"}`))
	if errorValue != nil {
		t.Fatalf("expected progress stop to succeed: %v", errorValue)
	}

	select {
	case <-typingRequests:
		t.Fatal("expected no typing request before debounce")
	case <-time.After(mattermostTypingDebounce + 200*time.Millisecond):
	}
}

func TestPlatformProgressManagerExpiresLeases(t *testing.T) {
	manager := newPlatformProgressManager()
	stopped := make(chan struct{}, 1)

	manager.Start("test", 20*time.Millisecond, func(ctx context.Context) {
		<-ctx.Done()
		stopped <- struct{}{}
	})

	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("expected progress lease to expire")
	}
	manager.mutex.Lock()
	_, isActive := manager.leaseByKey["test"]
	manager.mutex.Unlock()
	if isActive {
		t.Fatal("expected expired lease to be removed")
	}
}

func TestPlatformProgressManagerRefreshesLeaseWithoutDuplicateLoop(t *testing.T) {
	manager := newPlatformProgressManager()
	started := make(chan struct{}, 2)
	stopped := make(chan struct{}, 1)

	manager.Start("test", 50*time.Millisecond, func(ctx context.Context) {
		started <- struct{}{}
		<-ctx.Done()
		stopped <- struct{}{}
	})
	manager.Start("test", 150*time.Millisecond, func(ctx context.Context) {
		started <- struct{}{}
		<-ctx.Done()
		stopped <- struct{}{}
	})

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("expected progress loop to start")
	}
	select {
	case <-started:
		t.Fatal("expected existing lease refresh without duplicate loop")
	case <-time.After(20 * time.Millisecond):
	}
	select {
	case <-stopped:
		t.Fatal("expected refreshed lease to remain active after original ttl")
	case <-time.After(80 * time.Millisecond):
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("expected refreshed lease to expire")
	}
}

func TestMattermostReplyUploadsAttachmentsAndPostsFileIDs(t *testing.T) {
	companionDirectory := t.TempDir()
	attachmentPath := companionDirectory + "/screen.png"
	if errorValue := os.WriteFile(attachmentPath, []byte("png"), 0o600); errorValue != nil {
		t.Fatalf("expected attachment file: %v", errorValue)
	}
	uploadRequests := make(chan string, 1)
	postRequests := make(chan map[string]any, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/files":
			if errorValue := request.ParseMultipartForm(1024); errorValue != nil {
				t.Fatalf("expected multipart upload: %v", errorValue)
			}
			files := request.MultipartForm.File["files"]
			if request.MultipartForm.Value["channel_id"][0] != "channel-1" || len(files) != 1 || files[0].Filename != "screen.png" {
				t.Fatalf("unexpected upload form: %+v", request.MultipartForm)
			}
			uploadRequests <- files[0].Filename
			return testJSONResponse(http.StatusOK, map[string]any{"file_infos": []map[string]string{{"id": "file-1"}}}), nil
		case "/api/v4/posts":
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected post request to decode: %v", errorValue)
			}
			postRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{"id": "post-1"}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file: %v", errorValue)
	}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ChannelID: "channel-1"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	configuration.CompanionFileDirectory = companionDirectory
	service := Service{Configuration: configuration, HTTPClient: httpClient}

	_, errorValue = service.mattermostReply(context.Background(), mustJSON(t, replyRequest{
		ReplyTargetID: replyTargetID,
		Message:       "captured",
		RawEventID:    "raw-event-1",
		OutboxID:      "outbox-1",
		Attachments:   []platformFileSpec{{DevicePath: attachmentPath, Filename: "screen.png", ContentType: "image/png", SizeBytes: 3}},
	}))
	if errorValue != nil {
		t.Fatalf("expected reply with attachment: %v", errorValue)
	}

	select {
	case <-uploadRequests:
	default:
		t.Fatal("expected file upload request")
	}
	select {
	case payload := <-postRequests:
		fileIDs, isArray := payload["file_ids"].([]any)
		if !isArray || len(fileIDs) != 1 || fileIDs[0] != "file-1" {
			t.Fatalf("expected file ids in post payload, got %+v", payload)
		}
		props, isMap := payload["props"].(map[string]any)
		if !isMap || props["internkim_outbox_id"] != "outbox-1" {
			t.Fatalf("expected connector outbox metadata, got %+v", payload)
		}
	default:
		t.Fatal("expected post request")
	}
}

func TestMattermostReplyUploadsInlineAttachmentsFromBlueclawWorkspace(t *testing.T) {
	companionDirectory := t.TempDir()
	uploadRequests := make(chan string, 1)
	postRequests := make(chan map[string]any, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/files":
			if errorValue := request.ParseMultipartForm(1024); errorValue != nil {
				t.Fatalf("expected multipart upload: %v", errorValue)
			}
			files := request.MultipartForm.File["files"]
			if len(files) != 1 || files[0].Filename != "deck.pptx" {
				t.Fatalf("unexpected upload form: %+v", request.MultipartForm)
			}
			uploadRequests <- files[0].Filename
			return testJSONResponse(http.StatusOK, map[string]any{"file_infos": []map[string]string{{"id": "file-1"}}}), nil
		case "/api/v4/posts":
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected post request to decode: %v", errorValue)
			}
			postRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{"id": "post-1"}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file: %v", errorValue)
	}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ChannelID: "channel-1"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	configuration.CompanionFileDirectory = companionDirectory
	service := Service{Configuration: configuration, HTTPClient: httpClient}

	_, errorValue = service.mattermostReply(context.Background(), mustJSON(t, replyRequest{
		ReplyTargetID: replyTargetID,
		Message:       "deck",
		RawEventID:    "raw-event-1",
		OutboxID:      "outbox-1",
		Attachments: []platformFileSpec{{
			DevicePath:    "/workspace/deck.pptx",
			Filename:      "deck.pptx",
			ContentType:   "application/vnd.openxmlformats-officedocument.presentationml.presentation",
			SizeBytes:     4,
			ContentBase64: base64.StdEncoding.EncodeToString([]byte("pptx")),
		}},
	}))
	if errorValue != nil {
		t.Fatalf("expected inline attachment upload: %v", errorValue)
	}

	select {
	case <-uploadRequests:
	default:
		t.Fatal("expected file upload request")
	}
	select {
	case payload := <-postRequests:
		fileIDs, isArray := payload["file_ids"].([]any)
		if !isArray || len(fileIDs) != 1 || fileIDs[0] != "file-1" {
			t.Fatalf("expected file ids in post payload, got %+v", payload)
		}
	default:
		t.Fatal("expected post request")
	}
}

func TestPlatformAttachmentRejectsOutsideDevicePath(t *testing.T) {
	companionDirectory := t.TempDir()
	outsidePath := t.TempDir() + "/secret.txt"
	if errorValue := os.WriteFile(outsidePath, []byte("secret"), 0o600); errorValue != nil {
		t.Fatalf("expected outside file: %v", errorValue)
	}
	service := Service{Configuration: Configuration{CompanionFileDirectory: companionDirectory}}

	_, errorValue := service.validatePlatformFiles([]platformFileSpec{{DevicePath: outsidePath}})
	if errorValue == nil {
		t.Fatal("expected outside attachment path to fail")
	}
}

func TestSlackReplyUploadsAttachmentsWithExternalFlow(t *testing.T) {
	companionDirectory := t.TempDir()
	attachmentPath := companionDirectory + "/screen.png"
	if errorValue := os.WriteFile(attachmentPath, []byte("png"), 0o600); errorValue != nil {
		t.Fatalf("expected attachment file: %v", errorValue)
	}
	requestPaths := []string{}
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestPaths = append(requestPaths, request.URL.Path)
		switch request.URL.Path {
		case "/api/files.getUploadURLExternal":
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected upload url request: %v", errorValue)
			}
			if payload["filename"] != "screen.png" || payload["length"].(float64) != 3 {
				t.Fatalf("unexpected upload url payload: %+v", payload)
			}
			return testJSONResponse(http.StatusOK, map[string]any{"ok": true, "upload_url": "https://upload.slack.test/file", "file_id": "file-1"}), nil
		case "/file":
			document, _ := io.ReadAll(request.Body)
			if string(document) != "png" {
				t.Fatalf("unexpected upload bytes: %q", string(document))
			}
			return testJSONResponse(http.StatusOK, map[string]any{}), nil
		case "/api/files.completeUploadExternal":
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected complete request: %v", errorValue)
			}
			if payload["channel_id"] != "channel-1" || payload["initial_comment"] != "captured" {
				t.Fatalf("unexpected complete payload: %+v", payload)
			}
			return testJSONResponse(http.StatusOK, map[string]any{"ok": true, "files": []map[string]string{{"id": "file-1"}}}), nil
		default:
			t.Fatalf("unexpected Slack request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	tokenPath := t.TempDir() + "/slack-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file: %v", errorValue)
	}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "slack", ChannelID: "channel-1", ThreadTimestamp: "123.456"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.SlackTokenPath = tokenPath
	configuration.CompanionFileDirectory = companionDirectory
	service := Service{Configuration: configuration, HTTPClient: httpClient}

	_, errorValue = service.slackReply(context.Background(), mustJSON(t, replyRequest{
		ReplyTargetID: replyTargetID,
		Message:       "captured",
		Attachments:   []platformFileSpec{{DevicePath: attachmentPath, Filename: "screen.png", SizeBytes: 3}},
	}))
	if errorValue != nil {
		t.Fatalf("expected Slack reply with attachment: %v", errorValue)
	}
	for _, path := range requestPaths {
		if path == "/api/files.upload" {
			t.Fatal("expected Slack external upload flow, not files.upload")
		}
	}
}

func TestSignalReplySendsAttachmentPaths(t *testing.T) {
	companionDirectory := t.TempDir()
	attachmentPath := companionDirectory + "/screen.png"
	if errorValue := os.WriteFile(attachmentPath, []byte("png"), 0o600); errorValue != nil {
		t.Fatalf("expected attachment file: %v", errorValue)
	}
	receivedParams := make(chan map[string]any, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload signalJSONRPCRequest
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatalf("expected signal jsonrpc request: %v", errorValue)
		}
		params, isMap := payload.Params.(map[string]any)
		if !isMap {
			t.Fatalf("expected map params, got %+v", payload.Params)
		}
		receivedParams <- params
		return testJSONResponse(http.StatusOK, map[string]any{"result": "message-1"}), nil
	})}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "signal", SignalRecipient: "+15557654321"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.SignalJSONRPCURL = "http://signal.test"
	configuration.SignalAccount = "+15551234567"
	configuration.CompanionFileDirectory = companionDirectory
	service := Service{Configuration: configuration, HTTPClient: httpClient}

	_, errorValue = service.signalReplyFromRequest(context.Background(), bytes.NewReader(mustJSON(t, replyRequest{
		ReplyTargetID: replyTargetID,
		Message:       "captured",
		Attachments:   []platformFileSpec{{DevicePath: attachmentPath, Filename: "screen.png", SizeBytes: 3}},
	})))
	if errorValue != nil {
		t.Fatalf("expected signal reply with attachment: %v", errorValue)
	}
	select {
	case params := <-receivedParams:
		attachments, isArray := params["attachments"].([]any)
		if !isArray || len(attachments) != 1 || attachments[0] != attachmentPath {
			t.Fatalf("expected signal attachment path, got %+v", params)
		}
	default:
		t.Fatal("expected signal send request")
	}
}

func handleTestBlueclawForward(t *testing.T, request *http.Request) (*http.Response, error) {
	t.Helper()
	var event platformInboundEvent
	if errorValue := json.NewDecoder(request.Body).Decode(&event); errorValue != nil {
		return testJSONResponse(http.StatusBadRequest, map[string]string{"error": errorValue.Error()}), nil
	}
	if event.Prompt != "new dm" {
		t.Fatalf("expected only new dm to be forwarded, got %q", event.Prompt)
	}
	if event.Context.Sender.Name != "이서연" || event.Context.Sender.CallingName != "서연" || event.Context.Sender.Handle != "seoyeon" || event.Context.Sender.Email != "seoyeon@example.com" {
		t.Fatalf("expected sender identity, got %+v", event.Context.Sender)
	}
	if _, errorValue := time.Parse(time.RFC3339, event.Context.ReceivedAt); errorValue != nil {
		t.Fatalf("expected receivedAt RFC3339 timestamp, got %q", event.Context.ReceivedAt)
	}
	return testJSONResponse(http.StatusOK, map[string]string{}), nil
}

func handleTestMattermostPoll(t *testing.T, request *http.Request) *http.Response {
	t.Helper()
	switch {
	case request.URL.Path == "/api/v4/users/me":
		return testJSONResponse(http.StatusOK, map[string]string{"id": "bot-1", "username": "internkim"})
	case request.URL.Path == "/api/v4/users/bot-1/channels":
		return testJSONResponse(http.StatusOK, []map[string]string{{"id": "dm-1", "type": "D", "name": "user-1__bot-1"}})
	case request.URL.Path == "/api/v4/channels/dm-1/posts" && request.URL.Query().Get("per_page") == "1":
		return testJSONResponse(http.StatusOK, mattermostPostsResponse{
			Order: []string{"old-1"},
			Posts: map[string]mattermostPolledPost{
				"old-1": {ID: "old-1", UserID: "user-1", ChannelID: "dm-1", Message: "old dm", CreateAt: 1000},
			},
		})
	case request.URL.Path == "/api/v4/channels/dm-1/posts" && request.URL.Query().Get("since") == "1000":
		return testJSONResponse(http.StatusOK, mattermostPostsResponse{
			Order: []string{"new-1", "old-1"},
			Posts: map[string]mattermostPolledPost{
				"old-1": {ID: "old-1", UserID: "user-1", ChannelID: "dm-1", Message: "old dm", CreateAt: 1000},
				"new-1": {ID: "new-1", UserID: "user-1", ChannelID: "dm-1", Message: "new dm", CreateAt: 2000},
			},
		})
	case request.URL.Path == "/api/v4/channels/dm-1/posts" && request.URL.Query().Get("per_page") == "21":
		return testJSONResponse(http.StatusOK, mattermostPostsResponse{
			Order: []string{"new-1"},
			Posts: map[string]mattermostPolledPost{
				"new-1": {ID: "new-1", UserID: "user-1", ChannelID: "dm-1", Message: "new dm", CreateAt: 2000},
			},
		})
	case request.URL.Path == "/api/v4/users/user-1":
		return testJSONResponse(http.StatusOK, map[string]string{
			"id":         "user-1",
			"email":      "seoyeon@example.com",
			"username":   "seoyeon",
			"first_name": "서연",
			"last_name":  "이",
			"nickname":   "이서연",
		})
	default:
		t.Fatalf("unexpected Mattermost request: %s?%s", request.URL.Path, request.URL.RawQuery)
		return testJSONResponse(http.StatusNotFound, map[string]string{})
	}
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	document, errorValue := json.Marshal(value)
	if errorValue != nil {
		t.Fatalf("expected json marshal: %v", errorValue)
	}
	return document
}

func testJSONResponse(statusCode int, response any) *http.Response {
	var responseBody bytes.Buffer
	_ = json.NewEncoder(&responseBody).Encode(response)
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(&responseBody),
	}
}

func writePlatformTestFile(t *testing.T, value string) string {
	t.Helper()
	path := t.TempDir() + "/secret"
	if errorValue := os.WriteFile(path, []byte(value), 0600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestSlackNormalizeUsesThreadForChannelAndNoThreadForDM(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	channelEnvelope := slackSocketEnvelope{}
	channelEnvelope.Payload.Event.Type = "message"
	channelEnvelope.Payload.Event.User = "user-1"
	channelEnvelope.Payload.Event.Channel = "channel-1"
	channelEnvelope.Payload.Event.ChannelType = "channel"
	channelEnvelope.Payload.Event.Text = "hello"
	channelEnvelope.Payload.Event.TS = "1700000000.000100"

	channelEvent, hasEvent, errorValue := service.normalizeSlackEnvelope(context.Background(), channelEnvelope, "bot-1")
	if errorValue != nil {
		t.Fatalf("expected channel normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected channel event")
	}
	channelHandle, errorValue := decodePlatformHandle(channelEvent.ReplyTargetID)
	if errorValue != nil {
		t.Fatalf("expected channel reply target to decode: %v", errorValue)
	}
	if channelHandle.ThreadTimestamp != channelEnvelope.Payload.Event.TS {
		t.Fatalf("expected channel reply thread timestamp, got %q", channelHandle.ThreadTimestamp)
	}

	directEnvelope := channelEnvelope
	directEnvelope.Payload.Event.Channel = "dm-1"
	directEnvelope.Payload.Event.ChannelType = "im"
	directEnvelope.Payload.Event.TS = "1700000000.000200"
	directEvent, hasEvent, errorValue := service.normalizeSlackEnvelope(context.Background(), directEnvelope, "bot-1")
	if errorValue != nil {
		t.Fatalf("expected dm normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected dm event")
	}
	directHandle, errorValue := decodePlatformHandle(directEvent.ReplyTargetID)
	if errorValue != nil {
		t.Fatalf("expected dm reply target to decode: %v", errorValue)
	}
	if directHandle.ThreadTimestamp != "" {
		t.Fatalf("expected dm thread timestamp to be empty, got %q", directHandle.ThreadTimestamp)
	}
}

func TestRouterDoesNotRegisterDeprecatedPlatformEndpoints(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	for _, path := range []string{
		"/v1/platform/mattermost/bot.resolve",
		"/v1/platform/mattermost/conversation.kind",
		"/v1/platform/mattermost/typing.publish",
	} {
		request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
		responseRecorder := httptest.NewRecorder()
		service.router().ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != http.StatusNotFound {
			t.Fatalf("expected %s to be unregistered, got %d", path, responseRecorder.Code)
		}
	}
}

func TestIdentityResolveRequiresSenderID(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	request := httptest.NewRequest(http.MethodPost, "/v1/platform/signal/identity.resolve", strings.NewReader(`{"externalUserID":"legacy"}`))
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected missing senderID to fail, got %d", responseRecorder.Code)
	}
}

func TestForwardedPlatformEventDoesNotLeakLegacyFields(t *testing.T) {
	document, errorValue := json.Marshal(platformInboundEvent{
		ConversationID: "thread:channel-1:post-1",
		MessageID:      "post-1",
		SenderID:       "user-1",
		ReplyTargetID:  "reply",
		Prompt:         "hello",
		Context:        platformEventContext{Messages: []platformContextMessage{{Speaker: "admin", Text: "previous"}}},
	})
	if errorValue != nil {
		t.Fatalf("expected event to marshal: %v", errorValue)
	}
	var requestBody map[string]any
	if errorValue := json.Unmarshal(document, &requestBody); errorValue != nil {
		t.Fatalf("expected request body to decode: %v", errorValue)
	}
	forbiddenKeys := []string{"platform", "source", "eventID", "channelType", "rootID", "thread_ts", "post_id", "isBotMessage", "senderUserID", "text"}
	for _, key := range forbiddenKeys {
		if _, isFound := requestBody[key]; isFound {
			t.Fatalf("expected %q to be absent from forwarded body: %v", key, requestBody)
		}
	}
	if requestBody["prompt"] != "hello" {
		t.Fatalf("expected prompt in forwarded body, got %v", requestBody)
	}
}

func TestSignalNormalizeBuildsMinimalEvent(t *testing.T) {
	service := Service{Configuration: Configuration{SignalAccount: "+15551234567"}}
	envelope := signalReceiveEnvelope{}
	envelope.Envelope.Source = "+15557654321"
	envelope.Envelope.SourceName = "Lee"
	envelope.Envelope.Timestamp = 1700000000000
	envelope.Envelope.DataMessage.Message = "hello"

	event, hasEvent, errorValue := service.normalizeSignalEnvelope(envelope)
	if errorValue != nil {
		t.Fatalf("expected signal normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected signal event")
	}
	if event.ConversationID != "dm:+15557654321" || event.SenderID != "+15557654321" || event.Prompt != "hello" {
		t.Fatalf("unexpected signal event: %+v", event)
	}
	if event.ReplyTargetID == "" {
		t.Fatal("expected signal reply target")
	}
}

func TestHistoryFetchRejectsMissingCursor(t *testing.T) {
	service := Service{Configuration: DefaultConfiguration()}
	request := httptest.NewRequest(http.MethodPost, "/v1/platform/mattermost/history.fetch", bytes.NewReader([]byte(`{"limit":20}`)))
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected missing cursor to fail, got %d", responseRecorder.Code)
	}
}
