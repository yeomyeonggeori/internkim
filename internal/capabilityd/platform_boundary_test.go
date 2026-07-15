package capabilityd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/mattermostdefaults"
)

const testFutureMattermostPostCreateAt = int64(4102444800000)

func TestMattermostNormalizeSkipsSelfAndBuildsMinimalThreadEvent(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "hello",
		CreateAt:  1700000000000,
	}, "bot-1", "O", "town-square", platformAddressing{})
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
	}, "bot-1", "O", "town-square", platformAddressing{})
	if errorValue != nil {
		t.Fatalf("expected self normalization to be harmless: %v", errorValue)
	}
	if hasSelfEvent {
		t.Fatal("expected self message to be suppressed")
	}
}

func TestMattermostNormalizePreservesInputAttachmentFileIDs(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		FileIDs:   []string{"file-1", " ", "file-2"},
		CreateAt:  1700000000000,
	}, "bot-1", "D", "direct-message", platformAddressing{})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected attachment-only post to become an event")
	}
	if event.Prompt == "" {
		t.Fatal("expected attachment-only post to have a prompt")
	}
	if len(event.Context.InputAttachments) != 2 {
		t.Fatalf("expected two input attachments, got %+v", event.Context.InputAttachments)
	}
	if event.Context.InputAttachments[0].FileID != "file-1" || event.Context.InputAttachments[1].FileID != "file-2" {
		t.Fatalf("expected file ids to be preserved, got %+v", event.Context.InputAttachments)
	}
	if event.Context.InputAttachments[0].MessageID != "post-1" || event.Context.InputAttachments[0].Platform != "mattermost" {
		t.Fatalf("expected attachment reference metadata, got %+v", event.Context.InputAttachments[0])
	}
}

func TestMattermostNormalizeMarksAttachmentsOnly(t *testing.T) {
	attachmentEvent, _, errorValue := normalizeMattermostPost(mattermostPost{
		ID: "post-1", UserID: "user-1", ChannelID: "channel-1", FileIDs: []string{"file-1"}, CreateAt: 1700000000000,
	}, "bot-1", "O", "open", platformAddressing{})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !attachmentEvent.Context.AttachmentsOnly {
		t.Fatal("text-less attachment post must be marked attachmentsOnly")
	}

	captionedEvent, _, errorValue := normalizeMattermostPost(mattermostPost{
		ID: "post-2", UserID: "user-1", ChannelID: "channel-1", Message: "이거 봐줘", FileIDs: []string{"file-1"}, CreateAt: 1700000000000,
	}, "bot-1", "O", "open", platformAddressing{})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if captionedEvent.Context.AttachmentsOnly {
		t.Fatal("attachment post with caption text must not be attachmentsOnly")
	}
}

func TestMattermostCompanionRecoverySendsChannelInstructionByDM(t *testing.T) {
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{
		Platform:       "mattermost",
		ConversationID: "thread:channel-1:post-1",
		ChannelID:      "channel-1",
		ChannelType:    "O",
		RootID:         "post-1",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var mainMessage string
	var directMessage string
	var pairingRequest companionConnectPairingRequest
	service := mattermostRecoveryTestService(t, func(request *http.Request) (*http.Response, error) {
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
		case "https://mattermost.test/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
		case "https://mattermost.test/api/v4/channels/direct":
			var body []string
			if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
				t.Fatal(errorValue)
			}
			if !slices.Contains(body, "user-1") || !slices.Contains(body, "bot-1") {
				t.Fatalf("expected DM members, got %+v", body)
			}
			return testJSONResponse(http.StatusCreated, map[string]string{"id": "dm-1"}), nil
		case "https://mattermost.test/api/v4/posts":
			var body map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
				t.Fatal(errorValue)
			}
			if body["channel_id"] == "dm-1" {
				directMessage, _ = body["message"].(string)
				return testJSONResponse(http.StatusCreated, map[string]string{"id": "dm-post-1"}), nil
			}
			mainMessage, _ = body["message"].(string)
			return testJSONResponse(http.StatusCreated, map[string]string{"id": "channel-post-1"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	_, errorValue = service.mattermostReply(context.Background(), mustJSONRaw(t, map[string]any{
		"replyTargetID": replyTargetID,
		"message":       "Companion이 연결되어 있지 않아 브라우저를 열 수 없습니다.",
		"rawEventID":    "raw-1",
		"outboxID":      "outbox-1",
		"recoveryActions": []capabilities.RecoveryAction{{
			Kind:           "companion_connect",
			Delivery:       "dm_preferred",
			DownloadURL:    capabilities.CompanionMacOSBetaDownloadURL(),
			ConnectCommand: "/connect",
			PlatformUserID: "user-1",
		}},
	}))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if mainMessage != "Companion 연결 안내를 DM으로 보냈어요." {
		t.Fatalf("expected channel-safe main message, got %q", mainMessage)
	}
	if pairingRequest.OwnerPlatformUserID != "user-1" {
		t.Fatalf("expected requester pairing owner, got %+v", pairingRequest)
	}
	if !strings.Contains(directMessage, "internkim-companion-beta-macos-aarch64.dmg") || !strings.Contains(directMessage, "ABCD-1234") || !strings.Contains(directMessage, "/connect") {
		t.Fatalf("expected DM recovery guide, got %q", directMessage)
	}
}

func TestMattermostCompanionRecoveryStaysInDirectMessage(t *testing.T) {
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{
		Platform:       "mattermost",
		ConversationID: "dm:dm-1",
		ChannelID:      "dm-1",
		ChannelType:    "D",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var postedMessage string
	service := mattermostRecoveryTestService(t, func(request *http.Request) (*http.Response, error) {
		switch request.URL.String() {
		case "http://127.0.0.1:18080/_internkim/companion/pairing-codes":
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
			return testJSONResponse(http.StatusCreated, map[string]string{"id": "post-1"}), nil
		default:
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.String())
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})

	_, errorValue = service.mattermostReply(context.Background(), mustJSONRaw(t, map[string]any{
		"replyTargetID": replyTargetID,
		"message":       "Companion이 연결되어 있지 않아 브라우저를 열 수 없습니다.",
		"rawEventID":    "raw-1",
		"outboxID":      "outbox-1",
		"recoveryActions": []capabilities.RecoveryAction{{
			Kind:           "companion_connect",
			Delivery:       "dm_preferred",
			DownloadURL:    capabilities.CompanionMacOSBetaDownloadURL(),
			ConnectCommand: "/connect",
			PlatformUserID: "user-1",
		}},
	}))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(postedMessage, "Companion 앱 다운로드") || !strings.Contains(postedMessage, "ABCD-1234") {
		t.Fatalf("expected direct recovery guide, got %q", postedMessage)
	}
}

func mattermostRecoveryTestService(t *testing.T, roundTrip func(*http.Request) (*http.Response, error)) Service {
	t.Helper()
	return Service{
		Configuration: Configuration{
			AdmindBaseURL:       "http://127.0.0.1:18080",
			MattermostBaseURL:   "https://mattermost.test",
			MattermostTokenPath: writePlatformTestFile(t, "mattermost-token"),
			FleetIDPath:         writePlatformTestFile(t, "dc719d8e"),
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(roundTrip)},
	}
}

func mustJSONRaw(t *testing.T, value any) json.RawMessage {
	t.Helper()
	document, errorValue := json.Marshal(value)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func waitMattermostTypingPayload(t *testing.T, typingRequests <-chan map[string]string) map[string]string {
	t.Helper()
	select {
	case payload := <-typingRequests:
		return payload
	case <-time.After(2 * time.Second):
		t.Fatal("expected typing request")
	}
	return nil
}

func TestMattermostDirectMessageStartsThreadWithDirectHistory(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "dm-1",
		Message:   "hello",
	}, "bot-1", "D", "", platformAddressing{})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected direct message event")
	}
	if event.ConversationID != "thread:dm-1:post-1" {
		t.Fatalf("expected direct message to start thread conversation, got %q", event.ConversationID)
	}
	if event.Context.ConversationType != "D" || event.Context.ChannelID != "dm-1" {
		t.Fatalf("expected direct message metadata in context, got %+v", event.Context)
	}
	replyHandle, errorValue := decodePlatformHandle(event.ReplyTargetID)
	if errorValue != nil {
		t.Fatalf("expected reply target to decode: %v", errorValue)
	}
	if replyHandle.RootID != "post-1" {
		t.Fatalf("expected DM reply root to be post-1, got %q", replyHandle.RootID)
	}
	historyHandle, errorValue := decodePlatformHandle(event.Context.HistoryCursor)
	if errorValue != nil {
		t.Fatalf("expected history cursor to decode: %v", errorValue)
	}
	if historyHandle.ConversationID != "dm:dm-1" || historyHandle.RootID != "" || historyHandle.MessageID != "post-1" {
		t.Fatalf("expected direct history cursor for thread root, got %+v", historyHandle)
	}
}

func TestMattermostDirectMessageWithMentionStartsThreadWithDirectHistory(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "dm-1",
		Message:   "@internkim hello",
	}, "bot-1", "D", "", platformAddressing{BotMentioned: true})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected direct message event")
	}
	if event.ConversationID != "thread:dm-1:post-1" {
		t.Fatalf("expected mentioned dm to start thread conversation, got %q", event.ConversationID)
	}
	replyHandle, errorValue := decodePlatformHandle(event.ReplyTargetID)
	if errorValue != nil {
		t.Fatalf("expected reply target to decode: %v", errorValue)
	}
	if replyHandle.RootID != "post-1" {
		t.Fatalf("expected mentioned dm reply root to be post-1, got %q", replyHandle.RootID)
	}
	historyHandle, errorValue := decodePlatformHandle(event.Context.HistoryCursor)
	if errorValue != nil {
		t.Fatalf("expected history cursor to decode: %v", errorValue)
	}
	if historyHandle.ConversationID != "dm:dm-1" || historyHandle.RootID != "" || historyHandle.MessageID != "post-1" || historyHandle.ChannelID != "dm-1" {
		t.Fatalf("expected direct history cursor for mentioned dm, got %+v", historyHandle)
	}
}

