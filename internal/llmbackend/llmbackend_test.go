package llmbackend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (transport roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestOpenRouterStructuredRequestPreservesSchema(t *testing.T) {
	seed := int64(42)
	temperature := 0.1
	requestDocument, errorValue := json.Marshal(OpenRouterBackend{}.buildStructuredRequest(StructuredRequest{
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
	}, "openrouter/model").Document)
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
	requestDocument, errorValue := json.Marshal(OpenRouterBackend{}.buildStructuredRequest(StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "reply",
			Document:           json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
	}, "openrouter/model").Document)
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
	if _, isFound := document["max_tokens"]; isFound {
		t.Fatalf("expected unset output limit to be omitted, got %+v", document)
	}
}

func TestOpenRouterBackendNormalizesFencedStructuredJSON(t *testing.T) {
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
				Body:       io.NopCloser(strings.NewReader("{\"choices\":[{\"message\":{\"content\":\"```json\\n{\\\"reply\\\":\\\"ok\\\"}\\n```\"}}]}")),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "reply",
			Document:           json.RawMessage(`{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"],"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
	})

	if errorValue != nil {
		t.Fatalf("expected structured response: %v", errorValue)
	}
	if response.Content != `{"reply":"ok"}` {
		t.Fatalf("expected fenced JSON to normalize, got %s", response.Content)
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
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"continue__task_update","arguments":"{\"taskID\":\"task-1\",\"blueclawMessage\":\"publishing\"}"}}]}}]}`)),
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
	if response.Content != `{"action":"continue","message":"publishing","toolInput":{"taskID":"task-1"},"toolName":"task_update"}` {
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
	parameters := openRouterRequestToolParameters(t, tools, "continue__task_update")
	if _, isFound := parameters["additionalProperties"]; isFound {
		t.Fatalf("expected OpenRouter native tool parameters to omit additionalProperties, got %+v", parameters)
	}
	properties := parameters["properties"].(map[string]any)
	if _, isFound := properties["taskID"]; !isFound {
		t.Fatalf("expected projected tool parameters to preserve taskID, got %+v", parameters)
	}
	if _, isFound := properties["toolInput"]; isFound {
		t.Fatalf("expected projected tool parameters to omit nested toolInput, got %+v", parameters)
	}
	if _, isFound := properties["blueclawExecutionStateUpdate"]; isFound {
		t.Fatalf("expected per-tool continue schema to omit blueclawExecutionStateUpdate, got %+v", parameters)
	}
	if _, isFound := properties["blueclawNextStepPlan"]; isFound {
		t.Fatalf("expected per-tool continue schema to omit blueclawNextStepPlan, got %+v", parameters)
	}
	required := parameters["required"].([]any)
	if !requiredContains(required, "taskID") {
		t.Fatalf("expected required fields to include taskID, got %+v", parameters)
	}
	if requiredContains(required, "blueclawMessage") {
		t.Fatalf("expected optional blueclawMessage field to be removed from required, got %+v", parameters)
	}
	if receivedDocument["seed"] != float64(seed) {
		t.Fatalf("expected seed to be forwarded, got %+v", receivedDocument)
	}
	if receivedDocument["temperature"] != temperature {
		t.Fatalf("expected temperature to be forwarded, got %+v", receivedDocument)
	}
}

func TestOpenRouterBackendSendsGatewaySecretHeader(t *testing.T) {
	secretDirectory := t.TempDir()
	apiKeyPath := filepath.Join(secretDirectory, "openrouter-api-key")
	gatewaySecretPath := filepath.Join(secretDirectory, "gateway-secret")
	if errorValue := os.WriteFile(apiKeyPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(gatewaySecretPath, []byte("gateway-secret"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	receivedGatewaySecret := ""
	backend := OpenRouterBackend{
		KeyPath:             apiKeyPath,
		GatewaySecretPath:   gatewaySecretPath,
		GatewaySecretHeader: "X-INTERNKIM-GATEWAY-SECRET",
		BaseURL:             "https://openrouter.test/api/v1/chat/completions",
		ModelName:           "configured-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			receivedGatewaySecret = request.Header.Get("X-INTERNKIM-GATEWAY-SECRET")
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"ok"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := backend.CompleteText(context.Background(), TextRequest{Messages: []Message{{Role: "user", Content: "hello"}}})

	if errorValue != nil {
		t.Fatalf("expected text response: %v", errorValue)
	}
	if receivedGatewaySecret != "gateway-secret" {
		t.Fatalf("expected gateway secret header, got %q", receivedGatewaySecret)
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
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"event_add","arguments":"{\"title\":\"휴가\",\"startISO\":\"2026-05-09T09:00:00+09:00\",\"endISO\":\"2026-05-09T18:00:00+09:00\"}"}}]}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "내일 휴가 등록해줘"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "bluecollar_agent_turn_action",
			Document: testActionSchemaForDescriptors(t, capabilities.CalendarDescriptors()),
		},
	})

	if errorValue != nil {
		t.Fatalf("expected provider-returned tool name to resolve: %v", errorValue)
	}
	if !strings.Contains(response.Content, `"toolName":"event_add"`) {
		t.Fatalf("expected calendar tool action, got %s", response.Content)
	}
}

func TestOpenRouterBackendFallsBackToJSONSchemaWhenActionToolCallIsMissing(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	requestDocuments := []map[string]any{}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.ai/api/v1/chat/completions",
		ModelName: "configured-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var requestDocument map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			requestDocuments = append(requestDocuments, requestDocument)
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

	if errorValue != nil {
		t.Fatalf("expected JSON schema fallback response, got response=%+v error=%v", response, errorValue)
	}
	if response.ConstraintMode != ConstraintModeOpenAIJSONSchema {
		t.Fatalf("expected JSON schema fallback mode, got %q", response.ConstraintMode)
	}
	if len(requestDocuments) != 2 {
		t.Fatalf("expected native request then JSON schema fallback, got %d requests", len(requestDocuments))
	}
	if _, isFound := requestDocuments[0]["tools"]; !isFound {
		t.Fatalf("expected first request to use native tools, got %+v", requestDocuments[0])
	}
	if _, isFound := requestDocuments[1]["response_format"]; !isFound {
		t.Fatalf("expected second request to use JSON schema, got %+v", requestDocuments[1])
	}
	if strings.TrimSpace(response.Content) == "" {
		t.Fatal("expected fallback response content")
	}
}

