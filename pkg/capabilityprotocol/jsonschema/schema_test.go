package jsonschema

import (
	"encoding/json"
	"testing"
)

func TestObjectBuildsCanonicalSchema(t *testing.T) {
	document := Object(
		Required("title", String()),
		Field("isAllDay", Boolean()),
		Field("visibility", StringEnum("private", "public")),
		Field("attendees", Array(String())),
	).RawMessage()

	var parsed map[string]any
	if errorValue := json.Unmarshal(document, &parsed); errorValue != nil {
		t.Fatalf("expected schema JSON: %v", errorValue)
	}
	if parsed["type"] != "object" {
		t.Fatalf("expected object type, got %+v", parsed)
	}
	properties := parsed["properties"].(map[string]any)
	if properties["title"].(map[string]any)["type"] != "string" {
		t.Fatalf("expected title string schema, got %+v", properties)
	}
	required := parsed["required"].([]any)
	if len(required) != 1 || required[0] != "title" {
		t.Fatalf("expected required title, got %+v", required)
	}
	if parsed["additionalProperties"] != false {
		t.Fatalf("expected additionalProperties=false, got %+v", parsed)
	}
}

func TestObjectNormalizesEmptyProperties(t *testing.T) {
	document := Object().RawMessage()

	var parsed map[string]any
	if errorValue := json.Unmarshal(document, &parsed); errorValue != nil {
		t.Fatalf("expected schema JSON: %v", errorValue)
	}
	properties := parsed["properties"].(map[string]any)
	if len(properties) != 0 {
		t.Fatalf("expected empty properties, got %+v", properties)
	}
	if _, isFound := parsed["required"]; isFound {
		t.Fatalf("expected required to be omitted, got %+v", parsed)
	}
}

func TestRawFallsBackToEmptyObject(t *testing.T) {
	document := Raw(json.RawMessage(`{`)).RawMessage()

	var parsed map[string]any
	if errorValue := json.Unmarshal(document, &parsed); errorValue != nil {
		t.Fatalf("expected schema JSON: %v", errorValue)
	}
	if parsed["type"] != "object" {
		t.Fatalf("expected empty object fallback, got %+v", parsed)
	}
}
