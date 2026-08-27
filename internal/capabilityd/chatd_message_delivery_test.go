package capabilityd

import (
	"context"
	"encoding/json"
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

func TestChatdMessageSendCarriesWorkspaceAttachmentsAsDevicePaths(testContext *testing.T) {
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
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
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
	if len(receivedRequest.Attachments) != 1 || receivedRequest.Attachments[0].DevicePath != attachmentPath {
		testContext.Fatalf("attachments carried %+v", receivedRequest.Attachments)
	}
}

func TestChatdPlatformKeepsMattermostToolsHonestlyUnrouted(testContext *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"}}
	for toolName, invoke := range map[string]func(context.Context, capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error){
		"message_search": service.invokePlatformMessageSearch,
		"message_update": service.invokePlatformMessageUpdate,
		"message_delete": service.invokePlatformMessageDelete,
	} {
		response, errorValue := invoke(context.Background(), capabilities.ToolInvokeRequest{
			ToolName: toolName,
			Input:    json.RawMessage(`{}`),
			Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
		})
		if errorValue != nil {
			testContext.Fatalf("%s failed: %v", toolName, errorValue)
		}
		if response.Outcome != capabilities.ToolOutcomeFailed {
			testContext.Fatalf("%s should refuse on a chatd platform", toolName)
		}
		if !strings.Contains(response.Content, "not yet available") {
			testContext.Fatalf("%s answered %q", toolName, response.Content)
		}
	}
}

func TestChatdMessageSendRefusesDirectMessagesLoudly(testContext *testing.T) {
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
		testContext.Fatal("direct messages are not routed yet and must refuse loudly")
	}
	if !strings.Contains(response.Content, "currentChannel") {
		testContext.Fatalf("refusal should point at a working target, answered %q", response.Content)
	}
}
