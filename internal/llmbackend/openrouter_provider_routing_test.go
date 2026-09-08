package llmbackend

import (
	"encoding/json"
	"reflect"
	"testing"
)

func documentOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var document map[string]any
	if errorValue := json.Unmarshal(body, &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func routingOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	routing, isPresent := documentOf(t, body)["provider"].(map[string]any)
	if !isPresent {
		t.Fatalf("the request named no provider routing: %s", body)
	}
	return routing
}

func everyWayOfAsking(backend OpenRouterBackend, requireParameters bool, reasoningEffort string) map[string][]byte {
	structuredRequest := StructuredRequest{
		Messages:          []Message{{Role: "user", Content: "안녕"}},
		RequireParameters: requireParameters,
		ReasoningEffort:   reasoningEffort,
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "answer",
			Document: json.RawMessage(`{"type":"object"}`),
		},
	}
	textRequest := TextRequest{
		Messages:          []Message{{Role: "user", Content: "안녕"}},
		RequireParameters: requireParameters,
		ReasoningEffort:   reasoningEffort,
	}
	actionTools := []nativeActionTool{{FunctionName: "answer", Description: "answer", Parameters: json.RawMessage(`{"type":"object"}`)}}
	bodies := map[string][]byte{}
	bodies["structured"], _ = backend.buildStructuredRequest(structuredRequest, "a-model")
	bodies["prompted"], _ = backend.buildPromptedStructuredRequest(structuredRequest, "a-model")
	bodies["text"], _ = backend.buildTextRequest(textRequest, "a-model")
	bodies["action"], _, _ = backend.buildChatActionRequest(structuredRequest, "a-model", actionTools)
	return bodies
}

// A request goes out one of four ways depending on how the model is asked, and
// each way used to spell the provider block for itself. They ask for one thing,
// so they say it once: a request that skips the preference is served by whoever
// OpenRouter picks, silently and only sometimes.
func TestEveryWayOfAskingCarriesTheSameServingPreferences(t *testing.T) {
	backend := OpenRouterBackend{ProviderOrder: []string{"modal", "baseten"}, ProviderSort: "throughput"}
	for name, body := range everyWayOfAsking(backend, true, "low") {
		routing := routingOf(t, body)
		if routing["require_parameters"] != true {
			t.Errorf("%s asked for parameters to be optional: %v", name, routing)
		}
		if routing["allow_fallbacks"] != true {
			t.Errorf("%s would be refused rather than served by somebody else: %v", name, routing)
		}
		if routing["sort"] != "throughput" {
			t.Errorf("%s left the rest of the providers in price order: %v", name, routing)
		}
		if !reflect.DeepEqual(routing["order"], []any{"modal", "baseten"}) {
			t.Errorf("%s did not ask for the preferred providers first: %v", name, routing)
		}
		reasoning, _ := documentOf(t, body)["reasoning"].(map[string]any)
		if reasoning["effort"] != "low" {
			t.Errorf("%s did not carry the reasoning effort it was given: %s", name, body)
		}
	}
}

func TestAPreferenceRidesEvenWhenParametersMayBeDropped(t *testing.T) {
	backend := OpenRouterBackend{ProviderSort: "throughput"}
	for name, body := range everyWayOfAsking(backend, false, "") {
		routing := routingOf(t, body)
		if routing["sort"] != "throughput" {
			t.Errorf("%s was left to price-based routing: %v", name, routing)
		}
		if _, isRequired := routing["require_parameters"]; isRequired {
			t.Errorf("%s demanded parameter support nobody asked for: %v", name, routing)
		}
		if _, isNamed := documentOf(t, body)["reasoning"]; isNamed {
			t.Errorf("%s asked for a reasoning effort nobody gave it: %s", name, body)
		}
	}
}

func TestABackendWithNoPreferenceLeavesRoutingToOpenRouter(t *testing.T) {
	for name, body := range everyWayOfAsking(OpenRouterBackend{}, false, "") {
		if _, isNamed := documentOf(t, body)["provider"]; isNamed {
			t.Errorf("%s named a routing preference it was never given: %s", name, body)
		}
	}
}
