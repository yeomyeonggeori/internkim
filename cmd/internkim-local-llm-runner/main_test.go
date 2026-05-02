package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderPromptDoesNotStringifySchemaDocument(t *testing.T) {
	prompt := renderPrompt(requestDocument{
		Mode:     "structured",
		Messages: []message{{Role: "user", Content: "hello"}},
		ConstrainedDecoding: constraintRequest{
			Type: "json_schema",
			JSONSchema: schemaRequest{
				Name:     "reply",
				Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
			},
		},
	})

	if strings.Contains(prompt, `"required":["reply"]`) {
		t.Fatalf("expected prompt not to include schema JSON, got %q", prompt)
	}
}

func TestRenderPromptOnlyContainsMessages(t *testing.T) {
	prompt := renderPrompt(requestDocument{
		Mode:     "text",
		Messages: []message{{Role: "user", Content: "hello"}},
	})

	if prompt != "user: hello\n" {
		t.Fatalf("expected prompt to contain only messages, got %q", prompt)
	}
}

func TestIsJSONDocument(t *testing.T) {
	if !isJSONDocument(`{"reply":"ok"}`) {
		t.Fatal("expected JSON object to pass")
	}
	if isJSONDocument(`plain text`) {
		t.Fatal("expected plain text to fail")
	}
}

func TestSchemaDocumentUsesConstrainedDecodingSchema(t *testing.T) {
	request := requestDocument{
		ConstrainedDecoding: constraintRequest{
			Type: "json_schema",
			JSONSchema: schemaRequest{
				Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
			},
		},
	}

	if string(request.schemaDocument()) != `{"type":"object","required":["reply"]}` {
		t.Fatalf("expected constrained decoding schema, got %s", request.schemaDocument())
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
