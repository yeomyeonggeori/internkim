package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestChatdMessageSearchRoutesANamedChannelThroughChatd(testContext *testing.T) {
	var receivedPath string
	var receivedRequest chatdMessageSearchRequest
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		json.NewDecoder(request.Body).Decode(&receivedRequest)
		json.NewEncoder(writer).Encode(chatdMessageSearchResponse{
			ChannelID: "channel-uuid",
			Candidates: []chatdMessageSearchCandidate{
				{MessageID: "assistant-post", ChannelID: "channel-uuid", AuthorPubkeyHex: "bot-pubkey", AuthoredByAssistant: true, CreatedAt: 200, Text: "상하이 미팅 결과를 공유합니다", Score: 1},
				{MessageID: "requester-post", ChannelID: "channel-uuid", AuthorPubkeyHex: "requester-pubkey", CreatedAt: 100, Text: "상하이 미팅은 어땠나요?", Score: 0.9},
			},
		})
	}))
	defer chatdServer.Close()

	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}}
	response, errorValue := service.invokePlatformMessageSearch(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_search",
		Input:    json.RawMessage(`{"scope":"channel","channelName":"잡담","authoredBy":"assistant","queries":["상하이 미팅"]}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", RequesterPlatformUserID: "requester-pubkey"},
	})
	if errorValue != nil {
		testContext.Fatalf("search failed: %v", errorValue)
	}
	if response.Outcome == capabilities.ToolOutcomeFailed {
		testContext.Fatalf("search answered failure: %s", response.Content)
	}
	if receivedPath != "/v1/platform/buzz/message.search" {
		testContext.Fatalf("chatd received %q", receivedPath)
	}
	if receivedRequest.ChannelName != "잡담" || receivedRequest.AuthoredBy != "assistant" {
		testContext.Fatalf("chatd received %+v", receivedRequest)
	}
	if receivedRequest.RequesterPubkeyHex != "requester-pubkey" {
		testContext.Fatalf("requester pubkey missing from %+v", receivedRequest)
	}
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		testContext.Fatalf("result decode failed: %v", errorValue)
	}
	if len(result.MessageIDs) != 1 || result.MessageIDs[0] != "assistant-post" {
		testContext.Fatalf("deletable IDs carry %+v", result.MessageIDs)
	}
	if len(result.Candidates) != 2 {
		testContext.Fatalf("candidates carry %+v", result.Candidates)
	}
	if result.Candidates[0].AuthoredBy != "assistant" || !result.Candidates[0].Deletable {
		testContext.Fatalf("assistant candidate reads %+v", result.Candidates[0])
	}
	if result.Candidates[1].AuthoredBy != "requester" || result.Candidates[1].Deletable {
		testContext.Fatalf("requester candidate reads %+v", result.Candidates[1])
	}
	if !strings.Contains(result.Candidates[0].Preview, "상하이 미팅") {
		testContext.Fatalf("preview reads %q", result.Candidates[0].Preview)
	}
}

func TestChatdMessageSearchDefaultsToTheCurrentThread(testContext *testing.T) {
	var receivedRequest chatdMessageSearchRequest
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		json.NewDecoder(request.Body).Decode(&receivedRequest)
		json.NewEncoder(writer).Encode(chatdMessageSearchResponse{ChannelID: "channel-uuid"})
	}))
	defer chatdServer.Close()

	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}}
	response, errorValue := service.invokePlatformMessageSearch(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_search",
		Input:    json.RawMessage(`{"queries":["회식"]}`),
		Context: capabilities.ToolInvokeContext{
			Platform:      "buzz",
			ReplyTargetID: "buzz:channel-uuid:root-event",
		},
	})
	if errorValue != nil {
		testContext.Fatalf("search failed: %v", errorValue)
	}
	if response.Outcome == capabilities.ToolOutcomeFailed {
		testContext.Fatalf("search answered failure: %s", response.Content)
	}
	if receivedRequest.RootMessageID != "root-event" || receivedRequest.ReplyTargetID != "buzz:channel-uuid:root-event" {
		testContext.Fatalf("chatd received %+v", receivedRequest)
	}
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		testContext.Fatalf("result decode failed: %v", errorValue)
	}
	if result.Scope != "currentThread" {
		testContext.Fatalf("scope reads %q", result.Scope)
	}
}

func TestChatdMessageSearchNamesTheUnroutedDirectMessageScope(testContext *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:18090", ChatdPlatform: "buzz"}}
	response, errorValue := service.invokePlatformMessageSearch(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_search",
		Input:    json.RawMessage(`{"scope":"directMessage","personHint":"이샘플"}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
	})
	if errorValue != nil {
		testContext.Fatalf("search failed: %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed {
		testContext.Fatal("directMessage scope must fail loudly until it is routed")
	}
	if !strings.Contains(response.Content, "directMessage") {
		testContext.Fatalf("failure reads %q", response.Content)
	}
}