func TestMattermostDirectMessageThreadKeepsThreadRoot(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "reply-1",
		UserID:    "user-1",
		ChannelID: "dm-1",
		Message:   "thread reply",
		RootID:    "root-9",
	}, "bot-1", "D", "", platformAddressing{})
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

func TestMattermostChannelMessageWithoutMentionIsForwardedForAddressingGate(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "casual chatter",
	}, "bot-1", "O", "random-chat", platformAddressing{})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected non-default channel without mention to be forwarded for addressing gate")
	}
	if event.Context.Addressing.BotMentioned || event.Context.Addressing.OtherPersonMentioned {
		t.Fatalf("expected no addressing mention flags, got %+v", event.Context.Addressing)
	}
}

func TestMattermostChannelMessageWithMentionIsForwarded(t *testing.T) {
	event, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "@internkim hi",
	}, "bot-1", "O", "random-chat", platformAddressing{BotMentioned: true})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected channel mention to produce an event")
	}
	if event.ConversationID != "thread:channel-1:post-1" {
		t.Fatalf("expected mention to start a thread conversation, got %q", event.ConversationID)
	}
	if !event.Context.Addressing.BotMentioned {
		t.Fatalf("expected bot mention metadata, got %+v", event.Context.Addressing)
	}
}

func TestMattermostChannelMentionOfOtherPersonIsDropped(t *testing.T) {
	_, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "@gamyeong please review",
	}, "bot-1", "O", "random-chat", platformAddressing{OtherPersonMentioned: true})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if hasEvent {
		t.Fatal("expected channel message mentioning only another person to be dropped")
	}
}

func TestMattermostChannelBroadcastMentionIsDropped(t *testing.T) {
	_, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "@all standup in five minutes",
	}, "bot-1", "O", "random-chat", platformAddressing{})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if hasEvent {
		t.Fatal("expected channel broadcast @all mention to be dropped")
	}
}

func TestMattermostChannelMentionIncludingBotIsForwarded(t *testing.T) {
	_, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "@internkim @gamyeong take a look",
	}, "bot-1", "O", "random-chat", platformAddressing{BotMentioned: true, OtherPersonMentioned: true})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected channel message mentioning the bot to be forwarded even alongside others")
	}
}

func TestMattermostDirectMentionOfOtherPersonIsForwarded(t *testing.T) {
	_, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "direct-1",
		Message:   "@gamyeong lunch?",
	}, "bot-1", "D", "", platformAddressing{OtherPersonMentioned: true})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected direct message to be forwarded regardless of mentions")
	}
}

func TestMattermostTownSquareForwardsWithoutMention(t *testing.T) {
	_, hasEvent, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "channel-1",
		Message:   "town square chatter",
	}, "bot-1", "O", "town-square", platformAddressing{})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected town-square message to be forwarded without mention")
	}
}

func TestMattermostGroupMessageWithoutMentionIsForwardedForAddressingGate(t *testing.T) {
	_, hasEventWithoutMention, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-1",
		UserID:    "user-1",
		ChannelID: "group-1",
		Message:   "hi all",
	}, "bot-1", "G", "", platformAddressing{})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEventWithoutMention {
		t.Fatal("expected group message without mention to be forwarded for addressing gate")
	}

	_, hasEventWithMention, errorValue := normalizeMattermostPost(mattermostPost{
		ID:        "post-2",
		UserID:    "user-1",
		ChannelID: "group-1",
		Message:   "@internkim help",
	}, "bot-1", "G", "", platformAddressing{BotMentioned: true})
	if errorValue != nil {
		t.Fatalf("expected normalization to succeed: %v", errorValue)
	}
	if !hasEventWithMention {
		t.Fatal("expected group message with mention to be forwarded")
	}
}

func TestMattermostWebSocketPayloadPreservesMentionMetadata(t *testing.T) {
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

	event, hasEvent, errorValue := normalizeMattermostWebSocketPayload(buildPayload("O", "random-chat", "", "no mention"), "bot-1", "internkim")
	if errorValue != nil {
		t.Fatalf("expected payload normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected channel post without mention to be forwarded")
	}
	if event.Context.Addressing.BotMentioned || event.Context.Addressing.OtherPersonMentioned {
		t.Fatalf("expected no mention flags, got %+v", event.Context.Addressing)
	}

	event, hasEvent, errorValue = normalizeMattermostWebSocketPayload(buildPayload("O", "random-chat", `["bot-1"]`, "@internkim hi"), "bot-1", "internkim")
	if errorValue != nil {
		t.Fatalf("expected mention payload normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected channel mention payload to be forwarded")
	}
	if !event.Context.Addressing.BotMentioned || event.Context.Addressing.OtherPersonMentioned {
		t.Fatalf("expected bot mention metadata, got %+v", event.Context.Addressing)
	}

	_, hasEvent, errorValue = normalizeMattermostWebSocketPayload(buildPayload("O", "town-square", "", "@iam 아직 상태 업데이트는 툴로 추가 안 했었어요."), "bot-1", "internkim")
	if errorValue != nil {
		t.Fatalf("expected fallback mention payload normalization to succeed: %v", errorValue)
	}
	if hasEvent {
		t.Fatal("expected channel mention of only another person to be dropped at admission")
	}

	event, hasEvent, errorValue = normalizeMattermostWebSocketPayload(buildPayload("O", "town-square", "", "@internkim 확인해줘"), "bot-1", "internkim")
	if errorValue != nil {
		t.Fatalf("expected fallback bot mention payload normalization to succeed: %v", errorValue)
	}
	if !hasEvent {
		t.Fatal("expected fallback bot mention payload to be forwarded")
	}
	if !event.Context.Addressing.BotMentioned || event.Context.Addressing.OtherPersonMentioned {
		t.Fatalf("expected fallback bot mention metadata, got %+v", event.Context.Addressing)
	}
}

func TestMattermostAttendanceChannelPostsAreNotForwarded(t *testing.T) {
	buildAttendancePost := func(message string, rootID string) []byte {
		postDocument, _ := json.Marshal(mattermostPost{
			ID:        "post-1",
			UserID:    "user-1",
			ChannelID: "attendance-channel",
			Message:   message,
			RootID:    rootID,
		})
		envelope := map[string]any{
			"event": "posted",
			"data": map[string]any{
				"channel_type": "O",
				"channel_name": mattermostdefaults.AttendanceChannelName,
				"post":         string(postDocument),
			},
		}
		document, _ := json.Marshal(envelope)
		return document
	}

	for _, message := range []string{"출근", "출금", "@internkim 확인해줘"} {
		_, hasEvent, errorValue := normalizeMattermostWebSocketPayload(buildAttendancePost(message, "result-post-1"), "bot-1", "internkim")
		if errorValue != nil {
			t.Fatalf("expected attendance payload normalization to succeed: %v", errorValue)
		}
		if hasEvent {
			t.Fatalf("expected attendance channel reply %q to be ignored", message)
		}
	}
}

