package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const requesterPubkeyForSending = "2222222222222222222222222222222222222222222222222222222222222222"

type senderStandIn struct {
	capabilitiesAsked []string
	sentAsRequester   []map[string]any
}

func senderStandInService(t *testing.T, standIn *senderStandIn) Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		standIn.capabilitiesAsked = append(standIn.capabilitiesAsked, request.URL.Path)
		switch request.URL.Path {
		case "/v1/platform/buzz/requester.conversations.list":
			_ = json.NewEncoder(responseWriter).Encode(map[string]any{"conversations": conversationsOf(messageSendChannelTestChannels())})
		case "/v1/platform/buzz/requester.message.send":
			var fields map[string]any
			_ = json.NewDecoder(request.Body).Decode(&fields)
			standIn.sentAsRequester = append(standIn.sentAsRequester, fields)
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"id": "posted-as-requester"})
		case "/v1/platform/buzz/message.post":
			_ = json.NewEncoder(responseWriter).Encode(map[string]string{"messageID": "posted-as-agent"})
		default:
			t.Fatalf("unexpected request %s %s", request.Method, request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return Service{Configuration: Configuration{ChatdEndpoint: server.URL, ChatdPlatform: "buzz"}}
}

func sendMessageAs(t *testing.T, service Service, input string) capabilities.ToolInvokeResponse {
	t.Helper()
	response, errorValue := service.invokePlatformMessageTool(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(input),
		Context: capabilities.ToolInvokeContext{
			RequesterEmail:          "member1@example.com",
			RequesterPlatformUserID: requesterPubkeyForSending,
			ChannelID:               "room-1",
			ReplyTargetID:           "buzz:room-1:root-1",
		},
	})
	if errorValue != nil {
		t.Fatalf("message_send failed outright: %v", errorValue)
	}
	return response
}

func TestAChannelMessageSentAsTheRequesterIsSignedByTheRequester(t *testing.T) {
	standIn := &senderStandIn{}
	service := senderStandInService(t, standIn)

	response := sendMessageAs(t, service, `{"targetType":"channel","channelName":"general","message":"확인 부탁드려요","sender":"requester"}`)

	if response.Status != "sent" {
		t.Fatalf("expected the message to be sent, got %+v", response)
	}
	if len(standIn.sentAsRequester) != 1 {
		t.Fatalf("expected one message under the requester's key, asked %v", standIn.capabilitiesAsked)
	}
	sent := standIn.sentAsRequester[0]
	if sent["conversationID"] != "channel-general" || sent["requesterPubkeyHex"] != requesterPubkeyForSending || sent["body"] != "확인 부탁드려요" {
		t.Fatalf("the message went somewhere other than the named channel or under another key: %v", sent)
	}
}

func TestTheCurrentChannelTakesAMessageAsTheRequester(t *testing.T) {
	standIn := &senderStandIn{}
	service := senderStandInService(t, standIn)

	response := sendMessageAs(t, service, `{"targetType":"currentChannel","message":"네","sender":"requester"}`)

	if response.Status != "sent" || len(standIn.sentAsRequester) != 1 || standIn.sentAsRequester[0]["conversationID"] != "room-1" {
		t.Fatalf("expected the reply in the current channel under the requester's key, got %+v %v", response, standIn.sentAsRequester)
	}
}

func TestAChannelMessageWithoutASenderStillGoesOutAsTheAgent(t *testing.T) {
	standIn := &senderStandIn{}
	service := senderStandInService(t, standIn)

	response := sendMessageAs(t, service, `{"targetType":"currentChannel","message":"네"}`)

	if response.Status != "sent" || len(standIn.sentAsRequester) != 0 {
		t.Fatalf("expected the agent's own post, got %+v %v", response, standIn.capabilitiesAsked)
	}
}

func TestTheAgentNeverWritesToAPersonInItsOwnName(t *testing.T) {
	standIn := &senderStandIn{}
	service := senderStandInService(t, standIn)

	response := sendMessageAs(t, service, `{"targetType":"directMessage","personHint":"이샘플","message":"안녕하세요","sender":"agent"}`)

	if response.ErrorCode != "sender_not_allowed" || len(standIn.capabilitiesAsked) != 0 {
		t.Fatalf("expected the refusal before anything is asked, got %+v %v", response, standIn.capabilitiesAsked)
	}
}

func TestAThreadReplyIsNotPostedAsTheRequester(t *testing.T) {
	standIn := &senderStandIn{}
	service := senderStandInService(t, standIn)

	response := sendMessageAs(t, service, `{"targetType":"currentThread","message":"네","sender":"requester"}`)

	if response.ErrorCode != "sender_unsupported" || len(standIn.capabilitiesAsked) != 0 {
		t.Fatalf("expected the thread reply to be refused, got %+v %v", response, standIn.capabilitiesAsked)
	}
}

func TestAMessageAsTheRequesterCarriesNoAttachments(t *testing.T) {
	standIn := &senderStandIn{}
	service := senderStandInService(t, standIn)

	response := sendMessageAs(t, service, `{"targetType":"channel","channelName":"general","message":"자료","attachments":["/workspace/report.pdf"],"sender":"requester"}`)

	if response.ErrorCode != "sender_unsupported" || len(standIn.capabilitiesAsked) != 0 {
		t.Fatalf("expected the refusal before anything is asked, got %+v %v", response, standIn.capabilitiesAsked)
	}
}
