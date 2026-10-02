package jsonschema

import (
	"encoding/json"
	"strings"
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
	if _, isFound := parsed["additionalProperties"]; isFound {
		t.Fatalf("expected portable schema to omit additionalProperties, got %+v", parsed)
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

func TestIntegerRejectsFractionalValues(t *testing.T) {
	document := Object(Required("count", Integer())).RawMessage()

	if errorValue := ValidateInput(document, json.RawMessage(`{"count":1.5}`)); errorValue == nil {
		t.Fatal("expected fractional value to fail integer validation")
	}
	if errorValue := ValidateInput(document, json.RawMessage(`{"count":1}`)); errorValue != nil {
		t.Fatalf("expected integer value: %v", errorValue)
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

func TestValidateInputEnforcesCompleteDescriptorSchema(t *testing.T) {
	schemaDocument := json.RawMessage(`{
		"type":"object",
		"properties":{
			"taskID":{"type":"string","pattern":"^\\S(?:.*\\S)?$"},
			"revision":{"type":"integer","minimum":1}
		},
		"required":["taskID","revision"],
		"additionalProperties":false
	}`)
	invalidInputs := []json.RawMessage{
		nil,
		json.RawMessage(`{"taskID":"task-1"}`),
		json.RawMessage(`{"taskID":" task-1 ","revision":1}`),
		json.RawMessage(`{"taskID":"task-1","revision":0}`),
		json.RawMessage(`{"taskID":"task-1","revision":"1"}`),
		json.RawMessage(`{"taskID":"task-1","revision":1,"confirm":true}`),
	}
	for _, input := range invalidInputs {
		errorValue := ValidateInput(schemaDocument, input)
		if errorValue == nil || !strings.Contains(errorValue.Error(), "does not match") {
			t.Fatalf("expected input %s to fail validation, got %v", string(input), errorValue)
		}
	}
	if errorValue := ValidateInput(schemaDocument, json.RawMessage(`{"taskID":"task-1","revision":1}`)); errorValue != nil {
		t.Fatalf("expected valid input: %v", errorValue)
	}
}

func TestSafeIntegerRefusesAValueNoJavaScriptNumberHoldsExactly(t *testing.T) {
	document := Object(Required("count", SafeInteger())).RawMessage()

	if errorValue := ValidateInput(document, json.RawMessage(`{"count":9007199254740993}`)); errorValue == nil {
		t.Fatal("expected a count past the largest safe integer to fail validation")
	}
	if errorValue := ValidateInput(document, json.RawMessage(`{"count":9007199254740991}`)); errorValue != nil {
		t.Fatalf("expected the largest safe integer: %v", errorValue)
	}
}

func TestWithDialectNamesTheDraftOnTheDocumentItself(t *testing.T) {
	document := Object(Required("title", String())).WithDialect(Draft202012).RawMessage()

	var parsed map[string]any
	if errorValue := json.Unmarshal(document, &parsed); errorValue != nil {
		t.Fatalf("expected schema JSON: %v", errorValue)
	}
	if parsed["$schema"] != Draft202012 {
		t.Fatalf("expected the 2020-12 dialect, got %+v", parsed)
	}
	properties := parsed["properties"].(map[string]any)
	if _, isFound := properties["title"].(map[string]any)["$schema"]; isFound {
		t.Fatalf("expected only the root to name the dialect, got %+v", properties)
	}
}
