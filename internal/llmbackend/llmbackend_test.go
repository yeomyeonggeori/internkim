package llmbackend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (transport roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestOpenRouterStructuredRequestPreservesSchema(t *testing.T) {
	seed := int64(42)
	temperature := 0.1
	requestDocument, errorValue := buildOpenRouterStructuredRequest(StructuredRequest{
		Model: "openrouter/model",
		Messages: []Message{
			{Role: "user", Content: "hello"},
		},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "reply",
			Document:           json.RawMessage(`{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"],"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
		GenerationOptions:     &GenerationOptions{Seed: &seed, Temperature: &temperature},
		RequireParameters:     true,
		EnableResponseHealing: true,
	}, "openrouter/model")
	if errorValue != nil {
		t.Fatalf("expected request document: %v", errorValue)
	}

	var document map[string]any
	if errorValue := json.Unmarshal(requestDocument, &document); errorValue != nil {
		t.Fatalf("expected request to decode: %v", errorValue)
	}
	responseFormat := document["response_format"].(map[string]any)
	jsonSchema := responseFormat["json_schema"].(map[string]any)
	schema := jsonSchema["schema"].(map[string]any)
	required := schema["required"].([]any)
	if required[0] != "reply" {
		t.Fatalf("expected schema to be preserved, got %+v", schema)
	}
	if jsonSchema["strict"] != true {
		t.Fatalf("expected strict schema, got %+v", jsonSchema)
	}
	if document["seed"] != float64(seed) {
		t.Fatalf("expected seed to be forwarded, got %+v", document)
	}
	if document["temperature"] != temperature {
		t.Fatalf("expected temperature to be forwarded, got %+v", document)
	}
}

func TestOpenRouterStructuredRequestOmitsEmptyGenerationOptions(t *testing.T) {
	requestDocument, errorValue := buildOpenRouterStructuredRequest(StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "reply",
			Document:           json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
	}, "openrouter/model")
	if errorValue != nil {
		t.Fatalf("expected request document: %v", errorValue)
	}

	var document map[string]any
	if errorValue := json.Unmarshal(requestDocument, &document); errorValue != nil {
		t.Fatalf("expected request to decode: %v", errorValue)
	}
	if _, isFound := document["seed"]; isFound {
		t.Fatalf("expected empty seed to be omitted, got %+v", document)
	}
	if _, isFound := document["temperature"]; isFound {
		t.Fatalf("expected empty temperature to be omitted, got %+v", document)
	}
}

func TestOpenRouterBackendUsesChatToolCallingForAgentActions(t *testing.T) {
	seed := int64(99)
	temperature := 0.3
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var receivedDocument map[string]any
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.ai/api/v1/chat/completions",
		ModelName: "configured-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/api/v1/chat/completions" {
				t.Fatalf("expected chat completions path, got %s", request.URL.Path)
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&receivedDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"continue__site_app_publish","arguments":"{\"toolInput\":{\"siteID\":\"site-1\"},\"message\":\"publishing\",\"executionStateUpdate\":{}}"}}]}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages:               []Message{{Role: "user", Content: "publish"}},
		StructuredOutputSchema: testAgentActionSchema(),
		GenerationOptions:      &GenerationOptions{Seed: &seed, Temperature: &temperature},
	})

	if errorValue != nil {
		t.Fatalf("expected native action response: %v", errorValue)
	}
	if response.Content != `{"action":"continue","executionStateUpdate":{},"message":"publishing","toolInput":{"siteID":"site-1"},"toolName":"site.app.publish"}` {
		t.Fatalf("expected action JSON, got %s", response.Content)
	}
	if response.ConstraintMode != ConstraintModeNativeToolCall {
		t.Fatalf("expected native tool call constraint, got %q", response.ConstraintMode)
	}
	if _, isFound := receivedDocument["response_format"]; isFound {
		t.Fatalf("expected native tool request to omit response_format, got %+v", receivedDocument)
	}
	if receivedDocument["tool_choice"] != "required" {
		t.Fatalf("expected required tool choice, got %+v", receivedDocument)
	}
	if receivedDocument["parallel_tool_calls"] != false {
		t.Fatalf("expected parallel tool calls to be disabled, got %+v", receivedDocument)
	}
	tools := receivedDocument["tools"].([]any)
	if !openRouterRequestHasTool(tools, "finish") {
		t.Fatalf("expected finish control tool, got %+v", tools)
	}
	parameters := openRouterRequestToolParameters(t, tools, "continue__site_app_publish")
	if _, isFound := parameters["additionalProperties"]; isFound {
		t.Fatalf("expected OpenRouter native tool parameters to omit additionalProperties, got %+v", parameters)
	}
	properties := parameters["properties"].(map[string]any)
	toolInput := properties["toolInput"].(map[string]any)
	toolInputProperties := toolInput["properties"].(map[string]any)
	if _, isFound := toolInputProperties["siteID"]; !isFound {
		t.Fatalf("expected projected tool parameters to preserve siteID, got %+v", parameters)
	}
	required := parameters["required"].([]any)
	if len(required) != 2 || required[0] != "toolInput" || required[1] != "executionStateUpdate" {
		t.Fatalf("expected required field to be preserved, got %+v", parameters)
	}
	if receivedDocument["seed"] != float64(seed) {
		t.Fatalf("expected seed to be forwarded, got %+v", receivedDocument)
	}
	if receivedDocument["temperature"] != temperature {
		t.Fatalf("expected temperature to be forwarded, got %+v", receivedDocument)
	}
}

func TestOpenRouterBackendAcceptsProviderReturnedToolName(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.ai/api/v1/chat/completions",
		ModelName: "configured-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"calendar.event.add","arguments":"{\"title\":\"휴가\",\"startISO\":\"2026-05-09T09:00:00+09:00\",\"endISO\":\"2026-05-09T18:00:00+09:00\"}"}}]}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "내일 휴가 등록해줘"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "blueclaw_agent_turn_action",
			Document: testActionSchemaForDescriptors(t, capabilities.CalendarDescriptors()),
		},
	})

	if errorValue != nil {
		t.Fatalf("expected provider-returned tool name to resolve: %v", errorValue)
	}
	if !strings.Contains(response.Content, `"toolName":"calendar.event.add"`) {
		t.Fatalf("expected calendar tool action, got %s", response.Content)
	}
}

func TestOpenRouterBackendRejectsActionContentWhenToolCallIsMissing(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.ai/api/v1/chat/completions",
		ModelName: "configured-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"action\":\"finish\",\"finishMessage\":\"할 수 있는 일을 설명드릴게요.\",\"goalStatus\":\"satisfied\",\"goalSatisfied\":true,\"completionEvidence\":[],\"qualityReview\":[]}"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages:               []Message{{Role: "user", Content: "넌 뭐 할줄 알아?"}},
		StructuredOutputSchema: testAgentActionSchema(),
	})

	if errorValue == nil || !strings.Contains(errorValue.Error(), "did not include tool_calls") {
		t.Fatalf("expected missing tool_calls error, got response=%+v error=%v", response, errorValue)
	}
}

func TestOpenAICompatibleActionToolRequestUsesGenerationOptions(t *testing.T) {
	seed := int64(12)
	temperature := 0.6
	request := openAIActionToolRequest("local-model", []Message{{Role: "user", Content: "publish"}}, []nativeActionTool{{
		FunctionName: "continue__site_app_publish",
		Description:  "Call site.app.publish",
		Action:       "continue",
		ToolName:     "site.app.publish",
		Parameters:   json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
	}}, GenerationOptions{Seed: &seed, Temperature: &temperature})

	if request.Seed == nil || *request.Seed != seed {
		t.Fatalf("expected seed on OpenAI-compatible request, got %+v", request)
	}
	if request.Temperature == nil || *request.Temperature != temperature {
		t.Fatalf("expected temperature on OpenAI-compatible request, got %+v", request)
	}
	if len(request.Tools) != 1 || request.Tools[0].Function.Name != "continue__site_app_publish" {
		t.Fatalf("expected native tool call shape to remain, got %+v", request.Tools)
	}
	if request.ToolChoice != "required" {
		t.Fatalf("expected required tool choice, got %+v", request)
	}
	if request.ParallelTools == nil || *request.ParallelTools {
		t.Fatalf("expected parallel tool calls disabled, got %+v", request)
	}
}

func TestOpenAICompatibleMessagePartsBecomeMultimodalContent(t *testing.T) {
	request := openAIChatRequest("local-model", []Message{{
		Role:    "user",
		Content: "inspect this",
		Parts: []MessagePart{{
			Type:       "image",
			MimeType:   "image/png",
			DataBase64: "aW1hZ2U=",
		}},
	}}, nil, GenerationOptions{})

	if len(request.Messages) != 1 {
		t.Fatalf("expected one message, got %+v", request.Messages)
	}
	parts, ok := request.Messages[0].Content.([]map[string]any)
	if !ok || len(parts) != 2 {
		t.Fatalf("expected multimodal content parts, got %#v", request.Messages[0].Content)
	}
	imageURL, ok := parts[1]["image_url"].(map[string]string)
	if !ok || imageURL["url"] != "data:image/png;base64,aW1hZ2U=" {
		t.Fatalf("expected image data URL, got %#v", parts[1])
	}
}

func TestNativeActionToolsExposeFinishAsFinish(t *testing.T) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(testAgentActionSchema())
	if errorValue != nil {
		t.Fatalf("expected native tool set: %v", errorValue)
	}
	if !isActionSchema {
		t.Fatal("expected action schema")
	}
	tool, isFound := toolSet.ToolByName["finish"]
	if !isFound {
		t.Fatalf("expected finish tool, got %+v", toolSet.Tools)
	}
	if tool.Action != "finish" {
		t.Fatalf("expected finish tool to map to finish, got %+v", tool)
	}
	content, errorValue := nativeActionJSON(toolSet, "finish", `{"message":"done","goalStatus":"satisfied","goalSatisfied":true,"completionEvidence":[],"qualityReview":[]}`)
	if errorValue != nil {
		t.Fatalf("expected finish action JSON: %v", errorValue)
	}
	if !strings.Contains(content, `"action":"finish"`) || !strings.Contains(content, `"message":"done"`) {
		t.Fatalf("expected finish action, got %s", content)
	}
}

func TestNativeActionToolsRejectFunctionNameCollisions(t *testing.T) {
	_, _, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name: "blueclaw_agent_turn_action",
		Document: json.RawMessage(`{"oneOf":[
			{"type":"object","properties":{"action":{"type":"string","enum":["continue"]},"toolName":{"type":"string","enum":["a.b"]},"toolInput":{"type":"object"}},"required":["action","toolName","toolInput"]},
			{"type":"object","properties":{"action":{"type":"string","enum":["continue"]},"toolName":{"type":"string","enum":["a/b"]},"toolInput":{"type":"object"}},"required":["action","toolName","toolInput"]}
		]}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "maps multiple actions") {
		t.Fatalf("expected function name collision error, got %v", errorValue)
	}
}

func TestNativeActionToolsRemoveUndefinedRequiredParameters(t *testing.T) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name: "blueclaw_agent_turn_action",
		Document: json.RawMessage(`{"oneOf":[
			{"type":"object","properties":{"action":{"type":"string","enum":["continue"]},"toolName":{"type":"string","enum":["browser.open"]},"toolInput":{"type":"object","required":["url"],"additionalProperties":false}},"required":["action","toolName","toolInput"]}
		]}`),
	})
	if errorValue != nil {
		t.Fatalf("expected native tool set: %v", errorValue)
	}
	if !isActionSchema || len(toolSet.Tools) != 1 {
		t.Fatalf("expected one native action tool, got action=%v tools=%+v", isActionSchema, toolSet.Tools)
	}

	var parameters map[string]any
	if errorValue := json.Unmarshal(toolSet.Tools[0].Parameters, &parameters); errorValue != nil {
		t.Fatalf("expected parameters to decode: %v", errorValue)
	}
	properties, isFound := nativeToolInputProperties(parameters)
	if !isFound || len(properties) != 0 {
		t.Fatalf("expected empty properties to be explicit, got %+v", parameters)
	}
	assertNativeRequiredFieldsHaveProperties(t, "browser.open", parameters)
	if _, isFound := parameters["additionalProperties"]; isFound {
		t.Fatalf("expected unsupported native schema keywords to be removed, got %+v", parameters)
	}
}