func TestMattermostAddressingDetectsOtherAndMixedMentions(t *testing.T) {
	otherAddressing := mattermostAddressingFromMessage("@lee 확인 부탁해요", "internkim")
	if otherAddressing.BotMentioned || !otherAddressing.OtherPersonMentioned {
		t.Fatalf("expected other person mention, got %+v", otherAddressing)
	}

	mixedAddressing := mattermostAddressingFromMessage("@lee @channel @internkim 부탁", "internkim")
	if !mixedAddressing.BotMentioned || !mixedAddressing.OtherPersonMentioned {
		t.Fatalf("expected mixed mention metadata, got %+v", mixedAddressing)
	}

	plainNameAddressing := mattermostAddressingFromMessage("오늘 오후 5시 정기회의 추가 참석자 최견본, 이샘플", "internkim")
	if plainNameAddressing.BotMentioned || plainNameAddressing.OtherPersonMentioned {
		t.Fatalf("expected plain names not to count as mentions, got %+v", plainNameAddressing)
	}
}

func TestMattermostEnrichPreservesChannelAndAddressingMetadata(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/posts/post-1/thread":
			return testJSONResponse(http.StatusOK, struct {
				Order []string                         `json:"order"`
				Posts map[string]mattermostHistoryPost `json:"posts"`
			}{
				Order: []string{"post-1"},
				Posts: map[string]mattermostHistoryPost{
					"post-1": {ID: "post-1", UserID: "user-1", Message: "@internkim help", CreateAt: 1000},
				},
			}), nil
		case "/api/v4/users/user-1":
			return testJSONResponse(http.StatusOK, map[string]string{
				"id":       "user-1",
				"username": "lee",
			}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s?%s", request.URL.Path, request.URL.RawQuery)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{MattermostBaseURL: "http://mattermost.test", MattermostTokenPath: writePlatformTestFile(t, "test-token")},
		HTTPClient:    httpClient,
	}
	historyCursor, errorValue := encodePlatformHandle(platformHandle{
		Platform:       "mattermost",
		ConversationID: "thread:channel-1:post-1",
		ChannelID:      "channel-1",
		ChannelType:    "O",
		RootID:         "post-1",
		MessageID:      "post-1",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	event := service.enrichMattermostEvent(context.Background(), platformInboundEvent{
		SenderID: "user-1",
		Context: platformEventContext{
			HistoryCursor:    historyCursor,
			ConversationType: "O",
			ChannelID:        "channel-1",
			ChannelName:      "random-chat",
			Addressing:       platformAddressing{BotMentioned: true},
		},
	})

	if event.Context.ConversationType != "O" || event.Context.ChannelID != "channel-1" || event.Context.ChannelName != "random-chat" {
		t.Fatalf("expected channel metadata to survive enrich, got %+v", event.Context)
	}
	if !event.Context.Addressing.BotMentioned || event.Context.Addressing.OtherPersonMentioned {
		t.Fatalf("expected addressing metadata to survive enrich, got %+v", event.Context.Addressing)
	}
	if event.Context.Sender.UserID != "user-1" || event.Context.ReceivedAt == "" {
		t.Fatalf("expected sender and received metadata, got %+v", event.Context)
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
					"post-1":       {ID: "post-1", UserID: "user-1", Message: "previous", CreateAt: 1000},
					"thread-reply": {ID: "thread-reply", UserID: "bot-1", Message: "thread reply", RootID: "other-root", CreateAt: 1500},
					"post-2":       {ID: "post-2", UserID: "user-2", Message: "current", CreateAt: 2000},
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

func TestMattermostContextPreservesHistoryAttachments(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/channels/channel-1/posts":
			return testJSONResponse(http.StatusOK, struct {
				Order []string                         `json:"order"`
				Posts map[string]mattermostHistoryPost `json:"posts"`
			}{
				Order: []string{"post-1", "post-2"},
				Posts: map[string]mattermostHistoryPost{
					"post-1": {ID: "post-1", UserID: "user-1", Message: "첨부 확인", FileIDs: []string{"file-1"}, CreateAt: 1000},
					"post-2": {ID: "post-2", UserID: "user-2", Message: "current", CreateAt: 2000},
				},
			}), nil
		case "/api/v4/users/user-1":
			return testJSONResponse(http.StatusOK, map[string]string{
				"id":       "user-1",
				"username": "lee",
			}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s?%s", request.URL.Path, request.URL.RawQuery)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{MattermostBaseURL: "http://mattermost.test", MattermostTokenPath: writePlatformTestFile(t, "test-token")},
		HTTPClient:    httpClient,
	}

	contextValue := service.mattermostContext(context.Background(), platformHandle{ChannelID: "channel-1", MessageID: "post-2"}, 20)

	if len(contextValue.Messages) != 1 || len(contextValue.Messages[0].InputAttachments) != 1 {
		t.Fatalf("expected message attachment catalog, got %+v", contextValue.Messages)
	}
	messageAttachment := contextValue.Messages[0].InputAttachments[0]
	if messageAttachment.Platform != "mattermost" || messageAttachment.FileID != "file-1" || messageAttachment.MessageID != "post-1" {
		t.Fatalf("unexpected message attachment catalog: %+v", messageAttachment)
	}
	if len(contextValue.Materials) != 1 || contextValue.Materials[0].FileID != "file-1" {
		t.Fatalf("expected conversation material catalog, got %+v", contextValue.Materials)
	}
}

func TestMattermostContextAnnotatesReadableMentions(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/channels/channel-1/posts":
			return testJSONResponse(http.StatusOK, struct {
				Order []string                         `json:"order"`
				Posts map[string]mattermostHistoryPost `json:"posts"`
			}{
				Order: []string{"post-1", "post-2"},
				Posts: map[string]mattermostHistoryPost{
					"post-1": {
						ID:       "post-1",
						UserID:   "user-1",
						Message:  "@lee 시간 확인해주세요.",
						CreateAt: 1000,
						Metadata: struct {
							Mentions []string `json:"mentions"`
						}{Mentions: []string{"user-2"}},
					},
					"post-2": {ID: "post-2", UserID: "user-2", Message: "current", CreateAt: 2000},
				},
			}), nil
		case "/api/v4/users/user-1":
			return testJSONResponse(http.StatusOK, map[string]string{
				"id":       "user-1",
				"username": "kim",
				"nickname": "김표본",
			}), nil
		case "/api/v4/users/user-2":
			return testJSONResponse(http.StatusOK, map[string]string{
				"id":       "user-2",
				"username": "lee",
				"nickname": "이샘플",
			}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s?%s", request.URL.Path, request.URL.RawQuery)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{MattermostBaseURL: "http://mattermost.test", MattermostTokenPath: writePlatformTestFile(t, "test-token")},
		HTTPClient:    httpClient,
	}

	contextValue := service.mattermostContext(context.Background(), platformHandle{ChannelID: "channel-1", MessageID: "post-2"}, 20)

	if len(contextValue.Messages) != 1 {
		t.Fatalf("expected one history message, got %+v", contextValue.Messages)
	}
	if contextValue.Messages[0].Text != "@lee(이샘플) 시간 확인해주세요." {
		t.Fatalf("expected readable mention annotation, got %q", contextValue.Messages[0].Text)
	}
}

func TestMattermostContextKeepsHistoryCursorForDirectRoot(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/channels/dm-1/posts":
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
				"id":       "user-1",
				"username": "lee",
			}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s?%s", request.URL.Path, request.URL.RawQuery)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{MattermostBaseURL: "http://mattermost.test", MattermostTokenPath: writePlatformTestFile(t, "test-token")},
		HTTPClient:    httpClient,
	}
	handle := platformHandle{Platform: "mattermost", ConversationID: "dm:dm-1", ChannelID: "dm-1", ChannelType: "D", MessageID: "post-2"}
	contextValue := service.mattermostContext(context.Background(), handle, 20)
	if len(contextValue.Messages) != 1 || contextValue.Messages[0].Text != "previous" {
		t.Fatalf("expected previous direct message context, got %+v", contextValue.Messages)
	}
	historyHandle, errorValue := decodePlatformHandle(contextValue.HistoryCursor)
	if errorValue != nil {
		t.Fatalf("expected history cursor to decode: %v", errorValue)
	}
	if historyHandle.RootID != "" || historyHandle.MessageID != "post-2" || historyHandle.ChannelID != "dm-1" {
		t.Fatalf("expected direct root history cursor, got %+v", historyHandle)
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
	pollState := &mattermostPollState{LastSeenByChannel: map[string]int64{}}

	if errorValue := service.pollMattermost(context.Background(), pollState); errorValue != nil {
		t.Fatalf("expected first poll to seed watermark: %v", errorValue)
	}
	if forwardedCount != 0 {
		t.Fatalf("expected first poll not to replay old dm, got %d forwards", forwardedCount)
	}
	if pollState.LastSeenByChannel["dm-1"] <= 0 {
		t.Fatalf("expected dm watermark to seed, got %d", pollState.LastSeenByChannel["dm-1"])
	}

	if errorValue := service.pollMattermost(context.Background(), pollState); errorValue != nil {
		t.Fatalf("expected second poll to forward new message: %v", errorValue)
	}
	if forwardedCount != 1 {
		t.Fatalf("expected only new dm to be forwarded once, got %d forwards", forwardedCount)
	}
	if pollState.LastSeenByChannel["dm-1"] != testFutureMattermostPostCreateAt {
		t.Fatalf("expected dm watermark to advance, got %d", pollState.LastSeenByChannel["dm-1"])
	}
}

func TestMattermostPollerForwardsFirstMessageInNewDirectChannel(t *testing.T) {
	forwardedCount := 0
	channelListRequestCount := 0
	newPostCreateAt := time.Now().UnixMilli() + 1000
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == "blueclaw.test" {
			forwardedCount++
			return handleTestBlueclawForward(t, request)
		}
		if request.URL.Host != "mattermost.test" {
			t.Fatalf("unexpected request host: %s", request.URL.Host)
		}
		switch {
		case request.URL.Path == "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "username": "internkim", "is_bot": true}), nil
		case request.URL.Path == "/api/v4/users/bot-1/channels":
			channelListRequestCount++
			if channelListRequestCount == 1 {
				return testJSONResponse(http.StatusOK, []map[string]string{{"id": "dm-1", "type": "D", "name": "user-1__bot-1"}}), nil
			}
			return testJSONResponse(http.StatusOK, []map[string]string{
				{"id": "dm-1", "type": "D", "name": "user-1__bot-1"},
				{"id": "dm-2", "type": "D", "name": "user-1__bot-1"},
			}), nil
		case request.URL.Path == "/api/v4/channels/dm-1/posts" && request.URL.Query().Get("since") != "":
			return testJSONResponse(http.StatusOK, mattermostPostsResponse{Order: []string{}, Posts: map[string]mattermostPolledPost{}}), nil
		case request.URL.Path == "/api/v4/channels/dm-2/posts" && request.URL.Query().Get("since") != "":
			return testJSONResponse(http.StatusOK, mattermostPostsResponse{
				Order: []string{"new-2"},
				Posts: map[string]mattermostPolledPost{
					"new-2": {ID: "new-2", UserID: "user-1", ChannelID: "dm-2", Message: "new dm", CreateAt: newPostCreateAt, FileIDs: []string{"file-1"}},
				},
			}), nil
		case request.URL.Path == "/api/v4/channels/dm-2/posts" && request.URL.Query().Get("per_page") == "21":
			return testJSONResponse(http.StatusOK, mattermostPostsResponse{
				Order: []string{"new-2"},
				Posts: map[string]mattermostPolledPost{
					"new-2": {ID: "new-2", UserID: "user-1", ChannelID: "dm-2", Message: "new dm", CreateAt: newPostCreateAt, FileIDs: []string{"file-1"}},
				},
			}), nil
		case request.URL.Path == "/api/v4/users/user-1":
			return testJSONResponse(http.StatusOK, map[string]string{
				"id":         "user-1",
				"email":      "seoyeon@example.com",
				"username":   "seoyeon",
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
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	configuration.BlueclawBaseURL = "http://blueclaw.test"
	service := Service{Configuration: configuration, HTTPClient: httpClient}
	pollState := &mattermostPollState{LastSeenByChannel: map[string]int64{}}

	if errorValue := service.pollMattermost(context.Background(), pollState); errorValue != nil {
		t.Fatalf("expected initial poll to seed existing channel: %v", errorValue)
	}
	if forwardedCount != 0 {
		t.Fatalf("expected no forward during initial seed, got %d", forwardedCount)
	}
	if errorValue := service.pollMattermost(context.Background(), pollState); errorValue != nil {
		t.Fatalf("expected new channel poll to forward first message: %v", errorValue)
	}
	if forwardedCount != 1 {
		t.Fatalf("expected new direct channel first message to be forwarded, got %d forwards", forwardedCount)
	}
	if pollState.LastSeenByChannel["dm-2"] != newPostCreateAt {
		t.Fatalf("expected new channel watermark to advance, got %d", pollState.LastSeenByChannel["dm-2"])
	}
}

func TestMattermostBotChannelsPaginates(t *testing.T) {
	requestedPages := []string{}
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "mattermost.test" {
			t.Fatalf("unexpected request host: %s", request.URL.Host)
		}
		if request.URL.Path != "/api/v4/users/bot-1/channels" {
			t.Fatalf("unexpected Mattermost request: %s?%s", request.URL.Path, request.URL.RawQuery)
		}
		requestedPages = append(requestedPages, request.URL.Query().Get("page"))
		if request.URL.Query().Get("page") == "0" {
			channels := make([]mattermostBotChannel, 200)
			for index := range channels {
				channels[index] = mattermostBotChannel{ID: fmt.Sprintf("old-%d", index), Type: "D"}
			}
			return testJSONResponse(http.StatusOK, channels), nil
		}
		return testJSONResponse(http.StatusOK, []mattermostBotChannel{{ID: "new-dm", Type: "D"}}), nil
	})}
	service := Service{
		Configuration: Configuration{MattermostBaseURL: "http://mattermost.test", MattermostTokenPath: writePlatformTestFile(t, "test-token")},
		HTTPClient:    httpClient,
	}

	channels, errorValue := service.mattermostBotChannels(context.Background(), "bot-1")
	if errorValue != nil {
		t.Fatalf("expected channels to load: %v", errorValue)
	}
	if len(channels) != 201 {
		t.Fatalf("expected paginated channels, got %d", len(channels))
	}
	if channels[200].ID != "new-dm" {
		t.Fatalf("expected second page channel, got %+v", channels[200])
	}
	if strings.Join(requestedPages, ",") != "0,1" {
		t.Fatalf("expected page 0 and 1, got %v", requestedPages)
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
	pollState := &mattermostPollState{LastSeenByChannel: map[string]int64{"dm-1": 1000}, IsInitialized: true}

	if errorValue := service.pollMattermost(context.Background(), pollState); errorValue != nil {
		t.Fatalf("expected failed forward to stay in poll loop: %v", errorValue)
	}
	if forwardedCount != 1 {
		t.Fatalf("expected one forward attempt, got %d", forwardedCount)
	}
	if pollState.LastSeenByChannel["dm-1"] != 1000 {
		t.Fatalf("expected watermark to remain at failed event, got %d", pollState.LastSeenByChannel["dm-1"])
	}
}

func TestMattermostProgressStartPublishesTypingUntilStopped(t *testing.T) {
	typingRequests := make(chan map[string]string, 2)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
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

	threadPayload := waitMattermostTypingPayload(t, typingRequests)
	channelPayload := waitMattermostTypingPayload(t, typingRequests)
	if threadPayload["channel_id"] != "channel-1" || threadPayload["parent_id"] != "root-1" {
		t.Fatalf("unexpected thread typing payload: %+v", threadPayload)
	}
	if channelPayload["channel_id"] != "channel-1" || channelPayload["parent_id"] != "" {
		t.Fatalf("unexpected channel typing payload: %+v", channelPayload)
	}

	_, errorValue = service.mattermostStopProgressFromRequest(context.Background(), strings.NewReader(`{"replyTargetID":"`+replyTargetID+`"}`))
	if errorValue != nil {
		t.Fatalf("expected progress stop to succeed: %v", errorValue)
	}
}

func TestMattermostReplyStopsProgressBeforeSendingPost(t *testing.T) {
	typingRequests := make(chan map[string]string, 2)
	postRequests := make(chan map[string]any, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
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

	reply, errorValue := service.mattermostReply(context.Background(), json.RawMessage(`{"replyTargetID":"`+replyTargetID+`","message":"done","rawEventID":"raw-event-1","outboxID":"outbox-1"}`))
	if errorValue != nil {
		t.Fatalf("expected reply to succeed: %v", errorValue)
	}
	replyResult, isReplyResult := reply.(platformReplyResult)
	if !isReplyResult || replyResult.Platform != "mattermost" || replyResult.Visibility != "public" || !replyResult.MessageDelivered || replyResult.NativeAttachmentCount != 0 {
		t.Fatalf("expected public reply evidence without attachments, got %+v", reply)
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
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ConversationID: "thread:channel-1:root-1", ChannelID: "channel-1", RootID: "root-1"})
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

func TestMattermostEphemeralReplyUsesBotAuthentication(t *testing.T) {
	ephemeralRequests := make(chan map[string]any, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
		case "/api/v4/posts/ephemeral":
			if request.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("ephemeral authorization = %q", request.Header.Get("Authorization"))
			}
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected ephemeral request to decode: %v", errorValue)
			}
			ephemeralRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{"id": "ephemeral-1"}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ChannelID: "channel-1", RootID: "root-1"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	service := Service{Configuration: configuration, HTTPClient: httpClient}

	_, errorValue = service.mattermostReply(context.Background(), mustJSON(t, replyRequest{
		ReplyTargetID:   replyTargetID,
		Message:         "Checking current tasks",
		RawEventID:      "raw-event-1",
		OutboxID:        "outbox-1",
		EphemeralUserID: "requester-1",
	}))
	if errorValue != nil {
		t.Fatalf("expected ephemeral reply to succeed: %v", errorValue)
	}

	payload := <-ephemeralRequests
	if payload["user_id"] != "requester-1" {
		t.Fatalf("ephemeral target = %+v", payload)
	}
	post, isMap := payload["post"].(map[string]any)
	if !isMap || post["channel_id"] != "channel-1" || post["root_id"] != "root-1" {
		t.Fatalf("ephemeral post = %+v", payload["post"])
	}
	if post["user_id"] != "bot-1" {
		t.Fatalf("ephemeral author must be the authenticated bot, got %+v", post)
	}
}

func TestMattermostEphemeralReplyRejectsNativeAttachments(t *testing.T) {
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ConversationID: "thread:channel-1:root-1", ChannelID: "channel-1", RootID: "root-1"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	service := Service{Configuration: DefaultConfiguration()}

	_, errorValue = service.mattermostReply(context.Background(), mustJSON(t, replyRequest{
		ReplyTargetID:   replyTargetID,
		Message:         "private update",
		RawEventID:      "raw-event-1",
		OutboxID:        "outbox-1",
		EphemeralUserID: "user-1",
		Attachments:     []platformFileSpec{{DevicePath: "/workspace/report.docx", Filename: "report.docx"}},
	}))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "ephemeral reply cannot send native file attachments") {
		t.Fatalf("expected ephemeral native attachment rejection, got %v", errorValue)
	}
}

func TestMattermostReplyFallsBackWhenThreadRootIsInvalid(t *testing.T) {
	postRequests := make(chan map[string]any, 2)
	requestCount := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/api/v4/posts" {
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
		}
		var payload map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatalf("expected post request to decode: %v", errorValue)
		}
		postRequests <- payload
		requestCount++
		if requestCount == 1 {
			return testJSONResponse(http.StatusBadRequest, map[string]any{
				"id":      "api.post.create_post.root_id.app_error",
				"message": "Invalid RootId parameter.",
			}), nil
		}
		return testJSONResponse(http.StatusOK, map[string]string{"id": "post-2"}), nil
	})}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ChannelID: "channel-1", RootID: "missing-root"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.MattermostTokenPath = tokenPath
	service := Service{Configuration: configuration, HTTPClient: httpClient}

	_, errorValue = service.mattermostReply(context.Background(), json.RawMessage(`{"replyTargetID":"`+replyTargetID+`","message":"done","rawEventID":"raw-event-1","outboxID":"outbox-1"}`))
	if errorValue != nil {
		t.Fatalf("expected reply fallback to succeed: %v", errorValue)
	}
	firstPayload := <-postRequests
	secondPayload := <-postRequests
	if firstPayload["root_id"] != "missing-root" {
		t.Fatalf("expected first post to target thread root, got %+v", firstPayload)
	}
	if _, exists := secondPayload["root_id"]; exists || secondPayload["channel_id"] != "channel-1" {
		t.Fatalf("expected fallback post without root_id, got %+v", secondPayload)
	}
}

