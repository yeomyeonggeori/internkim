package capabilityprotocol

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	capabilityschema "gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
)

type refusedCall struct {
	Tool    string         `json:"tool"`
	Input   map[string]any `json:"input"`
	Refusal string         `json:"refusal"`
}

// A call is refused in two places — here, against the descriptor a device holds,
// and in the web app, against the zod the catalog is written in. The model that
// reads the refusal is the same one either way, so the two say it in one
// vocabulary. web/tests/unit/catalog/refusal-sentences.test.ts holds the other
// half to this same file.
func TestRefusalSentencesAreTheOnesTheCatalogPromises(t *testing.T) {
	document, errorValue := os.ReadFile("refusal-sentences.json")
	if errorValue != nil {
		t.Fatalf("the shared refusal sentences are not readable: %v", errorValue)
	}
	var calls []refusedCall
	if json.Unmarshal(document, &calls) != nil {
		t.Fatal("the shared refusal sentences are not readable as JSON")
	}
	if len(calls) == 0 {
		t.Fatal("a conformance file with no cases proves nothing")
	}

	for _, call := range calls {
		descriptor, isPublished := generatedToolDescriptor(call.Tool)
		if !isPublished {
			t.Fatalf("%s: the catalog publishes no such tool", call.Tool)
		}
		input, errorValue := json.Marshal(call.Input)
		if errorValue != nil {
			t.Fatalf("%s: the case's input is not JSON: %v", call.Tool, errorValue)
		}
		refused := capabilityschema.ValidateInput(descriptor.InputSchema, input)
		if refused == nil {
			t.Fatalf("%s: expected %q to be refused", call.Tool, input)
		}
		if !strings.Contains(refused.Error(), call.Refusal) {
			t.Fatalf("%s: expected the refusal to carry %q, got %q", call.Tool, call.Refusal, refused.Error())
		}
	}
}