func TestOpenRouterBackendFallsBackToJSONSchemaWhenProviderRejectsNativeActionTools(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	requestDocuments := []map[string]any{}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.ai/api/v1/chat/completions",
		ModelName: "configured-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var requestDocument map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			requestDocuments = append(requestDocuments, requestDocument)
			if len(requestDocuments) == 1 {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Provider returned error (400)"}}`)),
					Header:     make(http.Header),
				}, nil
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"action\":\"finish\",\"message\":\"done\",\"goalStatus\":\"satisfied\",\"goalSatisfied\":true,\"completionEvidence\":[],\"qualityReview\":[]}"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages:               []Message{{Role: "user", Content: "finish"}},
		StructuredOutputSchema: testAgentActionSchema(),
	})

	if errorValue != nil {
		t.Fatalf("expected fallback after provider 400: %v", errorValue)
	}
	if response.ConstraintMode != ConstraintModeOpenAIJSONSchema {
		t.Fatalf("expected JSON schema fallback mode, got %q", response.ConstraintMode)
	}
	if len(requestDocuments) != 2 {
		t.Fatalf("expected native request then JSON schema fallback, got %d requests", len(requestDocuments))
	}
	if _, isFound := requestDocuments[0]["tools"]; !isFound {
		t.Fatalf("expected first request to use native tools, got %+v", requestDocuments[0])
	}
	if _, isFound := requestDocuments[1]["response_format"]; !isFound {
		t.Fatalf("expected second request to use JSON schema, got %+v", requestDocuments[1])
	}
}