func TestMattermostReplySendsAskChoiceEphemeralControl(t *testing.T) {
	postRequests := make(chan map[string]any, 1)
	ephemeralRequests := make(chan map[string]any, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
		case "/api/v4/posts":
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected post request to decode: %v", errorValue)
			}
			postRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{"id": "post-1"}), nil
		case "/api/v4/posts/ephemeral":
			if request.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("ephemeral authorization = %q", request.Header.Get("Authorization"))
			}
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected ephemeral request to decode: %v", errorValue)
			}
			ephemeralRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{"id": "ephemeral-1"}), nil
		case "/api/v4/users/user-1":
			return testJSONResponse(http.StatusOK, map[string]string{"id": "user-1", "username": "user-one"}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ConversationID: "thread:channel-1:root-1", ChannelID: "channel-1", RootID: "root-1"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.BlueclawBaseURL = "http://blueclaw.test"
	configuration.AdmindBaseURL = "http://admind.test"
	configuration.MattermostTokenPath = tokenPath
	configuration.MattermostInteractiveTokenPath = t.TempDir() + "/interactive-token"
	service := Service{Configuration: configuration, HTTPClient: httpClient}
	requestDocument := map[string]any{
		"replyTargetID": replyTargetID,
		"message":       "구현은 어떻게 하는 게 좋을까요?",
		"rawEventID":    "raw-event-1",
		"outboxID":      "outbox-1",
		"interaction": map[string]any{
			"interactionID":        "interaction-1",
			"taskRunID":            "task-1",
			"kind":                 "ask_choice_single",
			"question":             "구현은 어떻게 하는 게 좋을까요?",
			"recommendedOptionKey": "A",
			"selectionMode":        "single",
			"targetPlatformUserID": "user-1",
			"options": []map[string]string{
				{"key": "A", "label": "최대한 가독성 있게", "shortLabel": "가독성"},
				{"key": "B", "label": "최대한 빠르게"},
				{"key": "C", "label": "최대한 짧게"},
			},
		},
	}
	payload, _ := json.Marshal(requestDocument)

	_, errorValue = service.mattermostReply(context.Background(), payload)
	if errorValue != nil {
		t.Fatalf("expected reply to succeed: %v", errorValue)
	}

	select {
	case payload := <-postRequests:
		if payload["message"] != "@user-one 구현은 어떻게 하는 게 좋을까요?" || payload["root_id"] != "root-1" {
			t.Fatalf("expected question post in thread, got %+v", payload)
		}
		props, isMap := payload["props"].(map[string]any)
		if !isMap {
			t.Fatalf("expected props, got %+v", payload)
		}
		if _, hasAttachments := props["attachments"]; hasAttachments {
			t.Fatalf("expected public post to carry no interactive attachment, got %+v", props)
		}
	default:
		t.Fatal("expected question post request")
	}

	select {
	case payload := <-ephemeralRequests:
		if payload["user_id"] != "user-1" {
			t.Fatalf("expected ephemeral control targeted at user-1, got %+v", payload)
		}
		post, isMap := payload["post"].(map[string]any)
		if !isMap {
			t.Fatalf("expected ephemeral post document, got %+v", payload)
		}
		if post["user_id"] != "bot-1" {
			t.Fatalf("expected ephemeral control author to be the authenticated bot, got %+v", post)
		}
		props := post["props"].(map[string]any)
		attachments := props["attachments"].([]any)
		if len(attachments) != 1 {
			t.Fatalf("expected one interactive attachment, got %+v", props)
		}
		attachment := attachments[0].(map[string]any)
		if !strings.Contains(attachment["text"].(string), "1. 최대한 가독성 있게 (추천)") {
			t.Fatalf("expected attachment text to include choice list, got %+v", attachment)
		}
		actions := attachment["actions"].([]any)
		action := actions[0].(map[string]any)
		if action["name"] != "가독성" {
			t.Fatalf("expected short choice button, got %+v", action)
		}
		integration := action["integration"].(map[string]any)
		if integration["url"] != "http://admind.test/_internkim/mattermost/actions" {
			t.Fatalf("expected admind action URL, got %+v", integration)
		}
		contextDocument := integration["context"].(map[string]any)
		if contextDocument["token"] == "" || contextDocument["action"] != "ask.choice" || contextDocument["conversationID"] != "thread:channel-1:root-1" || contextDocument["targetUserID"] != "user-1" || contextDocument["choiceLabel"] != "가독성" {
			t.Fatalf("expected ask action token context, got %+v", contextDocument)
		}
	default:
		t.Fatal("expected ephemeral control request")
	}
}

