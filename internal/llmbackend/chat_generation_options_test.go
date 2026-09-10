package llmbackend

import "testing"

func TestAChatRequestCarriesTheGenerationOptionsItWasGiven(t *testing.T) {
	seed := int64(7)
	temperature := 0.3
	maxTokens := 900
	request := openAIChatCompletionRequest("a-model", ChatRequest{
		Messages:          []ChatMessage{{Role: "user", Content: "안녕"}},
		GenerationOptions: &GenerationOptions{Seed: &seed, Temperature: &temperature, MaxTokens: &maxTokens},
	})
	if request.Seed == nil || *request.Seed != seed || request.Temperature == nil || *request.Temperature != temperature || request.MaxTokens == nil || *request.MaxTokens != maxTokens {
		t.Fatalf("the options the agent sent must reach the endpoint, got seed=%v temperature=%v max_tokens=%v", request.Seed, request.Temperature, request.MaxTokens)
	}
}

func TestAChatRequestWithoutOptionsAsksForNone(t *testing.T) {
	request := openAIChatCompletionRequest("a-model", ChatRequest{Messages: []ChatMessage{{Role: "user", Content: "안녕"}}})
	if request.Seed != nil || request.Temperature != nil || request.MaxTokens != nil {
		t.Fatalf("a request given no options must not invent any, got %+v", request)
	}
}
