package llmbackend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenRouterLiveAgentActionSchemaFromEnv(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	backend := OpenRouterBackend{
		KeyPath:    keyPath,
		BaseURL:    testEnvValue("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1/chat/completions"),
		ModelName:  testEnvValue("OPENROUTER_MODEL", "google/gemini-2.5-flash"),
		HTTPClient: httpClientWithTimeout(45 * time.Second),
	}
	request := StructuredRequest{
		Messages: []Message{{Role: "user", Content: "Call browser.open for https://example.com."}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "blueclaw_agent_turn_action",
			Document:           json.RawMessage(`{"oneOf":[{"type":"object","properties":{"action":{"type":"string","enum":["call_tool"]},"toolName":{"type":"string","enum":["browser.open"]},"toolInput":{"type":"object","required":["url"],"additionalProperties":false}},"required":["action","toolName","toolInput"],"additionalProperties":false}]}`),
			IsStrictlyEnforced: true,
		},
	}
	errorValue := assertOpenRouterLiveSchemaAccepted(ctx, backend, request)
	if errorValue != nil {
		t.Fatalf("expected live OpenRouter native tool schema to be accepted: %v", errorValue)
	}
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
	requestDocument, errorValue := buildOpenRouterResponsesActionRequest(request, backend.resolveModelName(request.Model), toolSet.Tools)
	if errorValue != nil {
		return errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, openRouterResponsesURL(backend.BaseURL), bytes.NewReader(requestDocument))
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
		return errors.New(string(responseDocument))
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