func TestOpenRouterBackendRetriesNativeActionWithFallbackModel(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	requestDocuments := []map[string]any{}
	backend := OpenRouterBackend{
		KeyPath:            secretPath,
		BaseURL:            "https://openrouter.ai/api/v1/chat/completions",
		ModelName:          "primary-model",
		FallbackModelNames: []string{"fallback-model"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var requestDocument map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			requestDocuments = append(requestDocuments, requestDocument)
			if requestDocument["model"] == "primary-model" {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Provider returned error (400)"}}`)),
					Header:     make(http.Header),
				}, nil
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
		t.Fatalf("expected fallback model native action response: %v", errorValue)
	}
	if response.Model != "fallback-model" {
		t.Fatalf("expected fallback model response, got %q", response.Model)
	}
	if response.ConstraintMode != ConstraintModeNativeToolCall {
		t.Fatalf("expected native tool call on fallback model, got %q", response.ConstraintMode)
	}
	if len(requestDocuments) != 2 {
		t.Fatalf("expected two native model attempts, got %d requests", len(requestDocuments))
	}
	if _, isFound := requestDocuments[1]["tools"]; !isFound {
		t.Fatalf("expected second request to remain native tool-call, got %+v", requestDocuments[1])
	}
}

func TestOpenRouterBackendFallsBackToPromptedJSONAfterEmptyStructuredContent(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	requestDocuments := []map[string]any{}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.ai/api/v1/chat/completions",
		ModelName: "single-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var requestDocument map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			requestDocuments = append(requestDocuments, requestDocument)
			switch len(requestDocuments) {
			case 1:
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":""}}]}`)),
					Header:     make(http.Header),
				}, nil
			case 2:
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":""}}]}`)),
					Header:     make(http.Header),
				}, nil
			default:
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"action\":\"finish\",\"message\":\"done\",\"goalStatus\":\"satisfied\",\"goalSatisfied\":true,\"completionEvidence\":[],\"qualityReview\":[]}"}}]}`)),
					Header:     make(http.Header),
				}, nil
			}
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages:               []Message{{Role: "user", Content: "finish"}},
		StructuredOutputSchema: testAgentActionSchema(),
	})

	if errorValue != nil {
		t.Fatalf("expected prompted JSON fallback response: %v", errorValue)
	}
	if response.Model != "single-model" {
		t.Fatalf("expected same model response, got %q", response.Model)
	}
	if response.ConstraintMode != ConstraintModePromptedJSON {
		t.Fatalf("expected prompted JSON mode, got %q", response.ConstraintMode)
	}
	if len(requestDocuments) != 3 {
		t.Fatalf("expected native, JSON schema, and prompted JSON requests, got %d", len(requestDocuments))
	}
	if _, isFound := requestDocuments[2]["response_format"]; isFound {
		t.Fatalf("expected prompted JSON request to omit response_format, got %+v", requestDocuments[2])
	}
}

func TestOpenRouterBackendUsesPromptedJSONFirstForFreeModel(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var requestDocument map[string]any
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.ai/api/v1/chat/completions",
		ModelName: "google/gemma-4-31b-it:free",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"action\":\"finish\",\"message\":\"done\",\"goalStatus\":\"satisfied\",\"goalSatisfied\":true,\"completionEvidence\":[],\"qualityReview\":[]}"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages:               []Message{{Role: "user", Content: "finish"}},
		StructuredOutputSchema: testAgentActionSchema(),
	})

	if errorValue != nil {
		t.Fatalf("expected prompted JSON response: %v", errorValue)
	}
	if response.ConstraintMode != ConstraintModePromptedJSON {
		t.Fatalf("expected prompted JSON mode, got %q", response.ConstraintMode)
	}
	if _, isFound := requestDocument["tools"]; isFound {
		t.Fatalf("expected free model request to omit native tools, got %+v", requestDocument)
	}
	if _, isFound := requestDocument["response_format"]; isFound {
		t.Fatalf("expected free model request to omit response_format, got %+v", requestDocument)
	}
}

func TestOpenAICompatibleActionToolRequestUsesGenerationOptions(t *testing.T) {
	seed := int64(12)
	temperature := 0.6
	maxTokens := 2048
	request := openAIActionToolRequest("local-model", []Message{{Role: "user", Content: "publish"}}, []nativeActionTool{{
		FunctionName: "continue__task_update",
		Description:  "Call task_update",
		Action:       "continue",
		ToolName:     "task_update",
		Parameters:   json.RawMessage(`{"type":"object","properties":{}}`),
	}}, GenerationOptions{Seed: &seed, Temperature: &temperature, MaxTokens: &maxTokens})

	if request.Seed == nil || *request.Seed != seed {
		t.Fatalf("expected seed on OpenAI-compatible request, got %+v", request)
	}
	if request.Temperature == nil || *request.Temperature != temperature {
		t.Fatalf("expected temperature on OpenAI-compatible request, got %+v", request)
	}
	if request.MaxTokens == nil || *request.MaxTokens != maxTokens {
		t.Fatalf("expected max tokens on OpenAI-compatible request, got %+v", request)
	}
	if len(request.Tools) != 1 || request.Tools[0].Function.Name != "continue__task_update" {
		t.Fatalf("expected native tool call shape to remain, got %+v", request.Tools)
	}
	if string(request.ToolChoice) != `"required"` {
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

func TestOpenRouterChatActionRequestUsesDocumentedImageInputShape(t *testing.T) {
	requestDocument, _, errorValue := OpenRouterBackend{}.buildChatActionRequest(StructuredRequest{
		Messages: []Message{{
			Role:    "user",
			Content: "inspect this",
			Parts: []MessagePart{{
				Type:       "image",
				MimeType:   "image/png",
				DataBase64: "aW1hZ2U=",
			}},
		}},
	}, "openrouter/model", []nativeActionTool{{
		FunctionName: "finish",
		Parameters:   json.RawMessage(`{"type":"object","properties":{}}`),
	}})
	if errorValue != nil {
		t.Fatalf("expected request document: %v", errorValue)
	}
	var request struct {
		Messages []struct {
			Role    string           `json:"role"`
			Content []map[string]any `json:"content"`
		} `json:"messages"`
	}
	if errorValue := json.Unmarshal(requestDocument, &request); errorValue != nil {
		t.Fatalf("expected json request: %v", errorValue)
	}
	if len(request.Messages) != 1 || request.Messages[0].Role != "user" {
		t.Fatalf("expected user message for image input, got %s", string(requestDocument))
	}
	content := request.Messages[0].Content
	if len(content) != 2 || content[0]["type"] != "text" || content[1]["type"] != "image_url" {
		t.Fatalf("expected text first, then image_url content, got %s", string(requestDocument))
	}
	imageURL, ok := content[1]["image_url"].(map[string]any)
	if !ok || imageURL["url"] != "data:image/png;base64,aW1hZ2U=" {
		t.Fatalf("expected OpenRouter data image URL, got %s", string(requestDocument))
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
		Name: "bluecollar_agent_turn_action",
		Document: json.RawMessage(`{"oneOf":[
			{"type":"object","properties":{"action":{"type":"string","enum":["continue"]},"toolName":{"type":"string","enum":["a.b"]},"toolInput":{"type":"object"}},"required":["action","toolName","toolInput"]},
			{"type":"object","properties":{"action":{"type":"string","enum":["continue"]},"toolName":{"type":"string","enum":["a/b"]},"toolInput":{"type":"object"}},"required":["action","toolName","toolInput"]}
		]}`),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "maps multiple actions") {
		t.Fatalf("expected function name collision error, got %v", errorValue)
	}
}

func TestNativeActionToolsPreserveFlattenedToolInputOptionalityForProviderCompatibility(t *testing.T) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name:     "bluecollar_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, capabilities.CalendarDescriptors()),
	})
	if errorValue != nil {
		t.Fatalf("expected native tool set: %v", errorValue)
	}
	if !isActionSchema {
		t.Fatal("expected action schema")
	}

	calendarAddTool := toolSet.ToolByName[nativeActionFunctionName("continue", "event_add")]
	var parameters map[string]any
	if errorValue := json.Unmarshal(calendarAddTool.Parameters, &parameters); errorValue != nil {
		t.Fatalf("expected calendar parameters: %v", errorValue)
	}
	properties, _ := parameters["properties"].(map[string]any)
	for _, fieldName := range []string{"title", "startsAt", "endsAt"} {
		if _, isFound := properties[fieldName]; !isFound {
			t.Fatalf("expected property %q in calendar schema: %+v", fieldName, parameters)
		}
	}
	if _, isFound := properties["toolInput"]; isFound {
		t.Fatalf("expected flattened calendar schema to omit toolInput: %+v", parameters)
	}
	required := parameters["required"].([]any)
	for _, fieldName := range []string{"title", "startsAt", "endsAt"} {
		if requiredContains(required, fieldName) {
			t.Fatalf("expected optional flattened toolInput field %s to be removed from required, got %+v", fieldName, parameters)
		}
	}
	assertNativeRequiredFieldsHaveProperties(t, "event_add", parameters)
}

func TestNativeActionToolsProjectEveryDefaultCapabilitySchema(t *testing.T) {
	descriptors := capabilities.DefaultToolDescriptors()
	for _, descriptor := range descriptors {
		toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
			Name:     "bluecollar_agent_turn_action",
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

func TestNativeActionToolsKeepLargeToolSetsPerToolWithoutDispatcher(t *testing.T) {
	const toolCount = 30
	descriptors := make([]capabilities.Descriptor, 0, toolCount)
	for index := 0; index < toolCount; index++ {
		descriptors = append(descriptors, capabilities.Descriptor{
			Name:        fmt.Sprintf("tool.%02d", index),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"]}`),
		})
	}
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name:     "bluecollar_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, descriptors),
	})
	if errorValue != nil {
		t.Fatalf("expected native tool set: %v", errorValue)
	}
	if !isActionSchema {
		t.Fatal("expected action schema")
	}
	if _, isDispatcher := toolSet.ToolByName["continue"]; isDispatcher {
		t.Fatalf("large tool sets must keep per-tool schemas instead of collapsing to a dispatcher, got %+v", toolSet.Tools)
	}
	if len(toolSet.Tools) != toolCount {
		t.Fatalf("expected %d per-tool functions, got %d", toolCount, len(toolSet.Tools))
	}
}