func TestNativeActionToolsSanitizeProviderSpecificSchemaKeywords(t *testing.T) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name: "blueclaw_agent_turn_action",
		Document: json.RawMessage(`{"oneOf":[
			{"type":"object","properties":{"action":{"type":"string","enum":["continue"]},"toolName":{"type":"string","enum":["calendar.event.add"]},"toolInput":{"type":"object","properties":{"people":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"}}]},"title":{"type":"string","default":"Untitled"}},"required":["title"],"additionalProperties":false}},"required":["action","toolName","toolInput"]}
		]}`),
	})
	if errorValue != nil {
		t.Fatalf("expected native tool set: %v", errorValue)
	}
	if !isActionSchema || len(toolSet.Tools) != 1 {
		t.Fatalf("expected one native action tool, got action=%v tools=%+v", isActionSchema, toolSet.Tools)
	}

	var parameters map[string]any
	if errorValue := json.Unmarshal(toolSet.Tools[0].Parameters, &parameters); errorValue != nil {
		t.Fatalf("expected parameters to decode: %v", errorValue)
	}
	if documentContainsKey(parameters, "oneOf") || documentContainsKey(parameters, "additionalProperties") || documentContainsKey(parameters, "default") {
		t.Fatalf("expected provider-specific schema keywords to be removed, got %+v", parameters)
	}
	properties, _ := nativeToolInputProperties(parameters)
	people := properties["people"].(map[string]any)
	if people["type"] != "string" {
		t.Fatalf("expected oneOf to collapse to first provider-compatible variant, got %+v", people)
	}
}

