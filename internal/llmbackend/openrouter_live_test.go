package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

func TestOpenRouterLiveDocumentedToolSchemaFromEnv(t *testing.T) {
	backend, apiKey := liveOpenRouterBackendFromEnv(t)
	ctx := context.Background()

	requestDocument, errorValue := json.Marshal(map[string]any{
		"model": backend.resolveModelName(""),
		"messages": []map[string]any{{
			"role":    "user",
			"content": "What are the titles of some James Joyce books?",
		}},
		"tools": []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name":        "search_gutenberg_books",
				"description": "Search for books in the Project Gutenberg library",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"search_terms": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "List of search terms to find books",
						},
					},
					"required": []string{"search_terms"},
				},
			},
		}},
		"stream":              false,
		"tool_choice":         "auto",
		"parallel_tool_calls": false,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := assertOpenRouterRawRequestAccepted(ctx, backend, apiKey, requestDocument); errorValue != nil {
		t.Fatalf("expected documented OpenRouter tool schema to be accepted: %v", errorValue)
	}
}

func TestOpenRouterLiveAgentActionSchemaFromEnv(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	ctx := context.Background()
	request := StructuredRequest{
		Messages: []Message{{Role: "user", Content: "Call browser_open for https://example.com."}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "blueclaw_agent_turn_action",
			Document:           testActionSchemaForDescriptors(t, []capabilities.Descriptor{findLiveDescriptor(t, capabilities.CompanionToolDescriptors(), "browser_open")}),
			IsStrictlyEnforced: true,
		},
	}
	errorValue := assertOpenRouterLiveSchemaAccepted(ctx, backend, request)
	if errorValue != nil {
		t.Fatalf("expected live OpenRouter native tool schema to be accepted: %v", errorValue)
	}
}

func findLiveDescriptor(t *testing.T, descriptors []capabilities.Descriptor, toolName string) capabilities.Descriptor {
	t.Helper()
	for _, descriptor := range descriptors {
		if descriptor.Name == toolName {
			return descriptor
		}
	}
	t.Fatalf("descriptor %s not found", toolName)
	return capabilities.Descriptor{}
}

func TestOpenRouterLiveCalendarActionSchemaFromEnv(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	ctx := context.Background()
	request := StructuredRequest{
		Messages: []Message{{Role: "user", Content: "Add vacation to the calendar."}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "blueclaw_agent_turn_action",
			Document:           testActionSchemaForDescriptors(t, capabilities.CalendarDescriptors()),
			IsStrictlyEnforced: true,
		},
	}
	errorValue := assertOpenRouterLiveSchemaAccepted(ctx, backend, request)
	if errorValue != nil {
		t.Fatalf("expected live OpenRouter calendar schema to be accepted: %v", errorValue)
	}
}

func TestOpenRouterLiveSingleCalendarActionSchemaFromEnv(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	ctx := context.Background()
	request := StructuredRequest{
		Messages: []Message{{Role: "user", Content: "Add vacation to the calendar."}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "blueclaw_agent_turn_action",
			Document:           testActionSchemaForDescriptors(t, []capabilities.Descriptor{findLiveDescriptor(t, capabilities.CalendarDescriptors(), "calendar_add")}),
			IsStrictlyEnforced: true,
		},
	}
	errorValue := assertOpenRouterLiveSchemaAccepted(ctx, backend, request)
	if errorValue != nil {
		t.Fatalf("expected live OpenRouter single calendar schema to be accepted: %v", errorValue)
	}
}