func TestNativeActionToolsKeepRepresentativeWorkingSetPerTool(t *testing.T) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name:     "bluecollar_agent_turn_action",
		Document: testActionSchemaWithControlActionsAndToolCount(t, 13),
	})
	if errorValue != nil {
		t.Fatalf("expected native tool set: %v", errorValue)
	}
	if !isActionSchema {
		t.Fatal("expected action schema")
	}
	if _, isDispatcher := toolSet.ToolByName["continue"]; isDispatcher {
		t.Fatalf("representative working set must keep per-tool schemas so tool arguments survive instead of collapsing to the argument-less dispatcher, got %+v", toolSet.Tools)
	}
	if len(toolSet.Tools) < 13 {
		t.Fatalf("expected one function per tool, got %d", len(toolSet.Tools))
	}
}

func TestNativeActionToolUsesPortableInputSchemaWithoutProjection(t *testing.T) {
	toolSet, isActionSchema, errorValue := nativeActionToolsForSchema(StructuredOutputSchema{
		Name: "bluecollar_agent_turn_action",
		Document: testActionSchemaForDescriptors(t, []capabilities.Descriptor{{
			Name:        "write",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`),
		}}),
	})
	if errorValue != nil {
		t.Fatalf("expected native tool set: %v", errorValue)
	}
	if !isActionSchema {
		t.Fatal("expected action schema")
	}
	tool := toolSet.ToolByName[nativeActionFunctionName("continue", "write")]
	var parameters map[string]any
	if errorValue := json.Unmarshal(tool.Parameters, &parameters); errorValue != nil {
		t.Fatalf("expected parameters json: %v", errorValue)
	}
	properties, isFound := parameters["properties"].(map[string]any)
	if !isFound {
		t.Fatalf("expected object properties, got %s", tool.Parameters)
	}
	if _, isFound := properties["path"]; !isFound {
		t.Fatalf("expected path property to survive projection, got %+v", properties)
	}
	if _, isFound := properties["toolInput"]; isFound {
		t.Fatalf("expected flattened schema to omit toolInput, got %+v", properties)
	}
	required := parameters["required"].([]any)
	for _, fieldName := range []string{"path", "content"} {
		if requiredContains(required, fieldName) {
			t.Fatalf("expected portable optional flattened field %s to be removed from required, got %+v in %s", fieldName, required, tool.Parameters)
		}
	}
	assertNativeSchemaIsProviderSafe(t, "write", tool.Parameters)
}

func TestOpenRouterBackendResolvesDefaultModel(t *testing.T) {
	backend := OpenRouterBackend{ModelName: "google/default-remote"}
	for _, modelName := range []string{"", "default", "DEFAULT", "local/anything"} {
		if resolvedModelName := backend.resolveModelName(modelName); resolvedModelName != "google/default-remote" {
			t.Fatalf("expected default remote model for %q, got %q", modelName, resolvedModelName)
		}
	}
}

func TestCompactMessagesForContextWindowPreservesSystemAndRecentCollapsingMiddle(t *testing.T) {
	hugeMiddleMessage := strings.Repeat("x", int(testContextWindowTokens)*4*2)
	messages := []Message{
		{Role: "system", Content: "you are a helpful assistant"},
		{Role: "user", Content: hugeMiddleMessage},
		{Role: "assistant", Content: hugeMiddleMessage},
		{Role: "user", Content: "recent turn one"},
		{Role: "assistant", Content: "recent turn two"},
		{Role: "user", Content: "recent turn three"},
		{Role: "assistant", Content: "recent turn four"},
		{Role: "user", Content: "recent turn five"},
		{Role: "assistant", Content: "most recent turn"},
	}

	compacted, errorValue := compactMessagesForContextWindow(messages, testContextWindowTokens)
	if errorValue != nil {
		t.Fatalf("expected compaction to succeed rather than error out: %v", errorValue)
	}
	if estimatedMessagesTokens(compacted) > inputTokenBudget(testContextWindowTokens) {
		t.Fatalf("expected compacted messages to fit the context window budget, got %d estimated tokens", estimatedMessagesTokens(compacted))
	}
	if compacted[0].Role != "system" || compacted[0].Content != "you are a helpful assistant" {
		t.Fatalf("expected leading system message to be preserved verbatim, got %+v", compacted[0])
	}
	lastSix := compacted[len(compacted)-6:]
	expectedRecent := []string{"recent turn one", "recent turn two", "recent turn three", "recent turn four", "recent turn five", "most recent turn"}
	for index, message := range lastSix {
		if message.Content != expectedRecent[index] {
			t.Fatalf("expected recent messages preserved verbatim in order, got %+v", lastSix)
		}
	}
	for _, message := range compacted {
		if strings.Contains(message.Content, hugeMiddleMessage) {
			t.Fatal("expected huge middle content to be collapsed rather than preserved")
		}
	}
}

func TestCompactMessagesForContextWindowLeavesLargeWindowUntouched(t *testing.T) {
	largeMessages := []Message{
		{Role: "system", Content: "you are a helpful assistant"},
		{Role: "user", Content: strings.Repeat("y", int(testContextWindowTokens)*4*2)},
	}

	compacted, errorValue := compactMessagesForContextWindow(largeMessages, DefaultContextWindowTokens)
	if errorValue != nil {
		t.Fatalf("expected default large window to accommodate the same message list untouched: %v", errorValue)
	}
	if len(compacted) != len(largeMessages) || compacted[1].Content != largeMessages[1].Content {
		t.Fatalf("expected messages to pass through unchanged under the default large context window, got %+v", compacted)
	}
}

func TestCompactMessagesForContextWindowFailsWhenNoMiddleContentCanBeCollapsed(t *testing.T) {
	singleHugeMessage := []Message{
		{Role: "user", Content: strings.Repeat("z", int(testContextWindowTokens)*4*2)},
	}

	_, errorValue := compactMessagesForContextWindow(singleHugeMessage, testContextWindowTokens)
	if errorValue == nil {
		t.Fatal("expected a single oversized message with nothing to collapse to fail with a clear error")
	}
}

func TestAutoProviderCompactsMessagesGenericallyBeforeDispatch(t *testing.T) {
	hugeMiddleMessage := strings.Repeat("x", int(testContextWindowTokens)*4*2)
	oversizedMessages := []Message{
		{Role: "system", Content: "you are a helpful assistant"},
		{Role: "user", Content: hugeMiddleMessage},
		{Role: "user", Content: "recent turn one"},
		{Role: "assistant", Content: "recent turn two"},
		{Role: "user", Content: "recent turn three"},
		{Role: "assistant", Content: "recent turn four"},
		{Role: "user", Content: "recent turn five"},
		{Role: "assistant", Content: "most recent turn"},
	}

	smallWindowRecorder := &recordingContextWindowProvider{contextWindowTokens: testContextWindowTokens}
	auto := AutoProvider{Providers: []Provider{smallWindowRecorder}}

	if _, errorValue := auto.CompleteText(context.Background(), TextRequest{Messages: oversizedMessages}); errorValue != nil {
		t.Fatalf("expected AutoProvider to compact and dispatch successfully: %v", errorValue)
	}
	if estimatedMessagesTokens(smallWindowRecorder.receivedMessages) >= estimatedMessagesTokens(oversizedMessages) {
		t.Fatal("expected AutoProvider to compact the oversized message list before dispatch, generically, not pass it through unchanged")
	}
	for _, message := range smallWindowRecorder.receivedMessages {
		if strings.Contains(message.Content, hugeMiddleMessage) {
			t.Fatal("expected the huge message to be collapsed by AutoProvider before reaching the candidate")
		}
	}

	defaultWindowRecorder := &recordingContextWindowProvider{contextWindowTokens: 0}
	autoWithDefaultWindow := AutoProvider{Providers: []Provider{defaultWindowRecorder}}
	if _, errorValue := autoWithDefaultWindow.CompleteText(context.Background(), TextRequest{Messages: oversizedMessages}); errorValue != nil {
		t.Fatalf("expected default-window candidate to receive the prompt untouched: %v", errorValue)
	}
	if len(defaultWindowRecorder.receivedMessages) != len(oversizedMessages) {
		t.Fatalf("expected default large context window to leave messages untouched, got %d of %d messages", len(defaultWindowRecorder.receivedMessages), len(oversizedMessages))
	}
}

type recordingContextWindowProvider struct {
	contextWindowTokens int64
	receivedMessages    []Message
}

func (provider *recordingContextWindowProvider) ContextWindowTokens() int64 {
	return provider.contextWindowTokens
}

func (provider *recordingContextWindowProvider) CompleteStructured(_ context.Context, request StructuredRequest) (Response, error) {
	provider.receivedMessages = request.Messages
	return Response{}, nil
}

func (provider *recordingContextWindowProvider) CompleteText(_ context.Context, request TextRequest) (Response, error) {
	provider.receivedMessages = request.Messages
	return Response{}, nil
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

func TestAutoProviderHasNoDefaultAttemptDeadline(t *testing.T) {
	auto := AutoProvider{Providers: []Provider{deadlineRejectingProvider{}}}
	if _, errorValue := auto.CompleteText(context.Background(), TextRequest{}); errorValue != nil {
		t.Fatalf("expected provider call without an arbitrary deadline: %v", errorValue)
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
			Name:     "bluecollar_agent_turn_action",
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
		"schemaName=bluecollar_agent_turn_action",
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

func testAgentActionSchema() StructuredOutputSchema {
	return StructuredOutputSchema{
		Name: "bluecollar_agent_turn_action",
		Document: json.RawMessage(`{"oneOf":[
			{"type":"object","properties":{"action":{"type":"string","enum":["finish"]},"message":{"type":"string"},"goalStatus":{"type":"string","enum":["satisfied"]},"goalSatisfied":{"type":"boolean"},"completionEvidence":{"type":"array"},"qualityReview":{"type":"array"},"executionStateUpdate":{"type":"object"}},"required":["action","message","goalStatus","goalSatisfied","completionEvidence","qualityReview","executionStateUpdate"]},
			{"type":"object","properties":{"action":{"type":"string","enum":["continue"]},"toolName":{"type":"string","enum":["task_update"]},"toolInput":{"type":"object","properties":{"taskID":{"type":"string"}},"required":["taskID"]},"message":{"type":"string"},"executionStateUpdate":{"type":"object"},"nextStepPlan":{"type":"object","properties":{"objective":{"type":"string"},"expectedTools":{"type":"array","items":{"type":"string"}},"doneCriteria":{"type":"array","items":{"type":"string"}},"risk":{"type":"string"},"workingSetReason":{"type":"string"}},"required":["objective","expectedTools","doneCriteria","risk","workingSetReason"]}},"required":["action","toolName","toolInput","executionStateUpdate","nextStepPlan"]}
		]}`),
		IsStrictlyEnforced: true,
	}
}

func testActionSchemaForDescriptors(t *testing.T, descriptors []capabilities.Descriptor) json.RawMessage {
	t.Helper()
	variants := make([]any, 0, len(descriptors))
	for _, descriptor := range descriptors {
		inputSchema := descriptor.InputSchema
		if len(inputSchema) == 0 {
			inputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		var toolInput any
		if errorValue := json.Unmarshal(inputSchema, &toolInput); errorValue != nil {
			t.Fatalf("input schema for %s is invalid: %v", descriptor.Name, errorValue)
		}
		toolInput = testNativePortableNestedSchema(toolInput)
		variants = append(variants, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action":    map[string]any{"type": "string", "enum": []string{"continue"}},
				"toolName":  map[string]any{"type": "string", "enum": []string{descriptor.Name}},
				"toolInput": toolInput,
				"executionStateUpdate": map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
				"nextStepPlan": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"objective":        map[string]any{"type": "string"},
						"expectedTools":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"doneCriteria":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"risk":             map[string]any{"type": "string"},
						"workingSetReason": map[string]any{"type": "string"},
					},
					"required": []string{"objective", "expectedTools", "doneCriteria", "risk", "workingSetReason"},
				},
			},
			"required": []string{"action", "toolName", "toolInput", "executionStateUpdate", "nextStepPlan"},
		})
	}
	document, errorValue := json.Marshal(map[string]any{"oneOf": variants})
	if errorValue != nil {
		t.Fatalf("expected action schema: %v", errorValue)
	}
	return document
}

func testNativePortableNestedSchema(value any) any {
	document, isObject := value.(map[string]any)
	if isObject {
		clone := map[string]any{}
		for fieldName, fieldValue := range document {
			if fieldName == "required" {
				continue
			}
			if fieldName == "type" && fieldValue == "integer" {
				clone[fieldName] = "number"
				continue
			}
			clone[fieldName] = testNativePortableNestedSchema(fieldValue)
		}
		if clone["type"] == "object" {
			if _, isFound := clone["properties"]; !isFound {
				clone["properties"] = map[string]any{}
			}
		}
		return clone
	}
	values, isArray := value.([]any)
	if isArray {
		clone := make([]any, 0, len(values))
		for _, item := range values {
			clone = append(clone, testNativePortableNestedSchema(item))
		}
		return clone
	}
	return value
}

func testActionSchemaWithControlActionsAndToolCount(t *testing.T, toolCount int) json.RawMessage {
	t.Helper()
	variants := []any{
		testControlActionVariant("finish"),
		testControlActionVariant("fail"),
		testControlActionVariant("require_capabilities"),
		testControlActionVariant("set_quality_criteria"),
	}
	descriptors := make([]capabilities.Descriptor, 0, toolCount)
	for index := 0; index < toolCount; index++ {
		descriptors = append(descriptors, capabilities.Descriptor{
			Name:        fmt.Sprintf("tool.%02d", index),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}}}`),
		})
	}
	var document actionSchemaDocument
	if errorValue := json.Unmarshal(testActionSchemaForDescriptors(t, descriptors), &document); errorValue != nil {
		t.Fatalf("expected action schema: %v", errorValue)
	}
	for _, variant := range document.OneOf {
		variants = append(variants, variant)
	}
	content, errorValue := json.Marshal(map[string]any{"oneOf": variants})
	if errorValue != nil {
		t.Fatalf("expected action schema: %v", errorValue)
	}
	return content
}

func testControlActionVariant(action string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{"type": "string", "enum": []string{action}},
		},
		"required": []string{"action"},
	}
}

