package relaypublish

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coder/websocket"
	nostr "github.com/nbd-wtf/go-nostr"
)

const (
	CreateChannelKind = 9007
	AddMemberKind     = 9000
	ProfileKind       = 0
	OpenDirectKind    = 41010
)

const (
	answerWait        = 15 * time.Second
	largestFrameBytes = 1 << 24
)

type Publisher struct {
	connection *websocket.Conn
}

func Connect(ctx context.Context, relayURL, dialURL, actorSecretHex string) (*Publisher, error) {
	options, errorValue := dialOptionsFor(relayURL, dialURL)
	if errorValue != nil {
		return nil, errorValue
	}
	connection, _, errorValue := websocket.Dial(ctx, addressToDial(relayURL, dialURL), options)
	if errorValue != nil {
		return nil, fmt.Errorf("could not open the relay at %s: %w", addressToDial(relayURL, dialURL), errorValue)
	}
	connection.SetReadLimit(largestFrameBytes)
	publisher := &Publisher{connection: connection}
	if errorValue := publisher.authenticate(ctx, nostr.NormalizeURL(relayURL), actorSecretHex); errorValue != nil {
		connection.CloseNow()
		return nil, errorValue
	}
	return publisher, nil
}

func addressToDial(relayURL, dialURL string) string {
	if strings.TrimSpace(dialURL) == "" {
		return relayURL
	}
	return dialURL
}

func dialOptionsFor(relayURL, dialURL string) (*websocket.DialOptions, error) {
	if strings.TrimSpace(dialURL) == "" {
		return &websocket.DialOptions{}, nil
	}
	named, errorValue := url.Parse(relayURL)
	if errorValue != nil || named.Host == "" {
		return nil, fmt.Errorf("the relay is named by %q, which is not an address", relayURL)
	}
	return &websocket.DialOptions{Host: named.Host}, nil
}

func (publisher *Publisher) Close() {
	publisher.connection.Close(websocket.StatusNormalClosure, "")
}

func (publisher *Publisher) authenticate(ctx context.Context, relayURL, actorSecretHex string) error {
	challenge, errorValue := publisher.awaitChallenge(ctx)
	if errorValue != nil {
		return errorValue
	}
	event := nostr.Event{
		CreatedAt: nostr.Now(),
		Kind:      nostr.KindClientAuthentication,
		Tags:      nostr.Tags{nostr.Tag{"relay", relayURL}, nostr.Tag{"challenge", challenge}},
	}
	if errorValue := event.Sign(actorSecretHex); errorValue != nil {
		return errorValue
	}
	if errorValue := publisher.send(ctx, &nostr.AuthEnvelope{Event: event}); errorValue != nil {
		return errorValue
	}
	return publisher.awaitAcceptance(ctx, event.ID)
}

func (publisher *Publisher) awaitChallenge(ctx context.Context) (string, error) {
	waiting, cancel := context.WithTimeout(ctx, answerWait)
	defer cancel()
	for {
		envelope, errorValue := publisher.read(waiting)
		if errorValue != nil {
			return "", fmt.Errorf("the relay sent no sign-in challenge: %w", errorValue)
		}
		if auth, isAuth := envelope.(*nostr.AuthEnvelope); isAuth && auth.Challenge != nil {
			return *auth.Challenge, nil
		}
	}
}

func (publisher *Publisher) awaitAcceptance(ctx context.Context, eventID string) error {
	waiting, cancel := context.WithTimeout(ctx, answerWait)
	defer cancel()
	for {
		envelope, errorValue := publisher.read(waiting)
		if errorValue != nil {
			return fmt.Errorf("the relay did not answer event %s: %w", eventID, errorValue)
		}
		answer, isAnswer := envelope.(*nostr.OKEnvelope)
		if !isAnswer || answer.EventID != eventID {
			continue
		}
		if !answer.OK {
			return fmt.Errorf("the relay refused event %s: %s", eventID, answer.Reason)
		}
		return nil
	}
}

func (publisher *Publisher) read(ctx context.Context) (nostr.Envelope, error) {
	_, message, errorValue := publisher.connection.Read(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	return nostr.ParseMessage(string(message)), nil
}

func (publisher *Publisher) send(ctx context.Context, envelope nostr.Envelope) error {
	message, errorValue := envelope.MarshalJSON()
	if errorValue != nil {
		return errorValue
	}
	return publisher.connection.Write(ctx, websocket.MessageText, message)
}

func (publisher *Publisher) CreateChannel(ctx context.Context, actorSecretHex, channelID, name, purpose, channelType, visibility string) error {
	tags := nostr.Tags{
		nostr.Tag{"h", channelID},
		nostr.Tag{"name", name},
		nostr.Tag{"visibility", visibility},
		nostr.Tag{"channel_type", channelType},
	}
	if strings.TrimSpace(purpose) != "" {
		tags = append(tags, nostr.Tag{"about", purpose})
	}
	return publisher.signAndPublish(ctx, actorSecretHex, CreateChannelKind, "", tags)
}

// The relay makes a direct conversation itself: it types the room dm, closes it,
// and names it "DM" rather than after the people in it, which is the only name
// that is right for everyone reading. Creating one as an ordinary channel is
// what put the reader's own name in the label and left the room open.
func (publisher *Publisher) OpenDirectMessage(ctx context.Context, actorSecretHex string, counterpartPubkeyHexes []string) error {
	tags := nostr.Tags{}
	for _, pubkeyHex := range counterpartPubkeyHexes {
		tags = append(tags, nostr.Tag{"p", strings.ToLower(strings.TrimSpace(pubkeyHex))})
	}
	return publisher.signAndPublish(ctx, actorSecretHex, OpenDirectKind, "", tags)
}

// An empty role asks for no role at all, which the relay reads as "leave the
// role this person already holds" and settles as member for someone new. Naming
// one is how an add also promotes.
func (publisher *Publisher) AddMember(ctx context.Context, actorSecretHex, channelID, memberPubkeyHex, role string) error {
	return publisher.signAndPublish(ctx, actorSecretHex, AddMemberKind, "", addMemberTags(channelID, memberPubkeyHex, role))
}

func addMemberTags(channelID, memberPubkeyHex, role string) nostr.Tags {
	tags := nostr.Tags{
		nostr.Tag{"h", channelID},
		nostr.Tag{"p", strings.ToLower(memberPubkeyHex)},
	}
	if trimmed := strings.TrimSpace(role); trimmed != "" {
		tags = append(tags, nostr.Tag{"role", trimmed})
	}
	return tags
}

func (publisher *Publisher) SetProfile(ctx context.Context, actorSecretHex, displayName, pictureURL string) error {
	content := `{"display_name":` + jsonString(displayName) + `,"name":` + jsonString(displayName)
	if pictureURL != "" {
		content += `,"picture":` + jsonString(pictureURL)
	}
	return publisher.signAndPublish(ctx, actorSecretHex, ProfileKind, content+`}`, nostr.Tags{})
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
		lastError = publisher.publish(ctx, event)
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

func (publisher *Publisher) publish(ctx context.Context, event nostr.Event) error {
	if errorValue := publisher.send(ctx, &nostr.EventEnvelope{Event: event}); errorValue != nil {
		return errorValue
	}
	return publisher.awaitAcceptance(ctx, event.ID)
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