func TestNativeActionToolsRemoveNonStringEnumsForOpenRouterCompatibility(t *testing.T) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name: "blueclaw_agent_turn_action",
		Document: json.RawMessage(`{"oneOf":[
			{"type":"object","properties":{"action":{"type":"string","enum":["continue"]},"toolName":{"type":"string","enum":["calendar.event.add"]},"toolInput":{"type":"object","properties":{"reminderLeadHours":{"type":"integer","enum":[1,2,3]},"color":{"type":"string","enum":["blue","green"]}},"required":["reminderLeadHours"],"additionalProperties":false}},"required":["action","toolName","toolInput"]}
		]}`),
	})
	if errorValue != nil {
		t.Fatalf("expected native tool set: %v", errorValue)
	}
	if !isActionSchema || len(toolSet.Tools) != 1 {
		t.Fatalf("expected one native action tool, got action=%v tools=%+v", isActionSchema, toolSet.Tools)
	}

	var parameters map[string]any
	if errorValue := json.Unmarshal(toolSet.Tools[0].Parameters, &parameters); errorValue != nil {
		t.Fatalf("expected parameters to decode: %v", errorValue)
	}
	properties, _ := nativeToolInputProperties(parameters)
	reminderLeadHours := properties["reminderLeadHours"].(map[string]any)
	if _, isFound := reminderLeadHours["enum"]; isFound {
		t.Fatalf("expected non-string enum to be removed, got %+v", reminderLeadHours)
	}
	color := properties["color"].(map[string]any)
	enumValues := color["enum"].([]any)
	if len(enumValues) != 2 || enumValues[0] != "blue" || enumValues[1] != "green" {
		t.Fatalf("expected string enum to be preserved, got %+v", color)
	}
}

