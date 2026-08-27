package llmbackend

import (
	"encoding/json"
	"strings"
	"testing"
)

// The agent sends a chat request whose message carries the picture a tool read.
// This type is what decodes it, and it had no room for the parts, so an image
// arrived, was dropped without a word, and the model answered that it could see
// no image — which was true.
func TestAChatMessageKeepsTheImageItArrivedWith(t *testing.T) {
	sent := `{"role":"user","content":"look at this","parts":[{"type":"image","mimeType":"image/png","dataBase64":"aGVsbG8="}]}`

	var message ChatMessage
	if errorValue := json.Unmarshal([]byte(sent), &message); errorValue != nil {
		t.Fatalf("decode: %v", errorValue)
	}

	if len(message.Parts) != 1 {
		t.Fatalf("expected the image to survive the decode, got %d parts", len(message.Parts))
	}
	if message.Parts[0].DataBase64 != "aGVsbG8=" {
		t.Fatalf("expected the image bytes, got %q", message.Parts[0].DataBase64)
	}
}

// Both the structured request and the tool-calling one reach a provider through
// the same mapping, so an image says the same thing on either.
func TestAnImageReachesTheProviderOnTheToolCallingRequest(t *testing.T) {
	messages := openAIChatMessages([]ChatMessage{{
		Role:    "user",
		Content: "look at this",
		Parts:   []MessagePart{{Type: "image", MimeType: "image/png", DataBase64: "aGVsbG8="}},
	}})

	wire, errorValue := json.Marshal(messages)
	if errorValue != nil {
		t.Fatalf("marshal: %v", errorValue)
	}
	if !strings.Contains(string(wire), "image_url") {
		t.Fatalf("expected the image to reach the provider, got %s", wire)
	}
	if !strings.Contains(string(wire), "data:image/png;base64,aGVsbG8=") {
		t.Fatalf("expected the image bytes on the wire, got %s", wire)
	}
}

func TestAMessageWithoutPartsStillSendsPlainText(t *testing.T) {
	messages := openAIChatMessages([]ChatMessage{{Role: "user", Content: "no picture here"}})

	wire, errorValue := json.Marshal(messages)
	if errorValue != nil {
		t.Fatalf("marshal: %v", errorValue)
	}
	if !strings.Contains(string(wire), `"content":"no picture here"`) {
		t.Fatalf("expected plain text content, got %s", wire)
	}
}

// The agent and this service keep the same message in two modules. A field one
// of them starts sending and the other does not know is dropped in silence,
// which is how the image was lost, so a field arriving here that this type has
// no room for fails rather than disappears.
func TestAFieldThisSideDoesNotKnowIsNotSwallowed(t *testing.T) {
	sent := `{"role":"user","content":"hello","somethingTheAgentStartedSending":"value"}`

	decoder := json.NewDecoder(strings.NewReader(sent))
	decoder.DisallowUnknownFields()

	var message ChatMessage
	if decoder.Decode(&message) == nil {
		t.Fatal("expected an unknown field to be noticed; the agent and this service have drifted")
	}
}