func TestMattermostReplySendsAskAttachmentEphemeralForRequester(t *testing.T) {
	postRequests := make(chan map[string]any, 1)
	ephemeralRequests := make(chan map[string]any, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
		case "/api/v4/posts":
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected post request to decode: %v", errorValue)
			}
			postRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{"id": "post-1"}), nil
		case "/api/v4/posts/ephemeral":
			if request.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("ephemeral authorization = %q", request.Header.Get("Authorization"))
			}
			var payload map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected ephemeral request to decode: %v", errorValue)
			}
			ephemeralRequests <- payload
			return testJSONResponse(http.StatusOK, map[string]string{"id": "ephemeral-1"}), nil
		case "/api/v4/users/requester-1":
			return testJSONResponse(http.StatusOK, map[string]string{"id": "requester-1", "username": "requester-one"}), nil
		default:
			t.Fatalf("unexpected Mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	replyTargetID, errorValue := encodePlatformHandle(platformHandle{Platform: "mattermost", ConversationID: "thread:channel-1:root-1", ChannelID: "channel-1", RootID: "root-1"})
	if errorValue != nil {
		t.Fatalf("expected reply target to encode: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.BlueclawBaseURL = "http://blueclaw.test"
	configuration.AdmindBaseURL = "http://admind.test"
	configuration.MattermostTokenPath = tokenPath
	configuration.MattermostInteractiveTokenPath = t.TempDir() + "/interactive-token"
	service := Service{Configuration: configuration, HTTPClient: httpClient}
	requestDocument := map[string]any{
		"replyTargetID":   replyTargetID,
		"message":         "테스트 님에게 다음 DM을 보내도 될까요?\n\n바보",
		"rawEventID":      "raw-event-1",
		"outboxID":        "outbox-1",
		"ephemeralUserID": "requester-1",
		"interaction": map[string]any{
			"interactionID": "interaction-1",
			"taskRunID":     "task-1",
			"kind":          "ask_confirm",
			"message":       "테스트 님에게 다음 DM을 보내도 될까요?\n\n바보",
		},
	}
	payload, _ := json.Marshal(requestDocument)

	_, errorValue = service.mattermostReply(context.Background(), payload)
	if errorValue != nil {
		t.Fatalf("expected reply to succeed: %v", errorValue)
	}

	select {
	case payload := <-postRequests:
		if payload["message"] != "@requester-one 테스트 님에게 다음 DM을 보내도 될까요?\n\n바보" {
			t.Fatalf("expected mentioned question post, got %+v", payload)
		}
		props, isMap := payload["props"].(map[string]any)
		if !isMap {
			t.Fatalf("expected post props, got %+v", payload)
		}
		if _, hasAttachments := props["attachments"]; hasAttachments {
			t.Fatalf("expected public post to carry no attachment, got %+v", props)
		}
	default:
		t.Fatal("expected public post request")
	}
	select {
	case payload := <-ephemeralRequests:
		if payload["user_id"] != "requester-1" {
			t.Fatalf("expected ephemeral attachment targeted at requester-1, got %+v", payload)
		}
		post, isMap := payload["post"].(map[string]any)
		if !isMap {
			t.Fatalf("expected ephemeral post document, got %+v", payload)
		}
		if post["user_id"] != "bot-1" {
			t.Fatalf("expected ephemeral attachment author to be the authenticated bot, got %+v", post)
		}
		props := post["props"].(map[string]any)
		attachments := props["attachments"].([]any)
		if len(attachments) != 1 {
			t.Fatalf("expected one ephemeral ask attachment, got %+v", props)
		}
		attachment := attachments[0].(map[string]any)
		actions := attachment["actions"].([]any)
		if len(actions) != 2 {
			t.Fatalf("expected confirm actions, got %+v", attachment)
		}
		action := actions[0].(map[string]any)
		integration := action["integration"].(map[string]any)
		contextDocument := integration["context"].(map[string]any)
		if _, hasMessage := contextDocument["message"]; hasMessage {
			t.Fatalf("expected action context without message copy, got %+v", contextDocument)
		}
	default:
		t.Fatal("expected ephemeral attachment request")
	}
}

func TestMattermostInteractionResolveClearsAttachments(t *testing.T) {
	patchRequests := make(chan map[string]any, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "http://mattermost.test/api/v4/posts/post-1/patch" || request.Method != http.MethodPut {
			t.Fatalf("unexpected Mattermost request: %s %s", request.Method, request.URL.String())
		}
		var payload map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
			t.Fatalf("expected patch request to decode: %v", errorValue)
		}
		patchRequests <- payload
		return testJSONResponse(http.StatusOK, map[string]string{"id": "post-1"}), nil
	})}
	tokenPath := t.TempDir() + "/mattermost-token"
	if errorValue := os.WriteFile(tokenPath, []byte("test-token\n"), 0o600); errorValue != nil {
		t.Fatalf("expected token file to be written: %v", errorValue)
	}
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	service := Service{Configuration: configuration, HTTPClient: httpClient}

	_, errorValue := service.mattermostInteractionResolve(context.Background(), strings.NewReader(`{"dispatchID":"post-1"}`))
	if errorValue != nil {
		t.Fatalf("expected interaction resolve to succeed: %v", errorValue)
	}

	select {
	case payload := <-patchRequests:
		if _, hasMessage := payload["message"]; hasMessage {
			t.Fatalf("expected resolve patch not to update message body, got %+v", payload)
		}
		props := payload["props"].(map[string]any)
		attachments := props["attachments"].([]any)
		if len(attachments) != 0 {
			t.Fatalf("expected attachments to be cleared, got %+v", payload)
		}
	default:
		t.Fatal("expected patch request")
	}
}