func assertNativeSchemaIsProviderSafe(t *testing.T, toolName string, schema json.RawMessage) {
	t.Helper()
	_, lintResult := NormalizeNativeSchema(schema)
	if len(lintResult.NormalizationsApplied) > 0 {
		t.Fatalf("schema for %s required provider portability normalization: %+v", toolName, lintResult.NormalizationsApplied)
	}
	if len(lintResult.RemainingViolations) > 0 {
		t.Fatalf("schema for %s is not provider safe: %+v", toolName, lintResult.RemainingViolations)
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

func requiredContains(required any, expected string) bool {
	switch values := required.(type) {
	case []any:
		for _, fieldName := range values {
			if fieldName == expected {
				return true
			}
		}
	case []string:
		for _, fieldName := range values {
			if fieldName == expected {
				return true
			}
		}
	}
	return false
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

type deadlineRejectingProvider struct{}

func (deadlineRejectingProvider) CompleteStructured(ctx context.Context, request StructuredRequest) (Response, error) {
	_ = request
	return completeWithoutDeadline(ctx)
}

func (deadlineRejectingProvider) CompleteText(ctx context.Context, request TextRequest) (Response, error) {
	_ = request
	return completeWithoutDeadline(ctx)
}

func completeWithoutDeadline(ctx context.Context) (Response, error) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return Response{}, errors.New("provider request received an arbitrary deadline")
	}
	return Response{Content: "ok"}, nil
}

func (provider staticProvider) CompleteStructured(context.Context, StructuredRequest) (Response, error) {
	return provider.response, provider.errorValue
}

func (provider staticProvider) CompleteText(context.Context, TextRequest) (Response, error) {
	return provider.response, provider.errorValue
}

type namedStaticProvider struct {
	name       string
	errorValue error
}

func (provider namedStaticProvider) Name() string {
	return provider.name
}

func (provider namedStaticProvider) CompleteStructured(context.Context, StructuredRequest) (Response, error) {
	return Response{}, provider.errorValue
}

func (provider namedStaticProvider) CompleteText(context.Context, TextRequest) (Response, error) {
	return Response{}, provider.errorValue
}

func TestOpenRouterBackendNormalizesHTTPErrorJSON(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	rawBody := `{"error":{"message":"Schema rejected by provider","code":"bad_request","metadata":{"token":"sk-provider-secret","callbackURL":"https://internal.test/provider/debug"}}}`
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "test-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader(rawBody)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := backend.CompleteText(context.Background(), TextRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if errorValue == nil {
		t.Fatal("expected normalized provider error")
	}
	errorMessage := errorValue.Error()
	if !strings.Contains(errorMessage, "openrouter: HTTP 400: Schema rejected by provider (bad_request)") {
		t.Fatalf("expected compact provider status and message, got %v", errorValue)
	}
	if strings.Contains(errorMessage, rawBody) || strings.Contains(errorMessage, "metadata") || strings.Contains(errorMessage, "sk-provider-secret") || strings.Contains(errorMessage, "internal.test") {
		t.Fatalf("expected raw provider body to be omitted, got %v", errorValue)
	}
}

