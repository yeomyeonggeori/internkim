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
// the current text comes from the platform's record of that exact message, the
// span is applied once, and the whole result is sent.
func TestChatdMessageUpdateAppliesTheQuotedSpan(testContext *testing.T) {
	edited := ""
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/platform/buzz/message.search":
			body, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(body), `"messageIDs":["m2"]`) {
				testContext.Fatalf("expected the lookup to name the message ID, got %s", body)
			}
			writer.Write([]byte(`{"channelID":"channel-1","candidates":[{"messageID":"m2","channelID":"channel-1","authorPubkeyHex":"pub-agent","authoredByAssistant":true,"createdAt":1787886000000,"text":"번역 안내입니다","score":1}]}`))
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
		writer.Write([]byte(`{"channelID":"channel-1","candidates":[{"messageID":"m2","channelID":"channel-1","authorPubkeyHex":"pub-agent","authoredByAssistant":true,"createdAt":1787886000000,"text":"안내 안내","score":1}]}`))
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

// A post found by message_search lives wherever it lives; editing it from the
// thread the request came from must not depend on the post being in that
// thread's recent history. The lookup reads the exact message by ID, so a
// channel post stays editable from a conversation elsewhere.
func TestChatdMessageUpdateEditsAPostOutsideTheCurrentThread(testContext *testing.T) {
	edited := ""
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/platform/buzz/message.search":
			writer.Write([]byte(`{"channelID":"channel-2","candidates":[{"messageID":"post-9","channelID":"channel-2","authorPubkeyHex":"pub-agent","authoredByAssistant":true,"createdAt":1787886000000,"text":"조언 모음","score":1}]}`))
		case "/v1/platform/buzz/message.edit":
			body, _ := io.ReadAll(request.Body)
			edited = string(body)
			writer.Write([]byte(`{"dispatchID":"post-9"}`))
		default:
			testContext.Fatalf("unexpected chatd path %s", request.URL.Path)
		}
	}))
	defer chatdServer.Close()
	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}, HTTPClient: chatdServer.Client()}

	response, errorValue := service.invokePlatformMessageUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_update",
		Input:    json.RawMessage(`{"messageID":"post-9","oldText":"조언 모음","newText":"조언 모음 (원본 이미지 포함)"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", ReplyTargetID: "buzz:channel-1:thread-7"},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		testContext.Fatalf("expected the cross-thread edit to land, got %q", response.Content)
	}
	if !strings.Contains(edited, `"message":"조언 모음 (원본 이미지 포함)"`) {
		testContext.Fatalf("expected the edited text to travel, got %s", edited)
	}
}

// A message the platform no longer has, or never had, fails as not found
// instead of pretending the conversation window was too small.
func TestChatdMessageUpdateSaysADeletedMessageIsGone(testContext *testing.T) {
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Write([]byte(`{"channelID":"","candidates":[]}`))
	}))
	defer chatdServer.Close()
	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}, HTTPClient: chatdServer.Client()}

	response, errorValue := service.invokePlatformMessageUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_update",
		Input:    json.RawMessage(`{"messageID":"gone-1","oldText":"안내","newText":"공지"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", ReplyTargetID: "buzz:channel-1"},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "deleted") {
		testContext.Fatalf("expected a not-found refusal, answered %q", response.Content)
	}
}

// "Add the original image to that post" names no text span: attachments alone
// re-publish the message as it reads, with the file carried the way a send
// carries one — read by the requester, handed over as content.
func TestChatdMessageUpdateAddsAFileWithoutChangingTheText(testContext *testing.T) {
	workspaceDirectory := testContext.TempDir()

	var editBody map[string]any
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/platform/buzz/message.search":
			writer.Write([]byte(`{"channelID":"channel-2","candidates":[{"messageID":"post-9","channelID":"channel-2","authorPubkeyHex":"pub-agent","authoredByAssistant":true,"createdAt":1787886000000,"text":"조언 모음","score":1}]}`))
		case "/v1/platform/buzz/message.edit":
			json.NewDecoder(request.Body).Decode(&editBody)
			writer.Write([]byte(`{"dispatchID":"post-9"}`))
		default:
			testContext.Fatalf("unexpected chatd path %s", request.URL.Path)
		}
	}))
	defer chatdServer.Close()
	service := Service{Configuration: Configuration{
		ChatdEndpoint:         chatdServer.URL,
		ChatdPlatform:         "buzz",
		BlueclawWorkspacePath: workspaceDirectory,
	}}

	response, errorValue := service.invokePlatformMessageUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_update",
		Input:    json.RawMessage(`{"messageID":"post-9","attachments":["/workspace/원본.png"]}`),
		Transport: capabilities.ToolInvokeTransport{
			WorkspaceFiles: []capabilities.WorkspaceFile{{
				WorkspacePath: "/workspace/원본.png",
				Filename:      "원본.png",
				ContentBase64: base64.StdEncoding.EncodeToString([]byte("png")),
			}},
		},
		Context: capabilities.ToolInvokeContext{Platform: "buzz", ReplyTargetID: "buzz:channel-1"},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		testContext.Fatalf("expected the attachment-only edit to land, got %q", response.Content)
	}
	if editBody["message"] != "조언 모음" {
		testContext.Fatalf("an attachment-only edit must keep the text, sent %v", editBody["message"])
	}
	attachments, _ := editBody["attachments"].([]any)
	if len(attachments) != 1 {
		testContext.Fatalf("expected the file to travel with the edit, got %v", editBody["attachments"])
	}
}

