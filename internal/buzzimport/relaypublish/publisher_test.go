package relaypublish

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	nostr "github.com/nbd-wtf/go-nostr"
)

const testSecretHex = "0000000000000000000000000000000000000000000000000000000000000001"

type heardByTheRelay struct {
	host     string
	relayTag string
	kinds    []int
}

func relayThatRefuses(t *testing.T, refusal string, heard *heardByTheRelay) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		heard.host = request.Host
		connection, errorValue := websocket.Accept(writer, request, nil)
		if errorValue != nil {
			return
		}
		defer connection.CloseNow()
		ctx := request.Context()
		_ = connection.Write(ctx, websocket.MessageText, []byte(`["AUTH","challenge-1"]`))
		for {
			_, message, errorValue := connection.Read(ctx)
			if errorValue != nil {
				return
			}
			answerEnvelope(ctx, connection, nostr.ParseMessage(string(message)), refusal, heard)
		}
	}))
}

func answerEnvelope(ctx context.Context, connection *websocket.Conn, envelope nostr.Envelope, refusal string, heard *heardByTheRelay) {
	var event nostr.Event
	switch received := envelope.(type) {
	case *nostr.AuthEnvelope:
		event = received.Event
		heard.relayTag = event.Tags.GetFirst([]string{"relay", ""}).Value()
	case *nostr.EventEnvelope:
		event = received.Event
	default:
		return
	}
	heard.kinds = append(heard.kinds, event.Kind)
	answer := nostr.OKEnvelope{EventID: event.ID, OK: true}
	if event.Kind != nostr.KindClientAuthentication && refusal != "" {
		answer = nostr.OKEnvelope{EventID: event.ID, OK: false, Reason: refusal}
	}
	message, _ := answer.MarshalJSON()
	_ = connection.Write(ctx, websocket.MessageText, message)
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestARelayReachedOnLoopbackHearsAndIsSignedItsPublicName(t *testing.T) {
	heard := &heardByTheRelay{}
	relay := relayThatRefuses(t, "", heard)
	defer relay.Close()
	ctx := testContext(t)

	publisher, errorValue := Connect(ctx, "wss://acme.example.test", strings.Replace(relay.URL, "http://", "ws://", 1), testSecretHex)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer publisher.Close()
	if errorValue := publisher.SetProfile(ctx, testSecretHex, "Sample"); errorValue != nil {
		t.Fatal(errorValue)
	}

	if heard.host != "acme.example.test" || heard.relayTag != "wss://acme.example.test" {
		t.Fatalf("the relay heard Host %q and AUTH relay tag %q, want the public name for both", heard.host, heard.relayTag)
	}
	if len(heard.kinds) != 2 || heard.kinds[1] != ProfileKind {
		t.Fatalf("the relay received kinds %v, want the sign-in and then the profile", heard.kinds)
	}
}

func TestARelayDialledByItsOwnNameNeedsNoSecondAddress(t *testing.T) {
	heard := &heardByTheRelay{}
	relay := relayThatRefuses(t, "", heard)
	defer relay.Close()
	address := strings.Replace(relay.URL, "http://", "ws://", 1)

	publisher, errorValue := Connect(testContext(t), address, "", testSecretHex)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	publisher.Close()

	if heard.relayTag != address || heard.host != strings.TrimPrefix(address, "ws://") {
		t.Fatalf("the relay heard Host %q and AUTH relay tag %q, want %q for both", heard.host, heard.relayTag, address)
	}
}

func TestARefusedEventCarriesTheRelaysReason(t *testing.T) {
	relay := relayThatRefuses(t, "rate-limited: slow down", &heardByTheRelay{})
	defer relay.Close()
	ctx := testContext(t)

	publisher, errorValue := Connect(ctx, "wss://acme.example.test", strings.Replace(relay.URL, "http://", "ws://", 1), testSecretHex)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer publisher.Close()

	refused := publisher.publish(ctx, nostr.Event{Kind: ProfileKind, CreatedAt: nostr.Now()})
	if refused == nil || !strings.Contains(refused.Error(), "rate-limited") {
		t.Fatalf("publish answered %v, want the relay's rate-limited reason", refused)
	}
}
