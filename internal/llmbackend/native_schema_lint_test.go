package llmbackend

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestNormalizeNativeSchemaConvertsNullableTypeArrayAndRemovesRequiredField(t *testing.T) {
	normalizedSchema, lintResult := NormalizeNativeSchema(json.RawMessage(`{
		"type":"object",
		"properties":{
			"title":{"type":["string","null"],"enum":["draft",null]}
		},
		"required":["title"]
	}`))

	if len(lintResult.RemainingViolations) != 0 {
		t.Fatalf("expected nullable schema to normalize without violations, got %+v", lintResult)
	}
	var document map[string]any
	if errorValue := json.Unmarshal(normalizedSchema, &document); errorValue != nil {
		t.Fatalf("expected normalized schema JSON: %v", errorValue)
	}
	properties := document["properties"].(map[string]any)
	title := properties["title"].(map[string]any)
	if title["type"] != "string" {
		t.Fatalf("expected nullable string union to become string, got %+v", title)
	}
	if requiredContains(stringSliceFromNativeSchema(document["required"]), "title") {
		t.Fatalf("expected nullable field removed from required, got %+v", document["required"])
	}
	enumValues := title["enum"].([]any)
	if nativeEnumContainsNull(enumValues) {
		t.Fatalf("expected null enum value removed, got %+v", enumValues)
	}
}

func TestNormalizeNativeSchemaRemovesBooleanEnum(t *testing.T) {
	normalizedSchema, lintResult := NormalizeNativeSchema(json.RawMessage(`{
		"type":"object",
		"properties":{
			"goalSatisfied":{"type":"boolean","enum":[true]},
			"goalStatus":{"type":"string","enum":["satisfied"]}
		},
		"required":["goalSatisfied","goalStatus"]
	}`))

	if len(lintResult.RemainingViolations) != 0 {
		t.Fatalf("expected boolean enum schema to normalize without violations, got %+v", lintResult)
	}
	var document map[string]any
	if errorValue := json.Unmarshal(normalizedSchema, &document); errorValue != nil {
		t.Fatalf("expected normalized schema JSON: %v", errorValue)
	}
	properties := document["properties"].(map[string]any)
	goalSatisfied := properties["goalSatisfied"].(map[string]any)
	if _, isFound := goalSatisfied["enum"]; isFound {
		t.Fatalf("expected boolean enum removed, got %+v", goalSatisfied)
	}
	goalStatus := properties["goalStatus"].(map[string]any)
	if _, isFound := goalStatus["enum"]; !isFound {
		t.Fatalf("expected string enum kept, got %+v", goalStatus)
	}
	if !requiredContains(stringSliceFromNativeSchema(document["required"]), "goalSatisfied") {
		t.Fatalf("expected goalSatisfied to stay required, got %+v", document["required"])
	}
}

func TestNormalizeNativeSchemaConvertsIntegerToNumber(t *testing.T) {
	normalizedSchema, lintResult := NormalizeNativeSchema(json.RawMessage(`{"type":"object","properties":{"count":{"type":"integer"}}}`))

	if len(lintResult.RemainingViolations) != 0 {
		t.Fatalf("expected integer schema to normalize without violations, got %+v", lintResult)
	}
	var document map[string]any
	if errorValue := json.Unmarshal(normalizedSchema, &document); errorValue != nil {
		t.Fatalf("expected normalized schema JSON: %v", errorValue)
	}
	properties := document["properties"].(map[string]any)
	count := properties["count"].(map[string]any)
	if count["type"] != "number" {
		t.Fatalf("expected integer to become number, got %+v", count)
	}
}

func TestNormalizeNativeSchemaStripsUnsupportedKeywords(t *testing.T) {
	normalizedSchema, lintResult := NormalizeNativeSchema(json.RawMessage(`{
		"type":"object",
		"additionalProperties":false,
		"properties":{"name":{"type":"string","minLength":1}}
	}`))

	if len(lintResult.RemainingViolations) != 0 {
		t.Fatalf("expected unsupported keywords to strip without violations, got %+v", lintResult)
	}
	var document map[string]any
	if errorValue := json.Unmarshal(normalizedSchema, &document); errorValue != nil {
		t.Fatalf("expected normalized schema JSON: %v", errorValue)
	}
	if _, isFound := document["additionalProperties"]; isFound {
		t.Fatalf("expected top-level unsupported keyword stripped, got %+v", document)
	}
	properties := document["properties"].(map[string]any)
	name := properties["name"].(map[string]any)
	if _, isFound := name["minLength"]; isFound {
		t.Fatalf("expected nested unsupported keyword stripped, got %+v", name)
	}
	if len(lintResult.NormalizationsApplied) != 2 {
		t.Fatalf("expected two stripped keyword findings, got %+v", lintResult.NormalizationsApplied)
	}
}

func TestNormalizeNativeSchemaComputesDepthAndEnumMetrics(t *testing.T) {
	_, lintResult := NormalizeNativeSchema(json.RawMessage(`{
		"type":"object",
		"properties":{
			"outer":{
				"type":"object",
				"properties":{
					"inner":{"type":"string","enum":["a","b","c"]}
				}
			},
			"items":{"type":"array","items":{"type":"number","enum":[1,2]}}
		}
	}`))

	if lintResult.MaxNestingDepth != 3 {
		t.Fatalf("expected nested depth to be recorded, got %+v", lintResult)
	}
	if lintResult.TotalPropertyCount != 3 {
		t.Fatalf("expected total property count, got %+v", lintResult)
	}
	if lintResult.LargestEnumSize != 3 {
		t.Fatalf("expected largest enum size, got %+v", lintResult)
	}
}

func TestOpenRouterNativeActionErrorIncludesSchemaLintDiagnostics(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatalf("expected secret fixture: %v", errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "google/gemini-test",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader(`{"error":"bad schema"}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Model:    "google/gemini-test",
		Messages: []Message{{Role: "user", Content: "publish"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name: "blueclaw_agent_turn_action",
			Document: testActionSchemaForDescriptors(t, []capabilities.Descriptor{{
				Name:        "file_write",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":["string","null"]}},"required":["path","content"]}`),
			}}),
		},
	})

	if errorValue == nil {
		t.Fatal("expected provider error")
	}
	for _, expectedFragment := range []string{"toolCount=", "maxDepth=", "lintFindings=", "converted_nullable_type_union", "removed_nullable_field"} {
		if !strings.Contains(errorValue.Error(), expectedFragment) {
			t.Fatalf("expected diagnostics fragment %q in %v", expectedFragment, errorValue)
		}
	}
}

func TestNativeActionToolSchemasArePortableAfterNormalization(t *testing.T) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name: "blueclaw_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, []capabilities.Descriptor{{
			Name:        "file_write",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":["string","null"]},"retries":{"type":"integer","minimum":0}},"required":["path","content","retries"],"additionalProperties":false}`),
		}}),
	})
	if errorValue != nil {
		t.Fatalf("expected native action tools: %v", errorValue)
	}
	if !isActionSchema {
		t.Fatal("expected action schema")
	}
	tool := toolSet.ToolByName[nativeActionFunctionName("continue", "file_write")]
	assertNativeSchemaIsProviderSafe(t, "file_write", tool.Parameters)
	if len(toolSet.NativeSchemaLint.NormalizationsApplied) == 0 {
		t.Fatalf("expected schema normalization findings, got %+v", toolSet.NativeSchemaLint)
	}
}
