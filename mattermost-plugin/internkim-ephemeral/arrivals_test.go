package main

import (
	"strings"
	"testing"

	"github.com/mattermost/mattermost/server/public/model"
)

func TestArrivalFromPostCarriesWhoSpokeAndWhoIsThere(t *testing.T) {
	post := &model.Post{Id: "post-7", ChannelId: "channel-1", UserId: "U-author", Message: "오늘 회의 30분 미뤄도 될까요"}

	arrival := arrivalFromPost(post, "이샘플", []string{"U-author", "U-first"})

	if arrival.MessageID != "post-7" || arrival.AuthorExternalID != "U-author" {
		t.Fatalf("arrival = %+v", arrival)
	}
	if arrival.AuthorName != "이샘플" {
		t.Fatalf("the relay should not have to look up who spoke: %q", arrival.AuthorName)
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


func TestDisplayNameOfPrefersTheNameAPersonChose(t *testing.T) {
	full := &model.User{Username: "sample", FirstName: "이", LastName: "샘플"}
	if displayNameOf(full) != "이 샘플" {
		t.Fatalf("name = %q", displayNameOf(full))
	}
}

func TestDisplayNameOfFallsBackToTheUsername(t *testing.T) {
	bare := &model.User{Username: "sample"}
	if displayNameOf(bare) != "sample" {
		t.Fatalf("name = %q", displayNameOf(bare))
	}
	if displayNameOf(nil) != "" {
		t.Fatal("nobody has no name")
	}
}