func TestOpenRouterBackendNormalizesHTTPErrorNonJSON(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	rawBody := "upstream failed with token sk-provider-secret at https://internal.test/provider/debug"
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "test-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader(rawBody)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	_, errorValue := backend.CompleteText(context.Background(), TextRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if errorValue == nil {
		t.Fatal("expected normalized provider error")
	}
	errorMessage := errorValue.Error()
	if !strings.Contains(errorMessage, "openrouter: HTTP 502:") || !strings.Contains(errorMessage, "non-JSON response") {
		t.Fatalf("expected status-coded non-JSON fallback, got %v", errorValue)
	}
	if strings.Contains(errorMessage, rawBody) || strings.Contains(errorMessage, "sk-provider-secret") || strings.Contains(errorMessage, "internal.test") {
		t.Fatalf("expected non-JSON provider body to be omitted, got %v", errorValue)
	}
}

func TestAutoProviderAggregatesNormalizedProviderErrors(t *testing.T) {
	firstRawBody := []byte(`{"error":{"message":"First provider rejected schema","code":"invalid_schema","metadata":{"token":"sk-first-secret"}}}`)
	secondRawBody := []byte(`{"error":{"message":"Second provider rejected schema","code":"bad_request","metadata":{"url":"https://internal.test/second"}}}`)
	auto := AutoProvider{
		Providers: []Provider{
			namedStaticProvider{name: "openrouter", errorValue: normalizeProviderError("openrouter", http.StatusBadRequest, firstRawBody)},
			namedStaticProvider{name: "llamacpp", errorValue: normalizeProviderError("llamacpp", http.StatusBadRequest, secondRawBody)},
		},
	}

	_, errorValue := auto.CompleteText(context.Background(), TextRequest{})
	if errorValue == nil {
		t.Fatal("expected provider chain failure")
	}
	errorMessage := errorValue.Error()
	if !strings.Contains(errorMessage, "openrouter: HTTP 400: First provider rejected schema (invalid_schema)") {
		t.Fatalf("expected first compact provider error, got %v", errorValue)
	}
	if !strings.Contains(errorMessage, "llamacpp: HTTP 400: Second provider rejected schema (bad_request)") {
		t.Fatalf("expected second compact provider error, got %v", errorValue)
	}
	if strings.Contains(errorMessage, string(firstRawBody)) || strings.Contains(errorMessage, string(secondRawBody)) || strings.Contains(errorMessage, "metadata") || strings.Contains(errorMessage, "sk-first-secret") || strings.Contains(errorMessage, "internal.test") {
		t.Fatalf("expected aggregate error to omit raw provider bodies, got %v", errorValue)
	}
}

