package llmbackend

import (
	"encoding/json"
	"reflect"
	"testing"
)

func routingOf(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var document map[string]any
	if errorValue := json.Unmarshal(body, &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	routing, isPresent := document["provider"].(map[string]any)
	if !isPresent {
		t.Fatalf("the request named no provider routing: %s", body)
	}
	return routing
}

// A request goes out one of four ways depending on how the model is asked, and
// each way used to spell the provider block for itself. They ask for one thing,
// so they say it once: a request that skips the order is served by whoever
// OpenRouter picks, silently and only sometimes.
func TestEveryWayOfAskingCarriesTheSameProviderRouting(t *testing.T) {
	structuredRequest := StructuredRequest{
		Messages:          []Message{{Role: "user", Content: "안녕"}},
		RequireParameters: true,
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "answer",
			Document: json.RawMessage(`{"type":"object"}`),
		},
	}
	textRequest := TextRequest{
		Messages:          []Message{{Role: "user", Content: "안녕"}},
		RequireParameters: true,
	}

	bodies := map[string][]byte{}
	for name, build := range map[string]func() ([]byte, error){
		"structured": func() ([]byte, error) {
			return buildOpenRouterStructuredRequest(structuredRequest, "a-model")
		},
		"prompted": func() ([]byte, error) {
			return buildOpenRouterPromptedStructuredRequest(structuredRequest, "a-model")
		},
		"text": func() ([]byte, error) { return buildOpenRouterTextRequest(textRequest, "a-model") },
	} {
		body, errorValue := build()
		if errorValue != nil {
			t.Fatalf("%s: %v", name, errorValue)
		}
		bodies[name] = body
	}

	wanted := openRouterProviderRouting()
	for name, body := range bodies {
		routing := routingOf(t, body)
		if routing["require_parameters"] != true {
			t.Errorf("%s asked for parameters to be optional: %v", name, routing)
		}
		if routing["allow_fallbacks"] != true {
			t.Errorf("%s would be refused rather than served by somebody else: %v", name, routing)
		}
		order, isList := routing["order"].([]any)
		if !isList {
			t.Fatalf("%s named no provider order: %v", name, routing)
		}
		named := make([]string, 0, len(order))
		for _, provider := range order {
			named = append(named, provider.(string))
		}
		if !reflect.DeepEqual(named, wanted["order"]) {
			t.Errorf("%s asks for %v, the others ask for %v", name, named, wanted["order"])
		}
	}
}

func TestTheProviderOrderIsTheOneTheCompanyAskedFor(t *testing.T) {
	if !reflect.DeepEqual(openRouterPreferredProviders, []string{"modal", "baseten"}) {
		t.Errorf("provider order = %v, want [modal baseten]", openRouterPreferredProviders)
	}
}
