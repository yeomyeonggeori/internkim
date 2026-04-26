package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMattermostNormalizeSkipsSelfAndBuildsMinimalThreadEvent(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "hello",
		CreateAt:  1700000000000,
	}, "bot-1", "O")
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
	}, "bot-1", "O")
	if errorValue != nil {
		t.Fatalf("expected self normalization to be harmless: %v", errorValue)
	}
	if hasSelfEvent {
		t.Fatal("expected self message to be suppressed")
	}
}

func TestMattermostDirectMessageDoesNotUseThreadRoot(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "dm-1",
		Message:   "hello",
	}, "bot-1", "D")
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected direct message event")
	}
	if event.ConversationID != "dm:dm-1" {
		t.Fatalf("expected dm conversation, got %q", event.ConversationID)
	}
	replyHandle, errorValue := decodePlatformHandle(event.ReplyTargetID)
	if errorValue != nil {
		t.Fatalf("expected reply target to decode: %v", errorValue)
	}
	if replyHandle.RootID != "" {
		t.Fatalf("expected DM reply root to be empty, got %q", replyHandle.RootID)
	}
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