func TestOpenRouterBackendPopulatesUsageFromStructuredResponse(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "test-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{\"reply\":\"ok\"}"}}],"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected structured response: %v", errorValue)
	}
	if response.Usage.PromptTokens != 10 {
		t.Fatalf("expected 10 prompt tokens, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 5 {
		t.Fatalf("expected 5 completion tokens, got %d", response.Usage.CompletionTokens)
	}
	if response.Usage.TotalTokens != 15 {
		t.Fatalf("expected 15 total tokens, got %d", response.Usage.TotalTokens)
	}
}

func TestOpenRouterBackendNormalizesUsageWhenTotalTokensIsZero(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "test-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"plain reply"}}],"usage":{"prompt_tokens":8,"completion_tokens":3,"total_tokens":0}}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteText(context.Background(), TextRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if errorValue != nil {
		t.Fatalf("expected text response: %v", errorValue)
	}
	if response.Usage.TotalTokens != 11 {
		t.Fatalf("expected total tokens normalized to 11, got %d", response.Usage.TotalTokens)
	}
	if response.Usage.PromptTokens != 8 {
		t.Fatalf("expected 8 prompt tokens, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 3 {
		t.Fatalf("expected 3 completion tokens, got %d", response.Usage.CompletionTokens)
	}
}

func TestOpenRouterBackendPopulatesUsageFromNativeActionResponse(t *testing.T) {
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
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"call-1","type":"function","function":{"name":"continue__task_update","arguments":"{\"taskID\":\"task-1\",\"blueclawMessage\":\"publishing\",\"blueclawExecutionStateUpdate\":{},\"blueclawNextStepPlan\":{\"objective\":\"confirm publish\",\"expectedTools\":[],\"doneCriteria\":[\"published\"],\"risk\":\"none\",\"workingSetReason\":\"publish result completes the task\"}}"}}]}}],"usage":{"prompt_tokens":20,"completion_tokens":8,"total_tokens":28}}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages:               []Message{{Role: "user", Content: "publish"}},
		StructuredOutputSchema: testAgentActionSchema(),
	})
	if errorValue != nil {
		t.Fatalf("expected native action response: %v", errorValue)
	}
	if response.Usage.PromptTokens != 20 {
		t.Fatalf("expected 20 prompt tokens, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 8 {
		t.Fatalf("expected 8 completion tokens, got %d", response.Usage.CompletionTokens)
	}
	if response.Usage.TotalTokens != 28 {
		t.Fatalf("expected 28 total tokens, got %d", response.Usage.TotalTokens)
	}
}

func TestOpenRouterBackendReturnsZeroUsageWhenUsageBlockIsAbsent(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.test/api/v1/chat/completions",
		ModelName: "test-model",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"plain reply"}}]}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteText(context.Background(), TextRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if errorValue != nil {
		t.Fatalf("expected text response: %v", errorValue)
	}
	if response.Usage.PromptTokens != 0 || response.Usage.CompletionTokens != 0 || response.Usage.TotalTokens != 0 {
		t.Fatalf("expected zero usage when block is absent, got %+v", response.Usage)
	}
}