func TestMattermostReactionAddCreatesBotReaction(t *testing.T) {
	reactionRequests := make(chan map[string]string, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
		case "/api/v4/reactions":
			if request.Method != http.MethodPost {
				t.Fatalf("unexpected reaction method: %s", request.Method)
			}
			var payload map[string]string
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected reaction payload to decode: %v", errorValue)
			}
			reactionRequests <- payload
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
	configuration := DefaultConfiguration()
	configuration.MattermostBaseURL = "http://mattermost.test"
	configuration.MattermostTokenPath = tokenPath
	service := Service{Configuration: configuration, HTTPClient: httpClient}
	request := httptest.NewRequest(http.MethodPost, "/v1/platform/mattermost/reaction.add", strings.NewReader(`{"messageID":"post-1","emojiName":"white_check_mark","reason":"consume"}`))
	request.SetPathValue("platform", "mattermost")
	responseRecorder := httptest.NewRecorder()

	service.handleReactionAdd(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected reaction response ok, got %d %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	select {
	case payload := <-reactionRequests:
		if payload["user_id"] != "bot-1" || payload["post_id"] != "post-1" || payload["emoji_name"] != "white_check_mark" {
			t.Fatalf("unexpected reaction payload: %+v", payload)
		}
	default:
		t.Fatal("expected reaction request")
	}
}

func TestMattermostProgressStartPublishesTypingImmediately(t *testing.T) {
	typingRequests := make(chan map[string]string, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
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
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected typing request before immediate stop can cancel progress")
	}
}

func TestMattermostProgressStartIgnoresTypingFailure(t *testing.T) {
	typingRequestCount := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/users/me":
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
		case "/api/v4/users/bot-1/typing":
			typingRequestCount++
			return testJSONResponse(http.StatusInternalServerError, map[string]string{"error": "typing unavailable"}), nil
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
		t.Fatalf("expected progress start to ignore typing failure: %v", errorValue)
	}
	if typingRequestCount != 1 {
		t.Fatalf("expected one initial typing attempt, got %d", typingRequestCount)
	}
	_, errorValue = service.mattermostStopProgressFromRequest(context.Background(), strings.NewReader(`{"replyTargetID":"`+replyTargetID+`"}`))
	if errorValue != nil {
		t.Fatalf("expected progress stop to succeed: %v", errorValue)
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

	reply, errorValue := service.mattermostReply(context.Background(), mustJSON(t, replyRequest{
		ReplyTargetID: replyTargetID,
		Message:       "captured",
		RawEventID:    "raw-event-1",
		OutboxID:      "outbox-1",
		Attachments:   []platformFileSpec{{DevicePath: attachmentPath, Filename: "screen.png", ContentType: "image/png", SizeBytes: 3}},
	}))
	if errorValue != nil {
		t.Fatalf("expected reply with attachment: %v", errorValue)
	}
	replyResult, isReplyResult := reply.(platformReplyResult)
	if !isReplyResult || replyResult.Platform != "mattermost" || replyResult.Visibility != "public" || replyResult.NativeAttachmentCount != 1 || len(replyResult.NativeAttachmentIDs) != 1 || replyResult.NativeAttachmentIDs[0] != "file-1" {
		t.Fatalf("expected Mattermost reply attachment evidence, got %+v", reply)
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

	reply, errorValue := service.mattermostReply(context.Background(), mustJSON(t, replyRequest{
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
	replyResult, isReplyResult := reply.(platformReplyResult)
	if !isReplyResult || replyResult.NativeAttachmentCount != 1 || len(replyResult.NativeAttachmentIDs) != 1 || replyResult.NativeAttachmentIDs[0] != "file-1" {
		t.Fatalf("expected inline attachment delivery evidence, got %+v", reply)
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

func TestMattermostImportAttachmentsWritesSanitizedDuplicateFilenames(t *testing.T) {
	workspacePath := t.TempDir()
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer mattermost-token" {
			t.Fatalf("expected mattermost bearer token, got %q", request.Header.Get("Authorization"))
		}
		switch request.URL.Path {
		case "/api/v4/files/file-1/info":
			return testJSONResponse(http.StatusOK, mattermostFileMetadata{ID: "file-1", Name: "../guide?.pdf", SizeBytes: 8, ContentType: "application/pdf"}), nil
		case "/api/v4/files/file-1":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("file-one")),
				Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			}, nil
		case "/api/v4/files/file-2/info":
			return testJSONResponse(http.StatusOK, mattermostFileMetadata{ID: "file-2", Name: "guide.pdf", SizeBytes: 8, ContentType: "application/pdf"}), nil
		case "/api/v4/files/file-2":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("file-two")),
				Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			}, nil
		default:
			t.Fatalf("unexpected mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{
			MattermostBaseURL:     "https://mattermost.test",
			MattermostTokenPath:   writePlatformTestFile(t, "mattermost-token"),
			BlueclawWorkspacePath: workspacePath,
		},
		HTTPClient: httpClient,
	}

	response, errorValue := service.mattermostImportAttachments(context.Background(), json.RawMessage(`{
		"messageID":"post-1",
		"targetDirectoryPath":"/workspace/private/people/person-1/inbox/mattermost/post-1",
		"inputAttachments":[
			{"platform":"mattermost","fileID":"file-1","messageID":"post-1"},
			{"platform":"mattermost","fileID":"file-2","messageID":"post-1"}
		]
	}`))
	if errorValue != nil {
		t.Fatalf("expected import to succeed: %v", errorValue)
	}
	if len(response.InputAttachments) != 2 {
		t.Fatalf("expected two imported attachments, got %+v", response.InputAttachments)
	}
	firstAttachment := response.InputAttachments[0]
	secondAttachment := response.InputAttachments[1]
	if !firstAttachment.IsAvailable || !secondAttachment.IsAvailable {
		t.Fatalf("expected attachments to be available, got %+v", response.InputAttachments)
	}
	if firstAttachment.Filename != "guide.pdf" || secondAttachment.Filename != "guide-2.pdf" {
		t.Fatalf("expected sanitized duplicate filenames, got %+v", response.InputAttachments)
	}
	if firstAttachment.Path != "/workspace/private/people/person-1/inbox/mattermost/post-1/guide.pdf" {
		t.Fatalf("expected virtual workspace path, got %q", firstAttachment.Path)
	}
	firstContent, errorValue := os.ReadFile(filepath.Join(workspacePath, "private", "people", "person-1", "inbox", "mattermost", "post-1", "guide.pdf"))
	if errorValue != nil {
		t.Fatalf("expected first imported file: %v", errorValue)
	}
	secondContent, errorValue := os.ReadFile(filepath.Join(workspacePath, "private", "people", "person-1", "inbox", "mattermost", "post-1", "guide-2.pdf"))
	if errorValue != nil {
		t.Fatalf("expected second imported file: %v", errorValue)
	}
	if string(firstContent) != "file-one" || string(secondContent) != "file-two" {
		t.Fatalf("expected imported file contents, got %q and %q", string(firstContent), string(secondContent))
	}
}

func TestMattermostImportAttachmentsVerifiesExistingFileContent(t *testing.T) {
	workspacePath := t.TempDir()
	downloadCount := 0
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/files/file-1/info":
			return testJSONResponse(http.StatusOK, mattermostFileMetadata{ID: "file-1", Name: "report.pdf", SizeBytes: 8, ContentType: "application/pdf"}), nil
		case "/api/v4/files/file-1":
			downloadCount++
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("document")),
				Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			}, nil
		default:
			t.Fatalf("unexpected mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{
			MattermostBaseURL:     "https://mattermost.test",
			MattermostTokenPath:   writePlatformTestFile(t, "mattermost-token"),
			BlueclawWorkspacePath: workspacePath,
		},
		HTTPClient: httpClient,
	}
	payload := json.RawMessage(`{
		"messageID":"post-1",
		"targetDirectoryPath":"/workspace/circles/staff/inbox/mattermost/thread-1/post-1",
		"inputAttachments":[{"platform":"mattermost","fileID":"file-1","messageID":"post-1"}]
	}`)

	firstResponse, errorValue := service.mattermostImportAttachments(context.Background(), payload)
	if errorValue != nil {
		t.Fatalf("expected first import to succeed: %v", errorValue)
	}
	secondResponse, errorValue := service.mattermostImportAttachments(context.Background(), payload)
	if errorValue != nil {
		t.Fatalf("expected second import to succeed: %v", errorValue)
	}
	if downloadCount != 2 {
		t.Fatalf("expected each import to verify downloaded content, got %d downloads", downloadCount)
	}
	if len(firstResponse.InputAttachments) != 1 || len(secondResponse.InputAttachments) != 1 {
		t.Fatalf("expected imported attachments, got first=%+v second=%+v", firstResponse.InputAttachments, secondResponse.InputAttachments)
	}
	firstAttachment := firstResponse.InputAttachments[0]
	secondAttachment := secondResponse.InputAttachments[0]
	if firstAttachment.Path != secondAttachment.Path || secondAttachment.Filename != "report.pdf" {
		t.Fatalf("expected existing import path to be reused, first=%+v second=%+v", firstAttachment, secondAttachment)
	}
	if _, errorValue := os.Stat(filepath.Join(workspacePath, "circles", "staff", "inbox", "mattermost", "thread-1", "post-1", "report-2.pdf")); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("expected no duplicate import file, stat error=%v", errorValue)
	}
}

