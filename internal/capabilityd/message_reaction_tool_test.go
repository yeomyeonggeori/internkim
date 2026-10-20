package capabilityd

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestAReactionIsAskedOfChatdAsTheRequesterInTheMessagesOwnChannel(t *testing.T) {
	chatd := answeringPerCall(answeringChannelsAsTheRequester)

	answered, _ := channelService(t, chatd).invokeMessageReactionTool(context.Background(),
		channelToolCall(messageReactionAddToolName, `{"messageID":"post-1","emoji":"tada"}`))

	if answered.IsError {
		t.Fatalf("expected the reaction to be carried, got %+v", answered)
	}
	reactions := askedOf(chatd, "requester.reaction.add")
	if len(reactions) != 1 {
		t.Fatalf("expected one reaction asked of chatd, got %+v", chatd.asked)
	}
	var body map[string]string
	_ = json.Unmarshal([]byte(reactions[0].Body), &body)
	if body["requesterPubkeyHex"] != requesterPubkeyHex || body["conversationID"] != "channel-1" || body["messageID"] != "post-1" || body["emoji"] != "tada" {
		t.Fatalf("expected the requester, the message's channel and the emoji as named, got %s", reactions[0].Body)
	}
}

func TestAReactionToAMessageNobodyCanReadChangesNothing(t *testing.T) {
	chatd := answeringPerCall(func(request *http.Request) (int, string) {
		if strings.HasSuffix(request.URL.Path, "/message.search") {
			return http.StatusOK, `{"candidates":[]}`
		}
		return answeringChannelsAsTheRequester(request)
	})

	answered, _ := channelService(t, chatd).invokeMessageReactionTool(context.Background(),
		channelToolCall(messageReactionAddToolName, `{"messageID":"post-404","emoji":"tada"}`))

	if !answered.IsError || answered.ErrorCode != "not_found" || len(askedOf(chatd, "requester.reaction.add")) != 0 {
		t.Fatalf("expected an unreadable message to be refused before reacting, got %+v", answered)
	}
}

func TestHidingADirectConversationFindsTheOneWithThatPerson(t *testing.T) {
	chatd := answeringPerCall(answeringChannelsAsTheRequester)

	answered, _ := channelService(t, chatd).invokeConversationVisibilityTool(context.Background(),
		channelToolCall(conversationHideToolName, `{"personHint":"박예시"}`))

	if answered.IsError {
		t.Fatalf("expected the conversation to be hidden, got %+v", answered)
	}
	hidden := askedOf(chatd, "requester.dm.hide")
	if len(hidden) != 1 || !strings.Contains(hidden[0].Body, `"conversationID":"dm-1"`) {
		t.Fatalf("expected dm-1 to be hidden as the requester, got %+v", chatd.asked)
	}
}

func TestHidingAConversationThatDoesNotExistSaysSo(t *testing.T) {
	chatd := answeringPerCall(func(request *http.Request) (int, string) {
		if strings.HasSuffix(request.URL.Path, "/requester.conversations.list") {
			return http.StatusOK, `{"conversations":[{"id":"channel-1","name":"전사-공지","kind":"group"}]}`
		}
		return answeringChannelsAsTheRequester(request)
	})

	answered, _ := channelService(t, chatd).invokeConversationVisibilityTool(context.Background(),
		channelToolCall(conversationHideToolName, `{"personHint":"박예시"}`))

	if !answered.IsError || answered.ErrorCode != "conversation_not_found" {
		t.Fatalf("expected no direct conversation to be named as missing, got %+v", answered)
	}
}

func TestAWholeRewriteSendsTheTextAsGivenWithoutQuotingTheOldOne(t *testing.T) {
	chatd := answeringPerCall(answeringChannelsAsTheRequester)
	request := channelToolCall("message_update", `{"messageID":"post-1","text":"회의는 9월 3일 오후 2시로 바뀌었습니다."}`)
	request.Context.ReplyTargetID = "buzz:channel-1"

	answered, _ := channelService(t, chatd).invokePlatformMessageTool(context.Background(), request)

	if answered.IsError {
		t.Fatalf("expected the rewrite to be carried, got %+v", answered)
	}
	if len(askedOf(chatd, "message.search")) != 0 {
		t.Fatal("a whole rewrite has nothing to quote, so the current text is not read")
	}
	edits := askedOf(chatd, "message.edit")
	if len(edits) != 1 || !strings.Contains(edits[0].Body, "회의는 9월 3일 오후 2시로 바뀌었습니다.") {
		t.Fatalf("expected the whole text to be sent, got %+v", edits)
	}
}

func TestARewriteAndASpanAtOnceAreRefused(t *testing.T) {
	if _, errorValue := decodePlatformMessageUpdateInput(json.RawMessage(`{"messageID":"post-1","text":"새 글","oldText":"옛","newText":"새"}`)); errorValue == nil {
		t.Fatal("expected text with oldText to be refused")
	}
}
