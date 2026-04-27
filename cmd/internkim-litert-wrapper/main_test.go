package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderPromptStringifiesSchemaDocument(t *testing.T) {
	prompt := renderPrompt(requestDocument{
		Messages: []message{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: schemaRequest{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})

	if !strings.Contains(prompt, `"required":["reply"]`) {
		t.Fatalf("expected prompt to include schema JSON, got %q", prompt)
	}
}

func TestValidateMinimumStructuredOutput(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","required":["reply"]}`)

	if !validateMinimumStructuredOutput(`{"reply":"ok"}`, schema) {
		t.Fatal("expected required field to pass")
	}
	if validateMinimumStructuredOutput(`{"message":"wrong"}`, schema) {
		t.Fatal("expected missing required field to fail")
	}
}