func TestMattermostImportAttachmentsBuildsImageInputPart(t *testing.T) {
	workspacePath := t.TempDir()
	imageDocument := []byte("png-image")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/files/file-1/info":
			return testJSONResponse(http.StatusOK, mattermostFileMetadata{ID: "file-1", Name: "mascot.png", SizeBytes: int64(len(imageDocument)), ContentType: "image/png"}), nil
		case "/api/v4/files/file-1":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(imageDocument)),
				Header:     http.Header{"Content-Type": []string{"image/png"}},
			}, nil
		default:
			t.Fatalf("unexpected mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{
			MattermostBaseURL:     "https://mattermost.test",
			MattermostTokenPath:   writePlatformTestFile(t, "mattermost-token"),
			BlueclawWorkspacePath: workspacePath,
		},
		HTTPClient: httpClient,
	}

	response, errorValue := service.mattermostImportAttachments(context.Background(), json.RawMessage(`{
		"messageID":"post-1",
		"targetDirectoryPath":"/workspace/private/people/person-1/inbox/mattermost/post-1",
		"inputAttachments":[{"platform":"mattermost","fileID":"file-1","messageID":"post-1"}]
	}`))
	if errorValue != nil {
		t.Fatalf("expected import to succeed: %v", errorValue)
	}
	if len(response.InputParts) != 1 {
		t.Fatalf("expected one input part, got %+v", response.InputParts)
	}
	part := response.InputParts[0]
	if part.Type != "image" || part.Image == nil || part.Image.MimeType != "image/png" {
		t.Fatalf("expected image part, got %+v", part)
	}
	if part.Image.DataBase64 != base64.StdEncoding.EncodeToString(imageDocument) {
		t.Fatalf("expected image bytes to be base64 encoded, got %q", part.Image.DataBase64)
	}
	if part.File == nil || part.File.Path != "/workspace/private/people/person-1/inbox/mattermost/post-1/mascot.png" {
		t.Fatalf("expected image file metadata, got %+v", part.File)
	}
}

func TestMattermostImportAttachmentsBuildsMarkdownFilePart(t *testing.T) {
	workspacePath := t.TempDir()
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-file")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/files/file-1/info":
			return testJSONResponse(http.StatusOK, mattermostFileMetadata{ID: "file-1", Name: "report.pdf", SizeBytes: 8, ContentType: "application/pdf"}), nil
		case "/api/v4/files/file-1":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("pdf")),
				Header:     http.Header{"Content-Type": []string{"application/pdf"}},
			}, nil
		default:
			t.Fatalf("unexpected mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{
			MattermostBaseURL:     "https://mattermost.test",
			MattermostTokenPath:   writePlatformTestFile(t, "mattermost-token"),
			OpenRouterKeyPath:     secretPath,
			OpenRouterBaseURL:     "https://openrouter.test/api/v1/chat/completions",
			OpenRouterModel:       "openrouter/vision-model",
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    "/test/python",
		},
		HTTPClient: httpClient,
		RunCommand: func(_ context.Context, _ string, _ []string, input []byte) ([]byte, error) {
			var helperRequest fileReadHelperRequest
			if errorValue := json.Unmarshal(input, &helperRequest); errorValue != nil {
				t.Fatal(errorValue)
			}
			if helperRequest.OCRMode != "always" || helperRequest.OpenRouterAPIKey != "sk-file" || helperRequest.OpenRouterBaseURL != "https://openrouter.test/api/v1" || helperRequest.OpenRouterModel != "openrouter/vision-model" {
				t.Fatalf("expected OpenRouter OCR helper request, got %+v", helperRequest)
			}
			return []byte(`{"content":"# Report\n\nConverted content"}`), nil
		},
	}

	response, errorValue := service.mattermostImportAttachments(context.Background(), json.RawMessage(`{
		"messageID":"post-1",
		"targetDirectoryPath":"/workspace/private/people/person-1/inbox/mattermost/post-1",
		"inputAttachments":[{"platform":"mattermost","fileID":"file-1","messageID":"post-1"}]
	}`))
	if errorValue != nil {
		t.Fatalf("expected import to succeed: %v", errorValue)
	}
	if len(response.InputParts) != 1 || response.InputParts[0].Type != "file" || response.InputParts[0].File == nil {
		t.Fatalf("expected file input part, got %+v", response.InputParts)
	}
	filePart := response.InputParts[0].File
	if filePart.ConversionStatus != "converted" || !strings.Contains(filePart.MarkdownPreview, "Converted content") {
		t.Fatalf("expected converted markdown preview, got %+v", filePart)
	}
}