func TestChatdMessageContextAnswersFromTheConversationItself(testContext *testing.T) {
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/platform/buzz/identity.self" {
			http.NotFound(writer, request)
			return
		}
		json.NewEncoder(writer).Encode(chatdIdentitySelfResponse{PubkeyHex: "bot-pubkey", Name: "김인턴"})
	}))
	defer chatdServer.Close()

	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}}
	response, errorValue := service.invokeChatdPlatformMessageContext(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_context",
		Input:    json.RawMessage(`{}`),
		Context: capabilities.ToolInvokeContext{
			Platform:                "buzz",
			ConversationID:          "buzz:channel-uuid:root-event",
			ConversationType:        "channel",
			ChannelName:             "잡담",
			ReplyTargetID:           "buzz:channel-uuid:root-event",
			RequesterPersonID:       "person-1",
			RequesterPlatformUserID: "requester-pubkey",
		},
	})
	if errorValue != nil {
		testContext.Fatalf("context failed: %v", errorValue)
	}
	if response.Outcome == capabilities.ToolOutcomeFailed {
		testContext.Fatalf("context answered failure: %s", response.Content)
	}
	var result platformMessageContextResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		testContext.Fatalf("result decode failed: %v", errorValue)
	}
	if result.Platform != "buzz" || result.ChannelID != "channel-uuid" || result.RootMessageID != "root-event" {
		testContext.Fatalf("context reads %+v", result)
	}
	if result.RequesterPlatformUserID != "requester-pubkey" {
		testContext.Fatalf("requester reads %+v", result)
	}
	if result.BotUserID != "bot-pubkey" || result.BotUsername != "김인턴" {
		testContext.Fatalf("bot identity reads %+v", result)
	}
}

func TestMessageDeleteApprovalPreviewQuotesTheTargets(testContext *testing.T) {
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/platform/buzz/message.search" {
			http.NotFound(writer, request)
			return
		}
		json.NewEncoder(writer).Encode(chatdMessageSearchResponse{Candidates: []chatdMessageSearchCandidate{
			{MessageID: "m1", Text: "2026년에 SaaS를 만든다고? 제가 드리는 최고의 조언", AuthoredByAssistant: true},
		}})
	}))
	defer chatdServer.Close()

	service := Service{Configuration: Configuration{ChatdEndpoint: chatdServer.URL, ChatdPlatform: "buzz"}}
	response, errorValue := service.resolveMessageDeleteTarget(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_delete",
		Input:    json.RawMessage(`{"messageIDs":["m1"]}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
	})
	if errorValue != nil {
		testContext.Fatalf("resolve failed: %v", errorValue)
	}
	var target capabilityToolTarget
	if errorValue := json.Unmarshal(response.Result, &target); errorValue != nil {
		testContext.Fatalf("target decode failed: %v", errorValue)
	}
	if !strings.Contains(target.Preview, "2026년에 SaaS를 만든다고?") {
		testContext.Fatalf("preview reads %q, want the message's own words", target.Preview)
	}
	if target.InputField != "" || target.ID != "" {
		testContext.Fatalf("a preview must not narrow the replayed input, got %+v", target)
	}
}

func TestMessageDeleteApprovalPreviewNeverBlocksOnALookupFailure(testContext *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:1", ChatdPlatform: "buzz"}}
	response, errorValue := service.resolveMessageDeleteTarget(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_delete",
		Input:    json.RawMessage(`{"messageIDs":["m1"]}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz"},
	})
	if errorValue != nil {
		testContext.Fatalf("resolve failed: %v", errorValue)
	}
	if response.Status != "no_target" {
		testContext.Fatalf("an unreachable lookup must resolve to no target, got %q", response.Status)
	}
}