func TestMattermostMessageUpdateRefusesAttachmentsLoudly(testContext *testing.T) {
	service := Service{}
	response, errorValue := service.invokePlatformMessageUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_update",
		Input:    json.RawMessage(`{"messageID":"post-1","attachments":["/workspace/원본.png"]}`),
		Context:  capabilities.ToolInvokeContext{Platform: "mattermost"},
	})
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(response.Content, "cannot be added") {
		testContext.Fatalf("attachments on a Mattermost update must refuse loudly, answered %q", response.Content)
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

func TestChatdMessageSendDeliversADirectMessageToAMember(testContext *testing.T) {
	memberPubkey := strings.Repeat("2", 64)
	var buzzKeyEmail string
	admindServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/admin/api/directory/people":
			json.NewEncoder(writer).Encode(map[string]any{"people": []map[string]any{{"memberID": "m-1", "email": "sample@example.com", "name": "이샘플"}}})
		case "/admin/api/directory/buzz-key":
			var payload struct {
				Email string `json:"email"`
			}
			json.NewDecoder(request.Body).Decode(&payload)
			buzzKeyEmail = payload.Email
			json.NewEncoder(writer).Encode(map[string]string{"pubkeyHex": memberPubkey})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer admindServer.Close()
	var receivedPath string
	var receivedPost struct {
		CounterpartPubkeyHex string `json:"counterpartPubkeyHex"`
		Message              string `json:"message"`
	}
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		json.NewDecoder(request.Body).Decode(&receivedPost)
		json.NewEncoder(writer).Encode(map[string]string{"channelID": "dm-uuid", "messageID": "event-9"})
	}))
	defer chatdServer.Close()

	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz", AdmindBaseURL: admindServer.URL}}
	response, errorValue := service.invokePlatformMessageSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"이샘플","message":"안내"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
	})
	if errorValue != nil {
		testContext.Fatalf("send failed: %v", errorValue)
	}
	if response.Outcome == capabilities.ToolOutcomeFailed {
		testContext.Fatalf("send answered failure: %s", response.Content)
	}
	if buzzKeyEmail != "sample@example.com" {
		testContext.Fatalf("buzz key was asked for %q", buzzKeyEmail)
	}
	if receivedPath != "/v1/platform/buzz/dm.post" {
		testContext.Fatalf("chatd received %q", receivedPath)
	}
	if receivedPost.CounterpartPubkeyHex != memberPubkey || receivedPost.Message != "안내" {
		testContext.Fatalf("chatd received %+v", receivedPost)
	}
	var result platformMessageSendResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		testContext.Fatalf("result decode failed: %v", errorValue)
	}
	if len(result.MessageIDs) != 1 || result.MessageIDs[0] != "event-9" {
		testContext.Fatalf("result carries %+v", result)
	}
}