func TestMattermostImportAttachmentsConvertsHTMLThroughMarkItDown(t *testing.T) {
	workspacePath := t.TempDir()
	htmlDocument := "<!doctype html><html><head><style>@font-face{src:url(data:font/woff2;base64,AAAA)}</style></head><body><h1>Raw HTML Title</h1></body></html>"
	markItDownCalled := false
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/files/file-1/info":
			return testJSONResponse(http.StatusOK, mattermostFileMetadata{ID: "file-1", Name: "page.html", SizeBytes: int64(len(htmlDocument)), ContentType: "text/html"}), nil
		case "/api/v4/files/file-1":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(htmlDocument)),
				Header:     http.Header{"Content-Type": []string{"text/html"}},
			}, nil
		default:
			t.Fatalf("unexpected mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{
			MattermostBaseURL:     "https://mattermost.test",
			MattermostTokenPath:   writePlatformTestFile(t, "mattermost-token"),
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    "/test/python",
		},
		HTTPClient: httpClient,
		RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
			markItDownCalled = true
			return json.Marshal(map[string]any{"content": "# Raw HTML Title", "truncated": false})
		},
	}

	response, errorValue := service.mattermostImportAttachments(context.Background(), json.RawMessage(`{
		"messageID":"post-1",
		"targetDirectoryPath":"/workspace/private/people/person-1/inbox/mattermost/post-1",
		"inputAttachments":[{"platform":"mattermost","fileID":"file-1","messageID":"post-1"}]
	}`))
	if errorValue != nil {
		t.Fatalf("expected import to succeed: %v", errorValue)
	}
	if !markItDownCalled {
		t.Fatal("HTML attachment preview must go through MarkItDown conversion, not raw text")
	}
	if len(response.InputParts) != 1 || response.InputParts[0].File == nil {
		t.Fatalf("expected file input part, got %+v", response.InputParts)
	}
	filePart := response.InputParts[0].File
	if !strings.Contains(filePart.MarkdownPreview, "Raw HTML Title") || strings.Contains(filePart.MarkdownPreview, "@font-face") {
		t.Fatalf("expected converted text without font/style noise, got %+v", filePart)
	}
}

func TestCollapseInlineBase64ReplacesLongRunsWithMarker(t *testing.T) {
	blob := strings.Repeat("A", 5000)
	collapsed := collapseInlineBase64("intro " + blob + " outro")
	if strings.Contains(collapsed, blob) {
		t.Fatal("expected long base64 run to be collapsed")
	}
	if !strings.Contains(collapsed, "intro ") || !strings.Contains(collapsed, " outro") {
		t.Fatalf("expected surrounding text preserved, got %q", collapsed)
	}
	if !strings.Contains(collapsed, "base64 data omitted") {
		t.Fatalf("expected omission marker, got %q", collapsed)
	}
	if changed := collapseInlineBase64("short token abc123 normal prose"); changed != "short token abc123 normal prose" {
		t.Fatalf("expected short tokens untouched, got %q", changed)
	}
}

func TestMattermostImportAttachmentsKeepsUnsupportedFileMetadata(t *testing.T) {
	workspacePath := t.TempDir()
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/api/v4/files/file-1/info":
			return testJSONResponse(http.StatusOK, mattermostFileMetadata{ID: "file-1", Name: "archive.bin", SizeBytes: 4, ContentType: "application/octet-stream"}), nil
		case "/api/v4/files/file-1":
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("data")),
				Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
			}, nil
		default:
			t.Fatalf("unexpected mattermost request: %s", request.URL.Path)
			return testJSONResponse(http.StatusNotFound, map[string]string{}), nil
		}
	})}
	service := Service{
		Configuration: Configuration{
			MattermostBaseURL:     "https://mattermost.test",
			MattermostTokenPath:   writePlatformTestFile(t, "mattermost-token"),
			BlueclawWorkspacePath: workspacePath,
			FileReadPythonPath:    "/test/python",
		},
		HTTPClient: httpClient,
		RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
			return []byte("unsupported format"), os.ErrInvalid
		},
	}

	response, errorValue := service.mattermostImportAttachments(context.Background(), json.RawMessage(`{
		"messageID":"post-1",
		"targetDirectoryPath":"/workspace/private/people/person-1/inbox/mattermost/post-1",
		"inputAttachments":[{"platform":"mattermost","fileID":"file-1","messageID":"post-1"}]
	}`))
	if errorValue != nil {
		t.Fatalf("expected import to succeed: %v", errorValue)
	}
	if len(response.InputParts) != 1 || response.InputParts[0].Type != "file" || response.InputParts[0].File == nil {
		t.Fatalf("expected file input part, got %+v", response.InputParts)
	}
	filePart := response.InputParts[0].File
	if filePart.ConversionStatus != "failed" || filePart.MarkdownPreview != "" || !strings.Contains(filePart.Path, "archive.bin") {
		t.Fatalf("expected failed conversion with accessible file metadata, got %+v", filePart)
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
	if len(event.Context.InputAttachments) != 1 || event.Context.InputAttachments[0].FileID != "file-1" {
		t.Fatalf("expected poll file_ids to be forwarded as input attachments, got %+v", event.Context.InputAttachments)
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
		return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "username": "internkim", "is_bot": true})
	case request.URL.Path == "/api/v4/users/bot-1/channels":
		return testJSONResponse(http.StatusOK, []map[string]string{{"id": "dm-1", "type": "D", "name": "user-1__bot-1"}})
	case request.URL.Path == "/api/v4/channels/dm-1/posts" && request.URL.Query().Get("since") != "":
		return testJSONResponse(http.StatusOK, mattermostPostsResponse{
			Order: []string{"new-1", "old-1"},
			Posts: map[string]mattermostPolledPost{
				"old-1": {ID: "old-1", UserID: "user-1", ChannelID: "dm-1", Message: "old dm", CreateAt: 1000},
				"new-1": {ID: "new-1", UserID: "user-1", ChannelID: "dm-1", Message: "new dm", CreateAt: testFutureMattermostPostCreateAt, FileIDs: []string{"file-1"}},
			},
		})
	case request.URL.Path == "/api/v4/channels/dm-1/posts" && request.URL.Query().Get("per_page") == "21":
		return testJSONResponse(http.StatusOK, mattermostPostsResponse{
			Order: []string{"new-1"},
			Posts: map[string]mattermostPolledPost{
				"new-1": {ID: "new-1", UserID: "user-1", ChannelID: "dm-1", Message: "new dm", CreateAt: 2000, FileIDs: []string{"file-1"}},
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

func TestEnrichMattermostEventPreservesAttachmentsOnly(t *testing.T) {
	historyCursor, errorValue := encodePlatformHandle(platformHandle{
		Platform: "mattermost", ConversationID: "thread:channel-1:post-1",
		ChannelID: "channel-1", ChannelType: "O", MessageID: "post-1",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	service := mattermostRecoveryTestService(t, func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader(`{"order":[],"posts":{}}`)),
			Header:     make(http.Header),
		}, nil
	})
	event := platformInboundEvent{
		ConversationID: "thread:channel-1:post-1",
		Context: platformEventContext{
			HistoryCursor:    historyCursor,
			ConversationType: "O",
			AttachmentsOnly:  true,
			InputAttachments: []platformInputAttachment{{FileID: "file-1"}},
		},
	}
	enriched := service.enrichMattermostEvent(context.Background(), event)
	if !enriched.Context.AttachmentsOnly {
		t.Fatal("enrichMattermostEvent must preserve AttachmentsOnly through the context rebuild")
	}
}