func TestOpenRouterLiveLargePerToolActionSchemaFromEnv(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	ctx := context.Background()
	const probeToolCount = 30
	descriptors := make([]capabilities.Descriptor, 0, probeToolCount)
	for index := 0; index < probeToolCount; index++ {
		descriptors = append(descriptors, capabilities.Descriptor{
			Name:        "probe.tool." + strconv.Itoa(index),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"value":{"type":"string"}},"required":["value"],"additionalProperties":false}`),
		})
	}
	request := StructuredRequest{
		Messages: []Message{{Role: "user", Content: "Call one available probe tool."}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "blueclaw_agent_turn_action",
			Document:           testActionSchemaForDescriptors(t, descriptors),
			IsStrictlyEnforced: true,
		},
	}
	errorValue := assertOpenRouterLiveSchemaAccepted(ctx, backend, request)
	if errorValue != nil {
		t.Fatalf("expected large per-tool live OpenRouter native tool schema to be accepted: %v", errorValue)
	}
}

func TestOpenRouterLiveLowTierDiscriminatedUnionFromEnv(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	response, errorValue := backend.CompleteStructured(ctx, StructuredRequest{
		Model: backend.ModelName,
		Messages: []Message{{
			Role:    "user",
			Content: "Choose the tool decision and set toolName to browser.open.",
		}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name: "low_tier_discriminated_union_probe",
			Document: json.RawMessage(`{
				"oneOf":[
					{
						"type":"object",
						"properties":{"kind":{"type":"string","enum":["reply"]},"message":{"type":"string"}},
						"required":["kind","message"],
						"additionalProperties":false
					},
					{
						"type":"object",
						"properties":{"kind":{"type":"string","enum":["tool"]},"toolName":{"type":"string","enum":["browser_open"]}},
						"required":["kind","toolName"],
						"additionalProperties":false
					}
				]
			}`),
			IsStrictlyEnforced: true,
		},
	})
	if errorValue != nil {
		t.Fatalf("expected low-tier discriminated union response: %v", errorValue)
	}
	if response.ConstraintMode != ConstraintModeOpenAIJSONSchema {
		t.Fatalf("expected openai_json_schema mode, got %q (content=%s)", response.ConstraintMode, response.Content)
	}
	var decision struct {
		Kind     string `json:"kind"`
		ToolName string `json:"toolName"`
	}
	if errorValue := json.Unmarshal([]byte(response.Content), &decision); errorValue != nil {
		t.Fatalf("expected discriminated union JSON, got %q: %v", response.Content, errorValue)
	}
	if decision.Kind != "tool" || decision.ToolName != "browser_open" {
		t.Fatalf("expected tool branch with browser_open, got %+v", decision)
	}
}

func TestOpenRouterLiveApprovalReplyDecisionFromEnv(t *testing.T) {
	backend, _ := liveOpenRouterBackendFromEnv(t)
	ctx := context.Background()
	response, errorValue := backend.CompleteStructured(ctx, StructuredRequest{
		Messages: []Message{
			{
				Role: "system",
				Content: strings.Join([]string{
					"You decide whether the latest user message approves a pending action.",
					"Return isApproval true only when the latest message authorizes proceeding with the pending action.",
					"Short Korean affirmatives such as 응, 네, 좋아, 진행해, 해줘, 그래, 해 are approvals when they answer the pending approval question.",
					"Return false for cancellation, hesitation, a new unrelated request, or a question.",
				}, "\n"),
			},
			{
				Role: "user",
				Content: strings.Join([]string{
					"Pending task:",
					"웹사이트를 만들어서 배포해",
					"",
					"Approval question:",
					"배포를 진행해도 될까요?",
					"",
					"Latest user message:",
					"해",
				}, "\n"),
			},
		},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "blueclaw_confirmation_reply_decision",
			Document:           json.RawMessage(`{"type":"object","properties":{"isApproval":{"type":"boolean"},"reason":{"type":"string"}},"required":["isApproval","reason"],"additionalProperties":false}`),
			IsStrictlyEnforced: true,
		},
	})
	if errorValue != nil {
		t.Fatalf("expected live approval classification response: %v", errorValue)
	}
	var decision struct {
		IsApproval bool   `json:"isApproval"`
		Reason     string `json:"reason"`
	}
	if errorValue := json.Unmarshal([]byte(response.Content), &decision); errorValue != nil {
		t.Fatalf("expected approval decision JSON, got %q: %v", response.Content, errorValue)
	}
	if !decision.IsApproval {
		t.Fatalf("expected short Korean approval to be true, got %+v", decision)
	}
}

func liveOpenRouterBackendFromEnv(t *testing.T) (OpenRouterBackend, string) {
	t.Helper()
	loadTestEnvFile(t)
	if os.Getenv("INTERNKIM_LIVE_LLM_TEST") != "1" {
		t.Skip("set INTERNKIM_LIVE_LLM_TEST=1 to run live OpenRouter schema compatibility test")
	}
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	if apiKey == "" {
		t.Skip("OPENROUTER_API_KEY is required for live OpenRouter schema compatibility test")
	}
	keyPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(keyPath, []byte(apiKey), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	backend := OpenRouterBackend{
		KeyPath:    keyPath,
		BaseURL:    testEnvValue("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1/chat/completions"),
		ModelName:  testEnvValue("OPENROUTER_MODEL", defaultXLowModelName),
		HTTPClient: httpClientWithTimeout(45 * time.Second),
	}
	return backend, apiKey
}

func assertOpenRouterLiveSchemaAccepted(ctx context.Context, backend OpenRouterBackend, request StructuredRequest) error {
	apiKey, errorValue := backend.resolveAPIKey()
	if errorValue != nil {
		return errorValue
	}
	toolSet, _, errorValue := nativeActionToolsForSchema(request.StructuredOutputSchema)
	if errorValue != nil {
		return errorValue
	}
	requestDocument, _, errorValue := buildOpenRouterChatActionRequest(request, backend.resolveModelName(request.Model), toolSet.Tools, toolSet.NativeSchemaLint)
	if errorValue != nil {
		return errorValue
	}
	return assertOpenRouterRawRequestAccepted(ctx, backend, apiKey, requestDocument)
}

func assertOpenRouterRawRequestAccepted(ctx context.Context, backend OpenRouterBackend, apiKey string, requestDocument []byte) error {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, backend.BaseURL, bytes.NewReader(requestDocument))
	if errorValue != nil {
		return errorValue
	}
	httpRequest.Header.Set("Authorization", "Bearer "+apiKey)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := backend.HTTPClient.Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer httpResponse.Body.Close()
	responseDocument, errorValue := io.ReadAll(httpResponse.Body)
	if errorValue != nil {
		return errorValue
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return normalizeProviderError("openrouter", httpResponse.StatusCode, responseDocument)
	}
	return nil
}

func loadTestEnvFile(t *testing.T) {
	t.Helper()
	workingDirectory, errorValue := os.Getwd()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, path := range testEnvFileCandidates(workingDirectory) {
		content, errorValue := os.ReadFile(path)
		if errorValue == nil {
			applyTestEnvContent(string(content))
			return
		}
	}
}

func testEnvFileCandidates(workingDirectory string) []string {
	return []string{
		filepath.Join(workingDirectory, ".env"),
		filepath.Join(workingDirectory, "..", ".env"),
		filepath.Join(workingDirectory, "..", "..", ".env"),
	}
}

func applyTestEnvContent(content string) {
	for _, line := range strings.Split(content, "\n") {
		key, value, isFound := strings.Cut(strings.TrimSpace(line), "=")
		if !isFound || strings.TrimSpace(key) == "" || strings.HasPrefix(strings.TrimSpace(key), "#") {
			continue
		}
		if os.Getenv(strings.TrimSpace(key)) == "" {
			os.Setenv(strings.TrimSpace(key), normalizeTestEnvValue(value))
		}
	}
}

func normalizeTestEnvValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}

func testEnvValue(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func httpClientWithTimeout(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
