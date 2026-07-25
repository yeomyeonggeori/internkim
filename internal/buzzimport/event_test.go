package buzzimport

import (
	"strings"
	"testing"
	"time"
)

const testAuthorSecret = "1178851e7a6068811817d7d1a9f3bb126279e84ad078e832908f785cd39c5aa8"

func testMessage() ImportedMessage {
	return ImportedMessage{
		ChannelID:       "4b24ca45-2860-42f4-bdca-4380f803d2aa",
		AuthorSecretHex: testAuthorSecret,
		Text:            "지난 분기 회고 정리했습니다",
		SentAt:          time.Date(2026, time.March, 14, 9, 20, 0, 0, time.UTC),
	}
}

func TestBuiltEventKeepsTheOriginalSendTimeAndVerifies(t *testing.T) {
	event, errorValue := BuildStreamEvent(testMessage())
	if errorValue != nil {
		t.Fatalf("build event: %v", errorValue)
	}
	if event.CreatedAt.Time().UTC() != testMessage().SentAt {
		t.Fatalf("expected the original send time, got %s", event.CreatedAt.Time().UTC())
	}
	if event.Kind != StreamMessageKind {
		t.Fatalf("expected a stream message, got kind %d", event.Kind)
	}
	verified, errorValue := event.CheckSignature()
	if errorValue != nil || !verified {
		t.Fatalf("imported event must verify as its author: %v", errorValue)
	}
}

func TestBuiltEventCarriesChannelAndThreadTags(t *testing.T) {
	message := testMessage()
	message.RootEventID = strings.Repeat("a", 64)
	message.ParentEventID = strings.Repeat("b", 64)
	message.MentionPubkeys = []string{strings.Repeat("c", 64), "too-short"}

	event, errorValue := BuildStreamEvent(message)
	if errorValue != nil {
		t.Fatalf("build event: %v", errorValue)
	}
	if channel := event.Tags.GetFirst([]string{"h"}); channel == nil || channel.Value() != message.ChannelID {
		t.Fatalf("expected the channel tag, got %v", event.Tags)
	}
	roots := 0
	replies := 0
	mentions := 0
	for _, tag := range event.Tags {
		switch {
		case tag[0] == "e" && len(tag) > 3 && tag[3] == "root":
			roots++
		case tag[0] == "e" && len(tag) > 3 && tag[3] == "reply":
			replies++
		case tag[0] == "p":
			mentions++
		}
	}
	if roots != 1 || replies != 1 {
		t.Fatalf("expected one root and one reply marker, got %d and %d", roots, replies)
	}
	if mentions != 1 {
		t.Fatalf("expected malformed mentions to be dropped, got %d", mentions)
	}
}

func TestBuiltEventOmitsAReplyMarkerEqualToItsRoot(t *testing.T) {
	message := testMessage()
	message.RootEventID = strings.Repeat("a", 64)
	message.ParentEventID = strings.Repeat("a", 64)

	event, errorValue := BuildStreamEvent(message)
	if errorValue != nil {
		t.Fatalf("build event: %v", errorValue)
	}
	for _, tag := range event.Tags {
		if tag[0] == "e" && len(tag) > 3 && tag[3] == "reply" {
			t.Fatal("a direct reply to the root must not repeat itself as a reply marker")
		}
	}
}

func TestBuiltEventRejectsIncompleteInput(t *testing.T) {
	for name, mutate := range map[string]func(*ImportedMessage){
		"no channel": func(message *ImportedMessage) { message.ChannelID = "" },
		"no author":  func(message *ImportedMessage) { message.AuthorSecretHex = "" },
		"no text":    func(message *ImportedMessage) { message.Text = "  " },
		"no time":    func(message *ImportedMessage) { message.SentAt = time.Time{} },
	} {
		message := testMessage()
		mutate(&message)
		if _, errorValue := BuildStreamEvent(message); errorValue == nil {
			t.Fatalf("expected %s to be rejected", name)
		}
	}
}
