package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func TestChatdMessageSearchRoutesANamedChannelThroughChatd(testContext *testing.T) {
	var receivedPath string
	var receivedRequest chatdMessageSearchRequest
	chatdServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/v1/platform/buzz/channels.list":
			testContext.Error("the channel was looked for among the agent's rooms")
		case "/v1/platform/buzz/requester.conversations.list":
			json.NewEncoder(writer).Encode(map[string]any{"conversations": []requesterConversation{{ID: "channel-uuid", Name: "잡담", Kind: "group"}}})
			return
		case "/v1/platform/buzz/requester.channels.open.list":
			json.NewEncoder(writer).Encode(map[string]any{"channels": []requesterConversation{}})
			return
		}
		receivedPath = request.URL.Path
		json.NewDecoder(request.Body).Decode(&receivedRequest)
		json.NewEncoder(writer).Encode(chatdMessageSearchResponse{
			ChannelID: "channel-uuid",
			Candidates: []chatdMessageSearchCandidate{
				{MessageID: "assistant-post", ChannelID: "channel-uuid", AuthorPubkeyHex: "bot-pubkey", AuthoredByAssistant: true, Editable: true, Deletable: true, CreatedAt: 200, Text: "상하이 미팅 결과를 공유합니다", Score: 1},
				{MessageID: "requester-post", ChannelID: "channel-uuid", AuthorPubkeyHex: "requester-pubkey", Editable: true, Deletable: true, CreatedAt: 100, Text: "상하이 미팅은 어땠나요?", Score: 0.9},
				{MessageID: "stranger-post", ChannelID: "channel-uuid", AuthorPubkeyHex: "stranger-pubkey", CreatedAt: 50, Text: "상하이 미팅 사진입니다", Score: 0.8},
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
	if receivedRequest.ChannelID != "channel-uuid" || receivedRequest.AuthoredBy != "assistant" || receivedRequest.ReadAsRequester {
		testContext.Fatalf("chatd received %+v", receivedRequest)
	}
	if receivedRequest.RequesterPubkeyHex != "requester-pubkey" {
		testContext.Fatalf("requester pubkey missing from %+v", receivedRequest)
	}
	var result platformMessageSearchResult
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		testContext.Fatalf("result decode failed: %v", errorValue)
	}
	if len(result.MessageIDs) != 2 || result.MessageIDs[0] != "assistant-post" || result.MessageIDs[1] != "requester-post" {
		testContext.Fatalf("deletable IDs carry %+v", result.MessageIDs)
	}
	if len(result.Candidates) != 3 {
		testContext.Fatalf("candidates carry %+v", result.Candidates)
	}
	if result.Candidates[0].AuthoredBy != "assistant" || !result.Candidates[0].Deletable || !result.Candidates[0].Editable {
		testContext.Fatalf("assistant candidate reads %+v", result.Candidates[0])
	}
	if result.Candidates[1].AuthoredBy != "requester" || !result.Candidates[1].Deletable || !result.Candidates[1].Editable {
		testContext.Fatalf("requester candidate reads %+v", result.Candidates[1])
	}
	if result.Candidates[2].AuthoredBy != "anyone" || result.Candidates[2].Deletable || result.Candidates[2].Editable {
		testContext.Fatalf("stranger candidate reads %+v", result.Candidates[2])
	}
	if result.Candidates[2].ProtectedReason != "" {
		testContext.Fatalf("capabilityd must not invent a reason chatd did not give: %q", result.Candidates[2].ProtectedReason)
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
			Platform:                "buzz",
			ReplyTargetID:           "buzz:channel-uuid:root-event",
			RequesterPlatformUserID: "requester-pubkey",
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

func searchingAsTheRequester(request *http.Request) (int, string) {
	if strings.HasSuffix(request.URL.Path, "/identity.self") {
		return http.StatusOK, `{"pubkeyHex":"` + agentPubkeyHex + `","name":"김인턴"}`
	}
	if strings.HasSuffix(request.URL.Path, "/requester.conversations.list") {
		return http.StatusOK, `{"conversations":[` +
			`{"id":"channel-1","name":"전사-공지","kind":"group"},` +
			`{"id":"dm-agent","kind":"dm","participantExternalIDs":["` + requesterPubkeyHex + `","` + agentPubkeyHex + `"]},` +
			`{"id":"dm-1","kind":"dm","participantExternalIDs":["` + requesterPubkeyHex + `","` + colleaguePubkeyHex + `"]}]}`
	}
	asked, _ := io.ReadAll(request.Body)
	if strings.Contains(string(asked), `"readAsRequester":true`) {
		return http.StatusOK, `{"candidates":[{"messageID":"said-in-private","channelID":"dm-1","authorPubkeyHex":"` + requesterPubkeyHex + `","text":"둘만 나눈 이야기"}]}`
	}
	return http.StatusOK, `{"candidates":[]}`
}

const agentPubkeyHex = "a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f90"

func searchAskedOfChatd(t *testing.T, chatd *standingIn) []chatdMessageSearchRequest {
	t.Helper()
	searches := []chatdMessageSearchRequest{}
	for _, call := range askedOf(chatd, "message.search") {
		var search chatdMessageSearchRequest
		if errorValue := json.Unmarshal([]byte(call.Body), &search); errorValue != nil {
			t.Fatalf("chatd was asked %q: %v", call.Body, errorValue)
		}
		searches = append(searches, search)
	}
	return searches
}

func askedThroughTheAPI(toolName string, input string) capabilities.ToolInvokeRequest {
	request := channelToolCall(toolName, input)
	request.Context.TaskSource = capabilities.TaskSourcePublicAPI
	return request
}

func TestEveryReadNamesTheRequesterAndIsTheirsAloneOnlyOnTheirOwnCall(t *testing.T) {
	inAConversation := []string{
		`{"queries":["이야기"]}`,
		`{"scope":"channel","channelID":"channel-1"}`,
		`{"scope":"channel","channelName":"전사-공지"}`,
		`{"scope":"directMessage"}`,
		`{"scope":"directMessage","personHint":"박예시"}`,
		`{"personHint":"박예시"}`,
		`{"messageIDs":["said-in-private"]}`,
	}
	throughTheAgent := func(toolName string, input string) capabilities.ToolInvokeRequest {
		return inAChannel(channelToolCall(toolName, input))
	}
	cases := []struct {
		calling                func(string, string) capabilities.ToolInvokeRequest
		inputs                 []string
		isTheRequestersOwnCall bool
	}{
		{calling: throughTheAgent, inputs: inAConversation},
		{calling: askedThroughTheAPI, inputs: inAConversation[1:], isTheRequestersOwnCall: true},
	}
	for _, caller := range cases {
		for _, input := range caller.inputs {
			chatd := answeringPerCall(searchingAsTheRequester)

			_, _ = channelService(t, chatd).invokePlatformMessageSearch(context.Background(), caller.calling("message_search", input))

			searches := searchAskedOfChatd(t, chatd)
			if len(searches) != 1 || searches[0].RequesterPubkeyHex != requesterPubkeyHex || searches[0].ReadAsRequester != caller.isTheRequestersOwnCall {
				t.Fatalf("%s (own call: %v): chatd was asked %+v", input, caller.isTheRequestersOwnCall, searches)
			}
			if len(askedOf(chatd, "channels.list")) != 0 {
				t.Fatalf("%s: the channel was looked for among the agent's rooms", input)
			}
		}
	}
}

func TestADirectConversationIsReadAsTheRequesterOnTheirOwnCall(t *testing.T) {
	conversationOf := map[string]string{
		`{"scope":"directMessage","personHint":"박예시","queries":["이야기"]}`: "dm-1",
		`{"scope":"directMessage"}`: "dm-agent",
		`{"personHint":"박예시"}`:      "dm-1",
	}
	for input, conversationID := range conversationOf {
		chatd := answeringPerCall(searchingAsTheRequester)

		answered, errorValue := channelService(t, chatd).invokePlatformMessageSearch(context.Background(), askedThroughTheAPI("message_search", input))

		if errorValue != nil || answered.IsError {
			t.Fatalf("%s: expected the conversation to be read, got %+v %v", input, answered, errorValue)
		}
		searches := searchAskedOfChatd(t, chatd)
		if len(searches) != 1 || searches[0].ChannelID != conversationID || !searches[0].ReadAsRequester || searches[0].RequesterPubkeyHex != requesterPubkeyHex {
			t.Fatalf("%s: expected %s read with the requester's key, chatd was asked %+v", input, conversationID, searches)
		}
		if !strings.Contains(string(answered.Result), `"scope":"directMessage"`) {
			t.Fatalf("%s: the answer names another scope than the one read: %s", input, answered.Result)
		}
		if !strings.Contains(string(answered.Result), `"authoredBy":"requester"`) {
			t.Fatalf("%s: the requester's own words are named theirs, answered %s", input, answered.Result)
		}
	}
}

func TestADirectConversationNobodyHasIsNotSearched(t *testing.T) {
	chatd := answeringPerCall(func(request *http.Request) (int, string) {
		if strings.HasSuffix(request.URL.Path, "/requester.conversations.list") {
			return http.StatusOK, `{"conversations":[]}`
		}
		return searchingAsTheRequester(request)
	})

	answered, _ := channelService(t, chatd).invokePlatformMessageSearch(context.Background(),
		askedThroughTheAPI("message_search", `{"scope":"directMessage","personHint":"박예시"}`))

	if !answered.IsError || !strings.Contains(string(answered.Result), `"errorCode":"conversation_not_found"`) || len(searchAskedOfChatd(t, chatd)) != 0 {
		t.Fatalf("expected a refusal and no search, got %+v after %+v", answered, chatd.asked)
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

func TestMessageContextOutsideAConversationIsRefusedRatherThanAnsweredEmpty(testContext *testing.T) {
	service := Service{Configuration: Configuration{ChatdEndpoint: "http://127.0.0.1:1", ChatdPlatform: "buzz"}}
	response, errorValue := service.invokeChatdPlatformMessageContext(context.Background(), capabilities.ToolInvokeRequest{
		ToolName: "message_context",
		Input:    json.RawMessage(`{}`),
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", RequesterPersonID: "person-1"},
	})
	if errorValue != nil {
		testContext.Fatalf("context failed: %v", errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeFailed || !strings.Contains(string(response.Result), `"errorCode":"conversation_not_available"`) {
		testContext.Fatalf("a call from outside a conversation is told there is none, answered %q", response.Content)
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
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", RequesterPlatformUserID: "requester-pubkey"},
	})
	if errorValue != nil {
		testContext.Fatalf("resolve failed: %v", errorValue)
	}
	var target capabilities.ApprovalTarget
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
		Context:  capabilities.ToolInvokeContext{Platform: "buzz", RequesterPlatformUserID: "requester-pubkey"},
	})
	if errorValue != nil {
		testContext.Fatalf("resolve failed: %v", errorValue)
	}
	if response.Status != "no_target" {
		testContext.Fatalf("an unreachable lookup must resolve to no target, got %q", response.Status)
	}
}

func TestASearchCarriesItsCursorAndAnswersWithTheNextOne(t *testing.T) {
	chatd := answeringPerCall(func(request *http.Request) (int, string) {
		if strings.HasSuffix(request.URL.Path, "/message.search") {
			return http.StatusOK, `{"candidates":[{"messageID":"older","channelID":"channel-1","authorPubkeyHex":"` + requesterPubkeyHex + `","text":"앞선 글"}],"nextCursor":"page-three"}`
		}
		return searchingAsTheRequester(request)
	})

	answered, errorValue := channelService(t, chatd).invokePlatformMessageSearch(context.Background(),
		askedThroughTheAPI("message_search", `{"scope":"channel","channelID":"channel-1","cursor":" page-two "}`))

	if errorValue != nil || answered.IsError {
		t.Fatalf("expected the page to be read, got %+v %v", answered, errorValue)
	}
	searches := searchAskedOfChatd(t, chatd)
	if len(searches) != 1 || searches[0].Cursor != "page-two" {
		t.Fatalf("chatd was asked %+v; the page the caller named is the page read", searches)
	}
	if !strings.Contains(string(answered.Result), `"nextCursor":"page-three"`) || !strings.Contains(string(answered.Result), `"hasMore":true`) {
		t.Fatalf("answered %s; a page with more behind it says so and names the way there", answered.Result)
	}
}

func TestASearchWithNothingBehindItSaysSo(t *testing.T) {
	chatd := answeringPerCall(searchingAsTheRequester)

	answered, _ := channelService(t, chatd).invokePlatformMessageSearch(context.Background(),
		askedThroughTheAPI("message_search", `{"scope":"directMessage","personHint":"박예시"}`))

	if strings.Contains(string(answered.Result), `"nextCursor"`) || !strings.Contains(string(answered.Result), `"hasMore":false`) {
		t.Fatalf("answered %s; the last page names no next one", answered.Result)
	}
}

func TestAMessageIsChangedWithTheEyesOfWhoeverAsked(t *testing.T) {
	throughTheAgent := func(toolName string, input string) capabilities.ToolInvokeRequest {
		return inAChannel(channelToolCall(toolName, input))
	}
	changes := map[string]string{
		"message_update": `{"messageID":"said-in-private","text":"고친 글"}`,
		"message_delete": `{"messageIDs":["said-in-private"]}`,
	}
	routes := map[string]string{"message_update": "message.edit", "message_delete": "message_delete"}
	callers := map[bool]func(string, string) capabilities.ToolInvokeRequest{false: throughTheAgent, true: askedThroughTheAPI}
	for toolName, input := range changes {
		for isTheRequestersOwnCall, calling := range callers {
			chatd := answering(`{}`)

			answered, errorValue := channelService(t, chatd).invokePlatformMessageTool(context.Background(), calling(toolName, input))

			if errorValue != nil || answered.IsError {
				t.Fatalf("%s (own call: %v): expected the change to be asked for, got %+v %v", toolName, isTheRequestersOwnCall, answered, errorValue)
			}
			asked := askedOf(chatd, routes[toolName])
			if len(asked) != 1 {
				t.Fatalf("%s (own call: %v): chatd was asked %+v", toolName, isTheRequestersOwnCall, chatd.asked)
			}
			var change chatdMessageChange
			if errorValue := json.Unmarshal([]byte(asked[0].Body), &change); errorValue != nil {
				t.Fatal(errorValue)
			}
			if change.MessageID != "said-in-private" || change.RequesterPubkeyHex != requesterPubkeyHex || change.ReadAsRequester != isTheRequestersOwnCall {
				t.Fatalf("%s (own call: %v): chatd was asked %+v", toolName, isTheRequestersOwnCall, change)
			}
		}
	}
}