// A hint the company cannot place must fail the send, never fall back to the
// conversation the request came from.
func TestChatdMessageSendToAStrangerFailsClosed(testContext *testing.T) {
	admindServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/admin/api/directory/people" {
			json.NewEncoder(writer).Encode(map[string]any{"people": []map[string]any{{"memberID": "m-1", "email": "sample@example.com", "name": "이샘플"}}})
			return
		}
		http.NotFound(writer, request)
	}))
	defer admindServer.Close()
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz", AdmindBaseURL: admindServer.URL}}
	response, errorValue := service.invokePlatformMessageSend(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"모르는사람","message":"안내"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
	})
	if errorValue != nil {
		testContext.Fatalf("send failed: %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed {
		testContext.Fatal("a stranger hint must fail the DM")
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

// chatd is the only judge of who may change a message, and it can only judge
// what it is told, so the actor travels with the edit and with the deletion.
func TestChatdMessageUpdateNamesTheRequesterToChatd(testContext *testing.T) {
	edited := ""
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/platform/buzz/message.search":
			writer.Write([]byte(`{"channelID":"channel-1","candidates":[{"messageID":"m2","channelID":"channel-1","authorPubkeyHex":"requester-pubkey","editable":true,"deletable":true,"createdAt":1787886000000,"text":"번역 안내입니다","score":1}]}`))
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
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", ReplyTargetID: "buzz:channel-1", RequesterPlatformUserID: "requester-pubkey"},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		testContext.Fatalf("expected the edit to land, got %q", response.Content)
	}
	if !strings.Contains(edited, `"requesterPubkeyHex":"requester-pubkey"`) {
		testContext.Fatalf("expected the edit to name who is asking, got %s", edited)
	}
}

func TestChatdPlatformDeleteNamesTheRequesterToChatd(testContext *testing.T) {
	deleted := ""
	service := Service{
		Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(request.Body)
			deleted = string(body)
			return testJSONResponse(http.StatusOK, map[string]any{}), nil
		})},
	}

	response, errorValue := service.invokePlatformMessageDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_delete",
		Input:    json.RawMessage(`{"messageIDs":["message-1"]}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", ReplyTargetID: "buzz:channel-1:message-0", RequesterPlatformUserID: "requester-pubkey"},
	})
	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded {
		testContext.Fatalf("expected the deletion to be carried out, got %+v", response)
	}
	if !strings.Contains(deleted, `"requesterPubkeyHex":"requester-pubkey"`) {
		testContext.Fatalf("expected the deletion to name who is asking, got %s", deleted)
	}
}

// A refusal is chatd's own sentence about its own matrix; capabilityd carries
// it to the model instead of rewriting it into a rule of its own.
func TestChatdMessageUpdateCarriesTheRefusalChatdWrote(testContext *testing.T) {
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/platform/buzz/message.search":
			writer.Write([]byte(`{"channelID":"channel-1","candidates":[{"messageID":"m2","channelID":"channel-1","authorPubkeyHex":"stranger-pubkey","editable":false,"deletable":false,"createdAt":1787886000000,"text":"번역 안내입니다","score":1}]}`))
		default:
			writer.WriteHeader(http.StatusForbidden)
			writer.Write([]byte(`{"error":"you may change a message the assistant sent, your own, or anyone's if you hold the channel admin role, and message m2 is none of those"}`))
		}
	}))
	defer chatdServer.Close()
	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}, HTTPClient: chatdServer.Client()}

	response, errorValue := service.invokePlatformMessageUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_update",
		Input:    json.RawMessage(`{"messageID":"m2","oldText":"번역 안내","newText":"수정된 안내"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", ReplyTargetID: "buzz:channel-1", RequesterPlatformUserID: "requester-pubkey"},
	})

	if errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed {
		testContext.Fatalf("expected the refusal to reach the model, got %q", response.Content)
	}
	if !strings.Contains(response.Content, "channel admin role") {
		testContext.Fatalf("expected chatd's own matrix in the failure, got %q", response.Content)
	}
}

// The relay accepts a change to a message only from its author, and chatd signs
// as the requester by asking admind which key that pubkey belongs to. An email
// is not a lookup key any more, so sending one would only invite chatd to
// derive an identity a second time.
func TestChatdMessageChangeNamesTheRequesterByKeyAlone(testContext *testing.T) {
	edited := ""
	deleted := ""
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		switch request.URL.Path {
		case "/v1/platform/buzz/message.search":
			writer.Write([]byte(`{"channelID":"channel-1","candidates":[{"messageID":"m2","channelID":"channel-1","authorPubkeyHex":"pub-requester","createdAt":1787886000000,"text":"먼저 쓴 글","score":1}]}`))
		case "/v1/platform/buzz/message.edit":
			edited = string(body)
			writer.Write([]byte(`{"dispatchID":"m2"}`))
		case "/v1/platform/buzz/message_delete":
			deleted = string(body)
			writer.Write([]byte(`{}`))
		default:
			testContext.Fatalf("unexpected chatd path %s", request.URL.Path)
		}
	}))
	defer chatdServer.Close()
	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}, HTTPClient: chatdServer.Client()}
	toolContext := capabilities.ToolInvokeContext{
		Platform:                "buzz",
		ReplyTargetID:           "buzz:channel-1",
		RequesterEmail:          "sample@example.com",
		RequesterPlatformUserID: "pub-requester",
	}

	if _, errorValue := service.invokePlatformMessageUpdate(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_update",
		Input:    json.RawMessage(`{"messageID":"m2","oldText":"먼저 쓴","newText":"고친"}`),
		Context:  toolContext,
	}); errorValue != nil {
		testContext.Fatal(errorValue)
	}
	if _, errorValue := service.invokePlatformMessageDelete(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_delete",
		Input:    json.RawMessage(`{"messageIDs":["m2"]}`),
		Context:  toolContext,
	}); errorValue != nil {
		testContext.Fatal(errorValue)
	}

	for _, sent := range []string{edited, deleted} {
		if !strings.Contains(sent, `"requesterPubkeyHex":"pub-requester"`) {
			testContext.Fatalf("expected the change to name the requester's key, got %s", sent)
		}
		if strings.Contains(sent, "requesterEmail") || strings.Contains(sent, "sample@example.com") {
			testContext.Fatalf("expected the change to carry no email at all, got %s", sent)
		}
	}
}
