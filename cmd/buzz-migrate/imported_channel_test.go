package main

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/buzzimport"
	"gitlab.com/eastriver/internkim/internal/buzzimport/mattermostrest"
)

// A re-import put 295 direct conversations into the relay marked open, which is
// every private exchange in the company readable by anyone the relay lets in.
func TestAConversationIsImportedClosed(t *testing.T) {
	for _, channelType := range []string{buzzimport.DirectChannelType, buzzimport.GroupChannelType} {
		visibility := importedChannelVisibility(buzzimport.MattermostChannel{Type: channelType})
		if visibility != "private" {
			t.Errorf("a %q channel imports as %q", channelType, visibility)
		}
	}
	if visibility := importedChannelVisibility(buzzimport.MattermostChannel{Type: buzzimport.OpenChannelType}); visibility != "open" {
		t.Errorf("an open channel imports as %q", visibility)
	}
}

// A closed room refuses a post from someone who is no longer in it, and an
// import is mostly such posts.
func TestEveryoneWhoEverWroteIsAMemberForTheImport(t *testing.T) {
	posts := []buzzimport.MattermostPost{
		{UserID: "still-here"},
		{UserID: "left-last-year"},
		{UserID: "left-last-year"},
		{UserID: ""},
	}

	everyone := memberUserIDsWithPastAuthors([]string{"still-here", "never-wrote"}, posts)

	if len(everyone) != 3 {
		t.Fatalf("members = %v", everyone)
	}
	seen := map[string]bool{}
	for _, userID := range everyone {
		seen[userID] = true
	}
	for _, expected := range []string{"still-here", "never-wrote", "left-last-year"} {
		if !seen[expected] {
			t.Errorf("%s is not a member, so their history would be refused", expected)
		}
	}
}

// The relay refuses kind:9007 without a name — "invalid: channel name is
// required" — so a conversation carries the participants as its stored name and
// each client renames it after the other side when it reads one.
func TestEveryImportedChannelCarriesAName(t *testing.T) {
	authors := map[string]mattermostrest.MattermostAuthor{
		"user-1": {DisplayName: "이샘플"},
		"user-2": {DisplayName: "박예시"},
	}
	conversation := buzzimport.MattermostChannel{Type: buzzimport.DirectChannelType, Name: "user-1__user-2"}

	if name := channelDisplayName(conversation, []string{"user-1", "user-2"}, authors); name == "" {
		t.Error("a conversation imported with no name is refused by the relay")
	}
}

// The relay finds a conversation by the people in it, and the hash it uses must
// match byte for byte or the import cannot learn the id of the room it opened.
func TestTheParticipantHashIgnoresOrderAndRepeats(t *testing.T) {
	first := "aa" + strings.Repeat("11", 31)
	second := "bb" + strings.Repeat("22", 31)

	oneWay, errorValue := participantHash([]string{first, second})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	otherWay, errorValue := participantHash([]string{second, first, first})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !bytes.Equal(oneWay, otherWay) {
		t.Error("the same people hashed to two rooms")
	}
	if len(oneWay) != sha256.Size {
		t.Errorf("hash is %d bytes", len(oneWay))
	}
}

func TestAGroupTooBigForTheRelayIsImportedAsAChannel(t *testing.T) {
	creatorSecret := strings.Repeat("1", 64)
	pubkeys := map[string]string{}
	crowd := []string{}
	for index := 0; index < 9; index++ {
		userID := "user-" + string(rune('a'+index))
		crowd = append(crowd, userID)
		pubkeys[userID] = string(rune('a'+index)) + strings.Repeat("0", 63)
	}
	group := buzzimport.MattermostChannel{Type: buzzimport.GroupChannelType, Name: "crowd"}

	_, isConversation := conversationParticipants(group, crowd, pubkeys, creatorSecret)

	if isConversation {
		t.Error("a group of nine others was sent to a verb that takes eight")
	}
}