func TestNativeActionToolsPreserveCalendarRequiredForOpenRouterCompatibility(t *testing.T) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name:     "blueclaw_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, capabilities.CalendarDescriptors()),
	})
	if errorValue != nil {
		t.Fatalf("expected native tool set: %v", errorValue)
	}
	if !isActionSchema {
		t.Fatal("expected action schema")
	}

	calendarAddTool := toolSet.ToolByName["continue__calendar_event_add"]
	var parameters map[string]any
	if errorValue := json.Unmarshal(calendarAddTool.Parameters, &parameters); errorValue != nil {
		t.Fatalf("expected calendar parameters: %v", errorValue)
	}
	properties, _ := nativeToolInputProperties(parameters)
	for _, fieldName := range []string{"title", "startISO", "endISO"} {
		if _, isFound := properties[fieldName]; !isFound {
			t.Fatalf("expected property %q in calendar schema: %+v", fieldName, parameters)
		}
	}
	required := nativeToolInputRequired(parameters)
	if len(required) != 3 {
		t.Fatalf("expected calendar required fields to be preserved, got %+v", parameters)
	}
	assertNativeRequiredFieldsHaveProperties(t, "calendar.event.add", parameters)
}

func TestNativeActionToolsProjectEveryDefaultCapabilitySchema(t *testing.T) {
	descriptors := append(capabilities.DefaultToolDescriptors(), capabilities.GoogleWorkspaceDescriptors()...)
	for _, descriptor := range descriptors {
		toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
			Name:     "blueclaw_agent_turn_action",
			Document: testActionSchemaForDescriptors(t, []capabilities.Descriptor{descriptor}),
		})
		if errorValue != nil {
			t.Fatalf("expected native tool set for %s: %v", descriptor.Name, errorValue)
		}
		if !isActionSchema {
			t.Fatal("expected action schema")
		}
		tool := toolSet.ToolByName[nativeActionFunctionName("continue", descriptor.Name)]
		if strings.TrimSpace(tool.FunctionName) == "" {
			t.Fatalf("expected native tool for %s", descriptor.Name)
		}
		assertNativeSchemaIsProviderSafe(t, descriptor.Name, tool.Parameters)
	}
}

func TestNativeActionToolsCompactLargeToolSetsForOpenRouterCompatibility(t *testing.T) {
	descriptors := make([]capabilities.Descriptor, 0, openRouterNativeToolMaxFunctionCount+1)
	for index := 0; index <= openRouterNativeToolMaxFunctionCount; index++ {
		descriptors = append(descriptors, capabilities.Descriptor{
			Name:        fmt.Sprintf("tool.%02d", index),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"],"additionalProperties":false}`),
		})
	}
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name:     "blueclaw_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, descriptors),
	})
	if errorValue != nil {
		t.Fatalf("expected compact native tool set: %v", errorValue)
	}
	if !isActionSchema {
		t.Fatal("expected action schema")
	}
	if len(toolSet.Tools) != 1 {
		t.Fatalf("expected dispatcher-only tool set, got %+v", toolSet.Tools)
	}
	dispatcher := toolSet.ToolByName["continue"]
	if !dispatcher.IsDispatcher || len(dispatcher.ToolNames) != len(descriptors) {
		t.Fatalf("expected continue dispatcher with tool names, got %+v", dispatcher)
	}
	content, errorValue := nativeActionJSON(toolSet, "continue", `{"toolName":"tool.03","toolInputJSON":"{\"value\":\"ok\"}","executionStateUpdate":{}}`)
	if errorValue != nil {
		t.Fatalf("expected dispatcher action JSON: %v", errorValue)
	}
	if !strings.Contains(content, `"toolName":"tool.03"`) || !strings.Contains(content, `"value":"ok"`) || strings.Contains(content, "toolInputJSON") {
		t.Fatalf("expected decoded dispatcher payload, got %s", content)
	}
}

func TestOpenRouterBackendResolvesDefaultModel(t *testing.T) {
	backend := OpenRouterBackend{ModelName: "google/default-remote"}
	for _, modelName := range []string{"", "default", "DEFAULT", "local/anything"} {
		if resolvedModelName := backend.resolveModelName(modelName); resolvedModelName != "google/default-remote" {
			t.Fatalf("expected default remote model for %q, got %q", modelName, resolvedModelName)
		}
	}
}

