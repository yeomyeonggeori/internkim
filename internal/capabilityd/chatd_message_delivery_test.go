package capabilityd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestChatdServesPlatformMatchesOnlyTheConfiguredPlatform(testContext *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"}}
	if !service.chatdServesPlatform("buzz") {
		testContext.Fatal("configured platform should route through chatd")
	}
	if service.chatdServesPlatform("mattermost") {
		testContext.Fatal("other platforms must keep their native route")
	}
	unconfigured := Service{Configuration: Configuration{ChatdPlatform: "buzz"}}
	if unconfigured.chatdServesPlatform("buzz") {
		testContext.Fatal("a missing endpoint must not route through chatd")
	}
}

func TestChatdMessageSendPostsCurrentChannelThroughChatd(testContext *testing.T) {
	var receivedPath string
	var receivedRequest chatdMessagePostRequest
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		json.NewDecoder(request.Body).Decode(&receivedRequest)
		json.NewEncoder(writer).Encode(chatdMessagePostResponse{MessageID: "event-7", ChannelID: "channel-uuid"})
	}))
	defer chatdServer.Close()

	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}}
	response, errorValue := service.invokePlatformMessageSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(`{"targetType":"currentChannel","message":"안내"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", ChannelID: "channel-uuid"},
	})
	if errorValue != nil {
		testContext.Fatalf("send failed: %v", errorValue)
	}
	if response.Outcome == capabilities.ToolOutcomeFailed {
		testContext.Fatalf("send answered failure: %s", response.Content)
	}
	if receivedPath != "/v1/platform/buzz/message.post" {
		testContext.Fatalf("chatd received %q", receivedPath)
	}
	if receivedRequest.ChannelID != "channel-uuid" || receivedRequest.Message != "안내" {
		testContext.Fatalf("chatd received %+v", receivedRequest)
	}
	var result platformMessageSendResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		testContext.Fatalf("result decode failed: %v", errorValue)
	}
	if len(result.MessageIDs) != 1 || result.MessageIDs[0] != "event-7" {
		testContext.Fatalf("result carries %+v", result)
	}
}

// The file is read by the person sending it and arrives as content; chatd is
// handed a copy on disk beside this daemon, never a reach into the workspace.
func TestChatdMessageSendCarriesAFileSomebodyElseRead(testContext *testing.T) {
	workspaceDirectory := testContext.TempDir()
	attachmentPath := filepath.Join(workspaceDirectory, "지도.png")
	if errorValue := os.WriteFile(attachmentPath, []byte("png"), 0o600); errorValue != nil {
		testContext.Fatal(errorValue)
	}

	var receivedRequest chatdMessagePostRequest
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		json.NewDecoder(request.Body).Decode(&receivedRequest)
		json.NewEncoder(writer).Encode(chatdMessagePostResponse{MessageID: "event-8"})
	}))
	defer chatdServer.Close()

	service := Service{Configuration: Configuration{
		ChatdEndpoint:         chatdServer.URL,
		ChatdPlatform:         "buzz",
		BlueclawWorkspacePath: workspaceDirectory,
	}}
	response, errorValue := service.invokePlatformMessageSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(`{"targetType":"channel","channelName":"잡담","message":"번역본입니다","attachments":["/workspace/지도.png"]}`),
		Transport: capabilities.ToolInvokeTransport{
			WorkspaceFiles: []capabilities.WorkspaceFile{{
				WorkspacePath: "/workspace/지도.png",
				Filename:      "지도.png",
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("png")),
			}},
		},
		Context: capabilities.ToolInvokeContext{Platform: "buzz"},
	})
	if errorValue != nil {
		testContext.Fatalf("send failed: %v", errorValue)
	}
	if response.Outcome == capabilities.ToolOutcomeFailed {
		testContext.Fatalf("send answered failure: %s", response.Content)
	}
	if receivedRequest.ChannelName != "잡담" {
		testContext.Fatalf("chatd received %+v", receivedRequest)
	}
	if len(receivedRequest.Attachments) != 1 || filepath.Base(receivedRequest.Attachments[0].DevicePath) != "지도.png" {
		testContext.Fatalf("attachments carried %+v", receivedRequest.Attachments)
	}
}

// The search a chatd platform answers is the conversation's own record. Asked
// with no target at all, it says what to name instead of guessing.
func TestChatdMessageSearchNeedsATargetToRead(testContext *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"}}
	response, errorValue := service.invokePlatformMessageSearch(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_search",
		Input:    json.RawMessage(`{}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "channelName") {
		testContext.Fatalf("expected guidance toward a channel, answered %q", response.Content)
	}
}

func TestChatdMessageSearchReadsAChannelsOwnRecord(testContext *testing.T) {
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/platform/buzz/message.search" {
			testContext.Fatalf("unexpected chatd path %s", request.URL.Path)
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"channelName":"잡담"`) || !strings.Contains(string(body), `"authoredBy":"assistant"`) {
			testContext.Fatalf("expected the channel name and author to travel, got %s", body)
		}
		writer.Write([]byte(`{"channelID":"channel-1","candidates":[
			{"messageID":"m2","channelID":"channel-1","authorPubkeyHex":"pub-agent","authoredByAssistant":true,"createdAt":1787886000000,"text":"번역 안내","score":1}
		]}`))
	}))
	defer chatdServer.Close()
	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}, HTTPClient: chatdServer.Client()}

	response, errorValue := service.invokePlatformMessageSearch(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_search",
		Input:    json.RawMessage(`{"channelName":"잡담","authoredBy":"assistant"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		testContext.Fatalf("expected the search to answer, got %q", response.Content)
	}
	result := string(response.Result)
	if !strings.Contains(result, `"m2"`) || strings.Contains(result, `"m1"`) {
		testContext.Fatalf("expected only the agent's own message, got %s", result)
	}
}

