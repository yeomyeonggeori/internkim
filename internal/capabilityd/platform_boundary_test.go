package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

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

func TestSlackReplyUploadsAttachmentsWithExternalTask(t *testing.T) {
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
	request := httptest.NewRequest(http.MethodPost, "/v1/platform/slack/history.fetch", bytes.NewReader([]byte(`{"limit":20}`)))
	responseRecorder := httptest.NewRecorder()
	service.router().ServeHTTP(responseRecorder, request)
	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected missing cursor to fail, got %d", responseRecorder.Code)
	}
}