func TestOllamaBackendStructuredOutputIsUnsupported(t *testing.T) {
	backend := OllamaBackend{BaseURL: "https://ollama.test", ModelName: "gemma3:1b"}

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "deterministic structured output") {
		t.Fatalf("expected deterministic structured output error, got %v", errorValue)
	}
}

func TestLlamaCppBackendStructuredOutputUsesResponseFormat(t *testing.T) {
	var receivedDocument map[string]any
	backend := LlamaCppBackend{
		BaseURL:   "https://llamacpp.test",
		ModelName: "default",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/v1/chat/completions" {
				t.Fatalf("unexpected path: %s", request.URL.Path)
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&receivedDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"reply\":\"ok\"}"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected llamacpp completion: %v", errorValue)
	}
	responseFormat, isFound := receivedDocument["response_format"].(map[string]any)
	if !isFound {
		t.Fatalf("expected response_format field, got %+v", receivedDocument)
	}
	if responseFormat["type"] != "json_schema" {
		t.Fatalf("expected json_schema response_format, got %+v", responseFormat)
	}
	if response.ConstraintMode != ConstraintModeLlamaJSONSchema {
		t.Fatalf("expected llama JSON schema mode, got %q", response.ConstraintMode)
	}
}

func TestLlamaCppBackendUsesChatToolCallingForAgentActions(t *testing.T) {
	var receivedDocument map[string]any
	backend := LlamaCppBackend{
		BaseURL:   "https://llamacpp.test",
		ModelName: "default",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/v1/chat/completions" {
				t.Fatalf("unexpected path: %s", request.URL.Path)
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&receivedDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"finish","arguments":"{\"message\":\"done\",\"goalStatus\":\"satisfied\",\"goalSatisfied\":true,\"completionEvidence\":[],\"qualityReview\":[]}"}}]}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages:               []Message{{Role: "user", Content: "finish"}},
		StructuredOutputSchema: testAgentActionSchema(),
	})

	if errorValue != nil {
		t.Fatalf("expected native action response: %v", errorValue)
	}
	if response.Content != `{"action":"finish","completionEvidence":[],"goalSatisfied":true,"goalStatus":"satisfied","message":"done","qualityReview":[]}` {
		t.Fatalf("expected final reply action, got %s", response.Content)
	}
	if _, isFound := receivedDocument["response_format"]; isFound {
		t.Fatalf("expected native tool request to omit response_format, got %+v", receivedDocument)
	}
	if _, isFound := receivedDocument["tools"]; !isFound {
		t.Fatalf("expected tools in request, got %+v", receivedDocument)
	}
	if receivedDocument["tool_choice"] != "required" {
		t.Fatalf("expected required tool choice, got %+v", receivedDocument)
	}
}

func TestManagedLlamaCppBackendStartsServiceAndRetriesText(t *testing.T) {
	chatRequests := 0
	healthRequests := 0
	startCommands := 0
	backend := ManagedLlamaCppBackend{
		Backend: LlamaCppBackend{
			BaseURL:   "http://llamacpp.test",
			ModelName: "local/gemma",
			HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				switch request.URL.Path {
				case "/health":
					healthRequests++
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("OK")),
						Header:     make(http.Header),
					}, nil
				case "/v1/chat/completions":
					chatRequests++
					if chatRequests == 1 {
						return nil, &url.Error{Op: "Post", URL: request.URL.String(), Err: errors.New("connection refused")}
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ready"}}]}`)),
						Header:     make(http.Header),
					}, nil
				default:
					t.Fatalf("unexpected path: %s", request.URL.Path)
					return nil, nil
				}
			})},
		},
		ServiceName:  "internkim-llamacpp.service",
		PollInterval: time.Millisecond,
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = standardInput
			startCommands++
			if executablePath != "systemctl" || strings.Join(arguments, " ") != "start internkim-llamacpp.service" {
				t.Fatalf("unexpected start command: %s %v", executablePath, arguments)
			}
			return nil, nil
		},
	}

	response, errorValue := backend.CompleteText(context.Background(), TextRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if errorValue != nil {
		t.Fatalf("expected managed llama.cpp retry: %v", errorValue)
	}
	if response.Content != "ready" {
		t.Fatalf("expected retry response, got %+v", response)
	}
	if startCommands != 1 || healthRequests != 1 || chatRequests != 2 {
		t.Fatalf("expected one start, one health check, two chat requests; got starts=%d health=%d chat=%d", startCommands, healthRequests, chatRequests)
	}
}

