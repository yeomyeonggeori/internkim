package llmbackend

import (
	"encoding/json"
	"os"
	"testing"
)

const blueprotocolEmbeddingPromptCasesPath = "../../.dependency/blueclaw/.dependency/blueprotocol/model/embeddingprompt/testdata/embeddinggemma_prompts.json"

type embeddingPromptCase struct {
	Model     string `json:"model"`
	InputType string `json:"inputType"`
	Text      string `json:"text"`
	Expected  string `json:"expected"`
}

func TestEmbeddingPromptsMatchTheOnesBlueprotocolSendsOnTheDirectEndpoint(t *testing.T) {
	document, errorValue := os.ReadFile(blueprotocolEmbeddingPromptCasesPath)
	if errorValue != nil {
		t.Fatalf("read the cases blueprotocol holds its EmbeddingGemma prompts to: %v", errorValue)
	}
	var cases []embeddingPromptCase
	if errorValue := json.Unmarshal(document, &cases); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(cases) == 0 {
		t.Fatal("blueprotocol's prompt cases are empty")
	}
	for _, promptCase := range cases {
		prepared := prepareEmbeddingInputs([]string{promptCase.Text}, EmbeddingRequest{InputType: promptCase.InputType}, promptCase.Model)
		if prepared[0] != promptCase.Expected {
			t.Errorf("%s %q %q: blueprotocol sends %q and capabilityd %q", promptCase.Model, promptCase.InputType, promptCase.Text, promptCase.Expected, prepared[0])
		}
	}
}