func TestCompactMessagesForContextWindowPreservesMidConversationSystemMessages(t *testing.T) {
	hugeMiddleMessage := strings.Repeat("x", int(testContextWindowTokens)*4*2)
	messages := []Message{
		{Role: "system", Content: "you are a helpful assistant"},
		{Role: "user", Content: hugeMiddleMessage},
		{Role: "system", Content: "critical mid-task directive that must survive"},
		{Role: "assistant", Content: hugeMiddleMessage},
		{Role: "user", Content: "recent turn one"},
		{Role: "assistant", Content: "recent turn two"},
		{Role: "user", Content: "recent turn three"},
		{Role: "assistant", Content: "recent turn four"},
		{Role: "user", Content: "recent turn five"},
		{Role: "assistant", Content: "most recent turn"},
	}

	compacted, errorValue := compactMessagesForContextWindow(messages, testContextWindowTokens)
	if errorValue != nil {
		t.Fatalf("expected compaction to succeed: %v", errorValue)
	}
	foundDirective := false
	for _, message := range compacted {
		if message.Role == "system" && message.Content == "critical mid-task directive that must survive" {
			foundDirective = true
		}
		if strings.Contains(message.Content, hugeMiddleMessage) {
			t.Fatal("expected huge non-system content to be collapsed")
		}
	}
	if !foundDirective {
		t.Fatalf("expected the mid-conversation system directive to be preserved verbatim, got %+v", compacted)
	}
}

func TestOpenRouterContextWindowComesFromModelCatalog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/models" {
			http.NotFound(responseWriter, request)
			return
		}
		responseWriter.Write([]byte(`{"data":[{"id":"google/gemma-3-12b-it","context_length":131072},{"id":"google/gemini-2.5-flash-lite","context_length":1048576}]}`))
	}))
	defer server.Close()

	backend := OpenRouterBackend{
		ModelName: "google/gemini-2.5-flash-lite",
		BaseURL:   server.URL + "/api/v1/chat/completions",
	}
	if tokens := backend.ContextWindowTokensForModel("google/gemma-3-12b-it"); tokens != 131072 {
		t.Fatalf("expected gemma-3-12b window from the catalog, got %d", tokens)
	}
	if tokens := backend.ContextWindowTokensForModel(""); tokens != 1048576 {
		t.Fatalf("expected empty request model to resolve through the backend default model in the catalog, got %d", tokens)
	}
	if tokens := contextWindowTokensFor(backend, "vendor/unknown-model"); tokens != DefaultContextWindowTokens {
		t.Fatalf("expected a model missing from the catalog to fall back to the default window, got %d", tokens)
	}
}

func TestOpenRouterContextWindowFallsBackWhenCatalogUnreachable(t *testing.T) {
	deadServer := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadServer.Close()

	backend := OpenRouterBackend{
		ModelName: "google/gemini-2.5-flash-lite",
		BaseURL:   deadServer.URL + "/api/v1/chat/completions",
	}
	if tokens := backend.ContextWindowTokensForModel("google/gemma-3-12b-it"); tokens != DefaultContextWindowTokens {
		t.Fatalf("expected unreachable catalog to fall back to the default window, got %d", tokens)
	}
}

// A model that answers with nothing has dropped the turn, not refused the
// schema. Giving up on it hands the turn to whatever stands next in the chain,
// which on a device is small enough to invent its answer.
func TestOpenRouterBackendRetriesOnceWhenTheModelAnswersWithNothing(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	attempts := 0
	backend := OpenRouterBackend{
		KeyPath:   secretPath,
		BaseURL:   "https://openrouter.ai/api/v1/chat/completions",
		ModelName: "z-ai/glm-5.3-flash",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			attempts++
			body := "{\"choices\":[{\"message\":{\"content\":\"\"}}]}"
			if attempts > 2 {
				body = "{\"choices\":[{\"message\":{\"content\":\"{\\\"reply\\\":\\\"ok\\\"}\"}}]}"
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		})},
	}

	response, errorValue := backend.CompleteStructured(context.Background(), StructuredRequest{
		Messages: []Message{{Role: "user", Content: "박예시 님한테 테스트로 뭐라고 dm 보내봐."}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "bluecollar_turn_router",
			Document:           json.RawMessage(`{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"],"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
	})
	if errorValue != nil {
		t.Fatalf("an empty answer should not end the attempt: %v", errorValue)
	}
	if response.Content != `{"reply":"ok"}` {
		t.Fatalf("content = %q", response.Content)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want the schema attempt, the prompted one, and one retry", attempts)
	}
}

func TestProviderChainSaysTheAnswerCameFromAFallback(t *testing.T) {
	failing := stubStructuredProvider{failure: errors.New("structured response content was empty")}
	standing := stubStructuredProvider{response: Response{Provider: "llamacpp", Model: "local/gemma", Content: "{}"}}

	response, errorValue := completeWithProviderChain(
		[]Provider{failing, standing},
		"schemaName=bluecollar_turn_router",
		func(candidate Provider) (Response, error) {
			return candidate.(stubStructuredProvider).complete()
		},
	)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !response.UsedFallback {
		t.Fatal("an answer from further down the chain must say so, or a device model passes for the tier that was asked for")
	}
	if !strings.Contains(response.FallbackReason, "content was empty") {
		t.Fatalf("fallback reason = %q", response.FallbackReason)
	}
}

type stubStructuredProvider struct {
	response Response
	failure  error
}

func (provider stubStructuredProvider) Name() string { return "stub" }

func (provider stubStructuredProvider) CompleteStructured(context.Context, StructuredRequest) (Response, error) {
	return provider.complete()
}

func (provider stubStructuredProvider) CompleteText(context.Context, TextRequest) (Response, error) {
	return provider.complete()
}

func (provider stubStructuredProvider) complete() (Response, error) {
	if provider.failure != nil {
		return Response{}, provider.failure
	}
	return provider.response, nil
}

const testContextWindowTokens int64 = 8192
