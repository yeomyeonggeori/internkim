package main

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestArrivalFromPostCarriesWhoSpokeAndWhoIsThere(t *testing.T) {
	post := &model.Post{Id: "post-7", ChannelId: "channel-1", UserId: "U-author", Message: "오늘 회의 30분 미뤄도 될까요"}

	arrival := arrivalFromPost(post, []string{"U-author", "U-first"})

	if arrival.MessageID != "post-7" || arrival.AuthorExternalID != "U-author" {
		t.Fatalf("arrival = %+v", arrival)
	}
	if arrival.ConversationID != "channel-1" {
		t.Fatalf("conversation = %q", arrival.ConversationID)
	}
	if arrival.Preview != post.Message {
		t.Fatalf("preview = %q", arrival.Preview)
	}
	if len(arrival.RecipientExternalIDs) != 2 {
		t.Fatalf("the relay decides who to drop, not the plugin: %v", arrival.RecipientExternalIDs)
	}
}

func TestUserIDsOfKeepsEveryoneInTheConversation(t *testing.T) {
	members := model.ChannelMembers{
		model.ChannelMember{ChannelId: "channel-1", UserId: "U-first"},
		model.ChannelMember{ChannelId: "channel-1", UserId: "U-second"},
	}

	if strings.Join(userIDsOf(members), ",") != "U-first,U-second" {
		t.Fatalf("userIDs = %v", userIDsOf(members))
	}
}

func TestTrimPreviewCutsByCharacterNotByByte(t *testing.T) {
	long := strings.Repeat("가", 500)

	trimmed := trimPreview(long)

	if len([]rune(trimmed)) != arrivalPreviewLimit {
		t.Fatalf("trimmed to %d runes", len([]rune(trimmed)))
	}
	if !strings.HasPrefix(long, trimmed) {
		t.Fatal("trimming must not corrupt the text it keeps")
	}
}

func TestTrimPreviewLeavesAShortMessageAlone(t *testing.T) {
	if trimPreview("짧은 메시지") != "짧은 메시지" {
		t.Fatal("a short message is not trimmed")
	}
}