// The edit contract is a quoted span, the same one the Mattermost path holds:
// the current text comes from the conversation's own record, the span is
// applied once, and the whole result is sent.
func TestChatdMessageUpdateAppliesTheQuotedSpan(testContext *testing.T) {
	edited := ""
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/platform/buzz/history.fetch":
			writer.Write([]byte(`{"channelID":"channel-1","messages":[{"id":"m2","speaker":"김인턴","text":"번역 안내입니다","isBot":true}]}`))
		case "/v1/platform/buzz/message.edit":
			body, _ := io.ReadAll(request.Body)
			edited = string(body)
			writer.Write([]byte(`{"dispatchID":"m2"}`))
		default:
			testContext.Fatalf("unexpected chatd path %s", request.URL.Path)
		}
	}))
	defer chatdServer.Close()
	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}, HTTPClient: chatdServer.Client()}

	response, errorValue := service.invokePlatformMessageUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_update",
		Input:    json.RawMessage(`{"messageID":"m2","oldText":"번역 안내","newText":"수정된 안내"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", ReplyTargetID: "buzz:channel-1"},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		testContext.Fatalf("expected the edit to land, got %q", response.Content)
	}
	if !strings.Contains(edited, `"message":"수정된 안내입니다"`) {
		testContext.Fatalf("expected the span applied to the current text, got %s", edited)
	}
}

func TestChatdMessageUpdateRefusesASpanItCannotAnchor(testContext *testing.T) {
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"channelID":"channel-1","messages":[{"id":"m2","speaker":"김인턴","text":"안내 안내","isBot":true}]}`))
	}))
	defer chatdServer.Close()
	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}, HTTPClient: chatdServer.Client()}

	response, errorValue := service.invokePlatformMessageUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_update",
		Input:    json.RawMessage(`{"messageID":"m2","oldText":"안내","newText":"공지"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", ReplyTargetID: "buzz:channel-1"},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "occurs once") {
		testContext.Fatalf("expected the ambiguous span to be refused, answered %q", response.Content)
	}
}

func TestChatdMessageSendRefusesDirectMessagesLoudly(testContext *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"}}
	response, errorValue := service.invokePlatformMessageSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(`{"targetType":"directMessage","message":"안내"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
	})
	if errorValue != nil {
		testContext.Fatalf("send failed: %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed {
		testContext.Fatal("direct messages are not routed yet and must refuse loudly")
	}
	if !strings.Contains(response.Content, "currentChannel") {
		testContext.Fatalf("refusal should point at a working target, answered %q", response.Content)
	}
}

// A DM aimed at another person has no working substitute: posting the content
// into the current conversation answers a different request than the one that
// was made, and the completion judge then reports a delivery that never
// happened. The refusal must forbid the substitute, not suggest it.
func TestChatdMessageSendToAnotherPersonForbidsTheSubstitute(testContext *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"}}
	response, errorValue := service.invokePlatformMessageSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"이샘플","message":"안내"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
	})
	if errorValue != nil {
		testContext.Fatalf("send failed: %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed {
		testContext.Fatal("a DM to another person is not routed yet and must refuse loudly")
	}
	if strings.Contains(response.Content, "currentChannel") {
		testContext.Fatalf("refusal must not steer the DM into the current conversation, answered %q", response.Content)
	}
	if !strings.Contains(response.Content, "could not be sent") {
		testContext.Fatalf("refusal should demand an honest failure report, answered %q", response.Content)
	}
}

// chatd has been able to delete a message as the person who wrote it since the
// messenger screen learned to. Only the route from here was missing, and the
// agent was told deletion did not exist on this platform.
func TestChatdPlatformDeletesAMessageThroughChatd(testContext *testing.T) {
	askedPath := ""
	service := Service{
		Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			askedPath = request.URL.Path
			return testJSONResponse(http.StatusOK, map[string]any{}), nil
		})},
	}

	response, errorValue := service.invokePlatformMessageDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_delete",
		Input:    json.RawMessage(`{"messageIDs":["message-1"]}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", ReplyTargetID: "buzz:channel-1:message-0"},
	})
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		testContext.Fatalf("expected the deletion to be carried out, got %+v", response)
	}
	if !strings.Contains(askedPath, "message_delete") {
		testContext.Fatalf("expected chatd to be asked to delete, it was asked %q", askedPath)
	}
}

// Run f06ce78f posted the full translation into the DM: the model passed the
// direct conversation's own channel id as the channel target, and nothing
// examined it. An id equal to the room the request came from is not a channel.
func TestAChannelPostAimedAtTheConversationItselfIsRefused(testContext *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"}}

	response, errorValue := service.invokePlatformMessageSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(`{"targetType":"channel","channelID":"6955ae67-dm","message":"안내"}`),
		Context: capabilities.ToolInvokeContext{
			Platform:         "buzz",
			ConversationType: "direct",
			ChannelID:        "6955ae67-dm",
		},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "channelName") {
		testContext.Fatalf("expected guidance toward naming the channel, answered %q", response.Content)
	}
}
