package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderPromptStringifiesSchemaDocument(t *testing.T) {
	prompt := renderPrompt(requestDocument{
		Mode:     "structured",
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

func TestRenderPromptForTextDoesNotRequestJSON(t *testing.T) {
	prompt := renderPrompt(requestDocument{
		Mode:     "text",
		Messages: []message{{Role: "user", Content: "hello"}},
	})

	if strings.Contains(prompt, "Return exactly one JSON object") {
		t.Fatalf("expected text prompt not to require structured JSON, got %q", prompt)
	}
	if !strings.Contains(prompt, "plain text") {
		t.Fatalf("expected text prompt to ask for plain text, got %q", prompt)
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

func TestExtractTextContentDoesNotRequireJSON(t *testing.T) {
	content, errorValue := extractTextContent("plain local reply\n")
	if errorValue != nil {
		t.Fatalf("expected plain text extraction: %v", errorValue)
	}
	if content != "plain local reply" {
		t.Fatalf("expected trimmed plain text, got %q", content)
	}
}

func TestLiteRTFailureOutputDetectsTraceback(t *testing.T) {
	if !isLiteRTFailureOutput([]byte("An error occurred\nTraceback\nRuntimeError: INTERNAL: ERROR: failed")) {
		t.Fatal("expected LiteRT traceback output to be treated as failure")
	}
	if isLiteRTFailureOutput([]byte("ok")) {
		t.Fatal("expected normal output to pass")
	}
}