func TestManagedLlamaCppBackendDoesNotStartServiceForProviderError(t *testing.T) {
	startCommands := 0
	backend := ManagedLlamaCppBackend{
		Backend: LlamaCppBackend{
			BaseURL:   "http://llamacpp.test",
			ModelName: "local/gemma",
			HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"error":"schema rejected"}`)),
					Header:     make(http.Header),
				}, nil
			})},
		},
		ServiceName:  "internkim-llamacpp.service",
		PollInterval: time.Millisecond,
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = executablePath
			_ = arguments
			_ = standardInput
			startCommands++
			return nil, nil
		},
	}

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "schema rejected") {
		t.Fatalf("expected provider error, got %v", errorValue)
	}
	if startCommands != 0 {
		t.Fatalf("expected provider error not to start service, got %d", startCommands)
	}
}

func TestMLXBackendStructuredOutputIsUnsupported(t *testing.T) {
	backend := MLXBackend{BaseURL: "https://mlx.test", ModelName: "default"}

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "deterministic structured output") {
		t.Fatalf("expected deterministic structured output error, got %v", errorValue)
	}
}

func TestStructuredOutputValidationRejectsNonJSON(t *testing.T) {
	backend := LlamaCppBackend{
		BaseURL:   "https://llamacpp.test",
		ModelName: "default",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			_ = request
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"plain text"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue == nil {
		t.Fatal("expected validation failure for non-JSON content")
	}
}

func TestAutoProviderReportsAggregateErrorWhenAllFail(t *testing.T) {
	auto := AutoProvider{
		Providers: []Provider{
			staticProvider{errorValue: errors.New("one")},
			staticProvider{errorValue: errors.New("two")},
		},
	}
	_, errorValue := auto.CompleteText(context.Background(), TextRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "one") || !strings.Contains(errorValue.Error(), "two") {
		t.Fatalf("expected aggregate error from chain, got %v", errorValue)
	}
}

func TestAutoProviderReportsNoProviderError(t *testing.T) {
	auto := AutoProvider{Providers: nil}
	_, errorValue := auto.CompleteText(context.Background(), TextRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "no llm provider") {
		t.Fatalf("expected no provider error, got %v", errorValue)
	}
}

func TestStructuredRequestTraceIncludesReproductionMetadata(t *testing.T) {
	seed := int64(1234)
	temperature := 0.2
	trace := structuredRequestTrace(StructuredRequest{
		Model:         "openrouter/model",
		Provider:      "openrouter",
		ExecutionMode: "remote",
		Messages:      []Message{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "blueclaw_agent_turn_action",
			Document: json.RawMessage(`{"type":"object","properties":{"reply":{"type":"string"}}}`),
		},
		GenerationOptions: &GenerationOptions{Seed: &seed, Temperature: &temperature},
	})

	for _, expectedFragment := range []string{
		"kind=structured",
		"constraintMode=native_tool_call",
		"toolChoice=required",
		"executionMode=remote",
		"provider=openrouter",
		"model=openrouter/model",
		"schemaName=blueclaw_agent_turn_action",
		"schemaHash=",
		"messagesHash=",
		"seed=1234",
		"temperature=0.2",
	} {
		if !strings.Contains(trace, expectedFragment) {
			t.Fatalf("expected trace to contain %q, got %q", expectedFragment, trace)
		}
	}
}

func TestBuildLocalProviderSetUsesRequestedOrder(t *testing.T) {
	providerSet := BuildLocalProviderSet(LocalProviderConfig{
		ProviderOrder:   []string{"llamacpp", "ollama", "mlx"},
		OllamaBaseURL:   "http://ollama.test",
		OllamaModel:     "gemma3:1b",
		LlamaCppBaseURL: "http://llamacpp.test",
		LlamaCppModel:   "local/gemma",
		MLXBaseURL:      "http://mlx.test",
		MLXModel:        "mlx-community/gemma",
	})

	expectedNames := []string{"llamacpp", "ollama", "mlx"}
	if len(providerSet.Backends) != len(expectedNames) {
		t.Fatalf("expected %d backends, got %d", len(expectedNames), len(providerSet.Backends))
	}
	for index, backend := range providerSet.Backends {
		if backend.Name() != expectedNames[index] {
			t.Fatalf("expected %s at index %d, got %s", expectedNames[index], index, backend.Name())
		}
	}
	if providerSet.ModelByBackendName["llamacpp"] != "local/gemma" {
		t.Fatalf("expected llama.cpp model mapping, got %+v", providerSet.ModelByBackendName)
	}
}

func TestBuildLocalProviderSetSkipsUnknownProviders(t *testing.T) {
	providerSet := BuildLocalProviderSet(LocalProviderConfig{
		ProviderOrder: []string{"unknown", "ollama"},
	})

	if len(providerSet.Backends) != 1 {
		t.Fatalf("expected one backend, got %d", len(providerSet.Backends))
	}
	if providerSet.Backends[0].Name() != "ollama" {
		t.Fatalf("expected ollama backend, got %s", providerSet.Backends[0].Name())
	}
}

func TestBuildLocalProviderSetCreatesRequestedLiteRTAccelerator(t *testing.T) {
	providerSet := BuildLocalProviderSet(LocalProviderConfig{
		ProviderOrder:    []string{"litert"},
		Accelerator:      "cpu",
		LiteRTModelPath:  "/models/model.litertlm",
		LiteRTRunnerPath: "/usr/local/bin/internkim-local-llm-runner",
	})

	if len(providerSet.Backends) != 1 {
		t.Fatalf("expected one LiteRT backend, got %d", len(providerSet.Backends))
	}
	backend, isLiteRT := providerSet.Backends[0].(LiteRTProvider)
	if !isLiteRT {
		t.Fatalf("expected LiteRT provider, got %T", providerSet.Backends[0])
	}
	if backend.Variant != "cpu" {
		t.Fatalf("expected cpu accelerator, got %q", backend.Variant)
	}
	if providerSet.ModelByBackendName["litert-cpu"] != "/models/model.litertlm" {
		t.Fatalf("expected LiteRT model mapping, got %+v", providerSet.ModelByBackendName)
	}
}

func TestLocalProviderSetDoesNotFallbackForStructuredByDefault(t *testing.T) {
	providerSet := BuildLocalProviderSet(LocalProviderConfig{
		ProviderOrder: []string{"ollama", "llamacpp"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			t.Fatalf("expected structured request not to reach llama.cpp after ollama rejection")
			return nil, nil
		})},
	})

	_, errorValue := providerSet.Provider.CompleteStructured(context.Background(), StructuredRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "ollama") {
		t.Fatalf("expected first provider structured error, got %v", errorValue)
	}
}

func TestParseProviderOrderUsesFallbackForEmptyValue(t *testing.T) {
	order := ParseProviderOrder("", []string{"llamacpp", "ollama"})
	if strings.Join(order, ",") != "llamacpp,ollama" {
		t.Fatalf("expected fallback order, got %v", order)
	}
}

func TestOpenRouterPingDetectsPlaceholderKey(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "key")
	if errorValue := os.WriteFile(secretPath, []byte("internkim-simulation-openrouter-api-key"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{KeyPath: secretPath}
	if errorValue := backend.Ping(context.Background()); errorValue == nil {
		t.Fatal("expected placeholder key to fail ping")
	}
}

func TestOllamaStreamTextEmitsTokens(t *testing.T) {
	backend := OllamaBackend{
		BaseURL:   "https://ollama.test",
		ModelName: "gemma3:1b",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			body := strings.Join([]string{
				`{"message":{"role":"assistant","content":"hel"},"done":false}`,
				`{"message":{"role":"assistant","content":"lo"},"done":false}`,
				`{"message":{"role":"assistant","content":""},"done":true}`,
			}, "\n")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	tokens := []string{}
	errorValue := backend.StreamText(context.Background(), TextRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	}, func(token string) {
		tokens = append(tokens, token)
	})
	if errorValue != nil {
		t.Fatalf("expected stream success: %v", errorValue)
	}
	if strings.Join(tokens, "") != "hello" {
		t.Fatalf("expected hello, got %v", tokens)
	}
}

func testAgentActionSchema() StructuredOutputSchema {
	return StructuredOutputSchema{
		Name: "blueclaw_agent_turn_action",
		Document: json.RawMessage(`{"oneOf":[
			{"type":"object","properties":{"action":{"type":"string","enum":["finish"]},"message":{"type":"string"},"goalStatus":{"type":"string","enum":["satisfied"]},"goalSatisfied":{"type":"boolean"},"completionEvidence":{"type":"array"},"qualityReview":{"type":"array"},"executionStateUpdate":{"type":"object"}},"required":["action","message","goalStatus","goalSatisfied","completionEvidence","qualityReview","executionStateUpdate"],"additionalProperties":false},
			{"type":"object","properties":{"action":{"type":"string","enum":["continue"]},"toolName":{"type":"string","enum":["site.app.publish"]},"toolInput":{"type":"object","properties":{"siteID":{"type":"string"}},"required":["siteID"],"additionalProperties":false},"message":{"type":"string"},"executionStateUpdate":{"type":"object"}},"required":["action","toolName","toolInput","executionStateUpdate"],"additionalProperties":false}
		]}`),
		IsStrictlyEnforced: true,
	}
}

func documentContainsKey(value any, key string) bool {
	document, isObject := value.(map[string]any)
	if isObject {
		if _, isFound := document[key]; isFound {
			return true
		}
		for _, fieldValue := range document {
			if documentContainsKey(fieldValue, key) {
				return true
			}
		}
		return false
	}
	values, isArray := value.([]any)
	if isArray {
		for _, item := range values {
			if documentContainsKey(item, key) {
				return true
			}
		}
	}
	return false
}

func testActionSchemaForDescriptors(t *testing.T, descriptors []capabilities.Descriptor) json.RawMessage {
	t.Helper()
	variants := make([]any, 0, len(descriptors))
	for _, descriptor := range descriptors {
		inputSchema := descriptor.InputSchema
		if len(inputSchema) == 0 {
			inputSchema = json.RawMessage(`{"type":"object"}`)
		}
		var toolInput any
		if errorValue := json.Unmarshal(inputSchema, &toolInput); errorValue != nil {
			t.Fatalf("input schema for %s is invalid: %v", descriptor.Name, errorValue)
		}
		variants = append(variants, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":    map[string]any{"type": "string", "enum": []string{"continue"}},
				"toolName":  map[string]any{"type": "string", "enum": []string{descriptor.Name}},
				"toolInput": toolInput,
			},
			"required":             []string{"action", "toolName", "toolInput"},
			"additionalProperties": false,
		})
	}
	document, errorValue := json.Marshal(map[string]any{"oneOf": variants})
	if errorValue != nil {
		t.Fatalf("expected action schema: %v", errorValue)
	}
	return document
}

func assertNativeSchemaIsProviderSafe(t *testing.T, toolName string, schema json.RawMessage) {
	t.Helper()
	var document any
	if errorValue := json.Unmarshal(schema, &document); errorValue != nil {
		t.Fatalf("schema for %s is invalid: %v", toolName, errorValue)
	}
	assertNativeSchemaValueIsProviderSafe(t, toolName, document)
}

func assertNativeSchemaValueIsProviderSafe(t *testing.T, toolName string, value any) {
	t.Helper()
	assertNativeSchemaMapIsProviderSafe(t, toolName, value, false)
}

func assertNativeSchemaMapIsProviderSafe(t *testing.T, toolName string, value any, isPropertiesMap bool) {
	t.Helper()
	document, isObject := value.(map[string]any)
	if isObject {
		for fieldName, fieldValue := range document {
			if isPropertiesMap {
				assertNativeSchemaMapIsProviderSafe(t, toolName, fieldValue, false)
				continue
			}
			if !openRouterNativeToolSchemaKeywordByName[fieldName] {
				t.Fatalf("schema for %s has unsupported key %s: %+v", toolName, fieldName, document)
			}
			if fieldName == "properties" {
				assertNativeSchemaMapIsProviderSafe(t, toolName, fieldValue, true)
				continue
			}
			assertNativeSchemaMapIsProviderSafe(t, toolName, fieldValue, false)
		}
		if document["type"] == "object" {
			if _, isFound := document["properties"]; !isFound {
				t.Fatalf("schema for %s has object without properties: %+v", toolName, document)
			}
			assertNativeRequiredFieldsHaveProperties(t, toolName, document)
		}
		if document["type"] == "array" {
			if _, isFound := document["items"]; !isFound {
				t.Fatalf("schema for %s has array without items: %+v", toolName, document)
			}
		}
		return
	}
	values, isArray := value.([]any)
	if isArray {
		for _, item := range values {
			assertNativeSchemaMapIsProviderSafe(t, toolName, item, false)
		}
	}
}

func assertNativeRequiredFieldsHaveProperties(t *testing.T, toolName string, document map[string]any) {
	t.Helper()
	properties, isProperties := document["properties"].(map[string]any)
	if !isProperties {
		return
	}
	required, isRequired := document["required"].([]any)
	if !isRequired {
		return
	}
	for _, fieldName := range required {
		fieldNameString, isString := fieldName.(string)
		if !isString {
			t.Fatalf("schema for %s has non-string required field: %+v", toolName, document)
		}
		if _, isFound := properties[fieldNameString]; !isFound {
			t.Fatalf("schema for %s requires undefined field %q: %+v", toolName, fieldNameString, document)
		}
	}
}

func nativeToolInputProperties(parameters map[string]any) (map[string]any, bool) {
	properties, isProperties := parameters["properties"].(map[string]any)
	if !isProperties {
		return nil, false
	}
	toolInput, isToolInput := properties["toolInput"].(map[string]any)
	if !isToolInput {
		return nil, false
	}
	toolInputProperties, isToolInputProperties := toolInput["properties"].(map[string]any)
	return toolInputProperties, isToolInputProperties
}

func nativeToolInputRequired(parameters map[string]any) []any {
	properties, isProperties := parameters["properties"].(map[string]any)
	if !isProperties {
		return nil
	}
	toolInput, isToolInput := properties["toolInput"].(map[string]any)
	if !isToolInput {
		return nil
	}
	required, _ := toolInput["required"].([]any)
	return required
}

func openRouterRequestToolParameters(t *testing.T, tools []any, functionName string) map[string]any {
	t.Helper()
	for _, tool := range tools {
		document := tool.(map[string]any)
		function := document["function"].(map[string]any)
		if function["name"] != functionName {
			continue
		}
		return function["parameters"].(map[string]any)
	}
	t.Fatalf("expected tool %s in request, got %+v", functionName, tools)
	return nil
}

func openRouterRequestHasTool(tools []any, functionName string) bool {
	for _, tool := range tools {
		document := tool.(map[string]any)
		function := document["function"].(map[string]any)
		if function["name"] == functionName {
			return true
		}
	}
	return false
}

type staticProvider struct {
	response   Response
	errorValue error
}

func (provider staticProvider) CompleteStructured(context.Context, StructuredRequest) (Response, error) {
	return provider.response, provider.errorValue
}

func (provider staticProvider) CompleteText(context.Context, TextRequest) (Response, error) {
	return provider.response, provider.errorValue
}
