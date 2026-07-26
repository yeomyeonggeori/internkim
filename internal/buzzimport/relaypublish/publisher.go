package relaypublish

import (
	"context"
	"strings"
	"time"

	nostr "github.com/nbd-wtf/go-nostr"
)

const (
	CreateChannelKind = 9007
	AddMemberKind     = 9000
	ProfileKind       = 0
)

type Publisher struct {
	relay *nostr.Relay
}

func Connect(ctx context.Context, relayURL string, actorSecretHex string) (*Publisher, error) {
	relay, errorValue := nostr.RelayConnect(ctx, relayURL)
	if errorValue != nil {
		return nil, errorValue
	}
	// The relay sends its NIP-42 challenge right after the socket opens; give the
	// read loop a moment to record it before signing the auth event against it.
	time.Sleep(700 * time.Millisecond)
	if errorValue := relay.Auth(ctx, func(event *nostr.Event) error {
		return event.Sign(actorSecretHex)
	}); errorValue != nil {
		relay.Close()
		return nil, errorValue
	}
	return &Publisher{relay: relay}, nil
}

func (publisher *Publisher) Close() {
	publisher.relay.Close()
}

func (publisher *Publisher) CreateChannel(ctx context.Context, actorSecretHex, channelID, name, purpose string) error {
	tags := nostr.Tags{
		nostr.Tag{"h", channelID},
		nostr.Tag{"name", name},
		nostr.Tag{"visibility", "open"},
		nostr.Tag{"channel_type", "stream"},
	}
	if strings.TrimSpace(purpose) != "" {
		tags = append(tags, nostr.Tag{"about", purpose})
	}
	return publisher.signAndPublish(ctx, actorSecretHex, CreateChannelKind, "", tags)
}

func (publisher *Publisher) AddMember(ctx context.Context, actorSecretHex, channelID, memberPubkeyHex string) error {
	return publisher.signAndPublish(ctx, actorSecretHex, AddMemberKind, "", nostr.Tags{
		nostr.Tag{"h", channelID},
		nostr.Tag{"p", strings.ToLower(memberPubkeyHex)},
	})
}

func (publisher *Publisher) SetProfile(ctx context.Context, actorSecretHex, displayName string) error {
	content := `{"display_name":` + jsonString(displayName) + `,"name":` + jsonString(displayName) + `}`
	return publisher.signAndPublish(ctx, actorSecretHex, ProfileKind, content, nostr.Tags{})
}

func (publisher *Publisher) signAndPublish(ctx context.Context, actorSecretHex string, kind int, content string, tags nostr.Tags) error {
	event := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      kind,
		Tags:      tags,
		Content:   content,
	}
	if errorValue := event.Sign(actorSecretHex); errorValue != nil {
		return errorValue
	}
	var lastError error
	for attempt := 0; attempt < 5; attempt++ {
		lastError = publisher.relay.Publish(ctx, event)
		if lastError == nil {
			return nil
		}
		if !strings.Contains(lastError.Error(), "rate-limited") {
			return lastError
		}
		time.Sleep(5500 * time.Millisecond)
	}
	return lastError
}

func jsonString(value string) string {
	var builder strings.Builder
	builder.WriteByte('"')
	for _, character := range value {
		switch character {
		case '"', '\\':
			builder.WriteByte('\\')
			builder.WriteRune(character)
		case '\n':
			builder.WriteString("\\n")
		default:
			builder.WriteRune(character)
		}
	}
	builder.WriteByte('"')
	return builder.String()
}
