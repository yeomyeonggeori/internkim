package buzzimport

import (
	"errors"
	"strings"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"
)

const StreamMessageKind = 9

type ImportedMessage struct {
	ChannelID       string
	AuthorSecretHex string
	Text            string
	SentAt          time.Time
	RootEventID     string
	ParentEventID   string
	MentionPubkeys  []string
}

func BuildStreamEvent(message ImportedMessage) (nostr.Event, error) {
	if strings.TrimSpace(message.ChannelID) == "" {
		return nostr.Event{}, errors.New("channel id is required")
	}
	if strings.TrimSpace(message.AuthorSecretHex) == "" {
		return nostr.Event{}, errors.New("author secret is required")
	}
	if strings.TrimSpace(message.Text) == "" {
		return nostr.Event{}, errors.New("message text is required")
	}
	if message.SentAt.IsZero() {
		return nostr.Event{}, errors.New("message time is required")
	}
	event := nostr.Event{
		CreatedAt: nostr.Timestamp(message.SentAt.UTC().Unix()),
		Kind:      StreamMessageKind,
		Tags:      streamEventTags(message),
		Content:   message.Text,
	}
	if errorValue := event.Sign(message.AuthorSecretHex); errorValue != nil {
		return nostr.Event{}, errorValue
	}
	return event, nil
}

func streamEventTags(message ImportedMessage) nostr.Tags {
	tags := nostr.Tags{nostr.Tag{"h", message.ChannelID}}
	rootEventID := strings.TrimSpace(message.RootEventID)
	parentEventID := strings.TrimSpace(message.ParentEventID)
	if rootEventID != "" {
		tags = append(tags, nostr.Tag{"e", rootEventID, "", "root"})
	}
	if parentEventID != "" && parentEventID != rootEventID {
		tags = append(tags, nostr.Tag{"e", parentEventID, "", "reply"})
	}
	for _, mentionPubkey := range message.MentionPubkeys {
		mentionPubkey = strings.ToLower(strings.TrimSpace(mentionPubkey))
		if len(mentionPubkey) == 64 {
			tags = append(tags, nostr.Tag{"p", mentionPubkey})
		}
	}
	return tags
}
