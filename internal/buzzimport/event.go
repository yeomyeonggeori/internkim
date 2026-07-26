package buzzimport

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"
)

const StreamMessageKind = 9
const ProfileKind = 0

type ImportedMessage struct {
	ChannelID       string
	AuthorSecretHex string
	Text            string
	SentAt          time.Time
	RootEventID     string
	ParentEventID   string
	MentionPubkeys  []string
	MediaTags       [][]string
}

func BuildStreamEvent(message ImportedMessage) (nostr.Event, error) {
	if strings.TrimSpace(message.ChannelID) == "" {
		return nostr.Event{}, errors.New("channel id is required")
	}
	if strings.TrimSpace(message.AuthorSecretHex) == "" {
		return nostr.Event{}, errors.New("author secret is required")
	}
	if strings.TrimSpace(message.Text) == "" && len(message.MediaTags) == 0 {
		return nostr.Event{}, errors.New("message needs text or media")
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

func BuildProfileEvent(authorSecretHex string, displayName string, pictureURL string) (nostr.Event, error) {
	if strings.TrimSpace(authorSecretHex) == "" {
		return nostr.Event{}, errors.New("author secret is required")
	}
	profile := map[string]string{"display_name": displayName, "name": displayName}
	if strings.TrimSpace(pictureURL) != "" {
		profile["picture"] = pictureURL
	}
	content, errorValue := json.Marshal(profile)
	if errorValue != nil {
		return nostr.Event{}, errorValue
	}
	event := nostr.Event{
		CreatedAt: nostr.Timestamp(time.Now().UTC().Unix()),
		Kind:      ProfileKind,
		Tags:      nostr.Tags{},
		Content:   string(content),
	}
	if errorValue := event.Sign(authorSecretHex); errorValue != nil {
		return nostr.Event{}, errorValue
	}
	return event, nil
}

func streamEventTags(message ImportedMessage) nostr.Tags {
	tags := nostr.Tags{nostr.Tag{"h", message.ChannelID}}
	rootEventID := strings.TrimSpace(message.RootEventID)
	parentEventID := strings.TrimSpace(message.ParentEventID)
	if parentEventID == "" {
		parentEventID = rootEventID
	}
	if rootEventID != "" {
		if parentEventID == rootEventID {
			tags = append(tags, nostr.Tag{"e", rootEventID, "", "reply"})
		} else {
			tags = append(tags, nostr.Tag{"e", rootEventID, "", "root"})
			tags = append(tags, nostr.Tag{"e", parentEventID, "", "reply"})
		}
	}
	for _, mentionPubkey := range message.MentionPubkeys {
		mentionPubkey = strings.ToLower(strings.TrimSpace(mentionPubkey))
		if len(mentionPubkey) == 64 {
			tags = append(tags, nostr.Tag{"p", mentionPubkey})
		}
	}
	for _, mediaTag := range message.MediaTags {
		if len(mediaTag) > 0 {
			tags = append(tags, nostr.Tag(mediaTag))
		}
	}
	return tags
}

const ReactionKind = 7

type ImportedReaction struct {
	ChannelID         string
	AuthorSecretHex   string
	TargetEventID     string
	Emoji             string
	CustomShortcode   string
	CustomEmojiURL    string
	CreatedAt         time.Time
}

func BuildReactionEvent(reaction ImportedReaction) (nostr.Event, error) {
	if strings.TrimSpace(reaction.TargetEventID) == "" {
		return nostr.Event{}, errors.New("reaction needs a target")
	}
	if strings.TrimSpace(reaction.AuthorSecretHex) == "" {
		return nostr.Event{}, errors.New("reaction author secret is required")
	}
	content := reaction.Emoji
	tags := nostr.Tags{nostr.Tag{"e", reaction.TargetEventID}}
	if reaction.CustomEmojiURL != "" && reaction.CustomShortcode != "" {
		content = ":" + reaction.CustomShortcode + ":"
		tags = append(tags, nostr.Tag{"emoji", reaction.CustomShortcode, reaction.CustomEmojiURL})
	} else if strings.TrimSpace(content) == "" {
		return nostr.Event{}, errors.New("reaction needs an emoji")
	}
	if strings.TrimSpace(reaction.ChannelID) != "" {
		tags = append(tags, nostr.Tag{"h", reaction.ChannelID})
	}
	createdAt := reaction.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	event := nostr.Event{
		CreatedAt: nostr.Timestamp(createdAt.UTC().Unix()),
		Kind:      ReactionKind,
		Tags:      tags,
		Content:   content,
	}
	if errorValue := event.Sign(reaction.AuthorSecretHex); errorValue != nil {
		return nostr.Event{}, errorValue
	}
	return event, nil
}
