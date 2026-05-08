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

func TestOpenRouterLiveCalendarActionSchemaFromEnv(t *testing.T) {
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
		Messages: []Message{{Role: "user", Content: "Add vacation to the calendar."}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:               "blueclaw_agent_turn_action",
			Document:           json.RawMessage(liveCalendarActionSchema),
			IsStrictlyEnforced: true,
		},
	}
	errorValue := assertOpenRouterLiveSchemaAccepted(ctx, backend, request)
	if errorValue != nil {
		t.Fatalf("expected live OpenRouter calendar schema to be accepted: %v", errorValue)
	}
}

const liveCalendarActionSchema = `{"oneOf":[
	{"type":"object","properties":{"action":{"type":"string","enum":["final_reply"]},"finalReply":{"type":"string"},"goalStatus":{"type":"string","enum":["satisfied"]},"goalSatisfied":{"type":"boolean"},"completionEvidence":{"type":"array"},"qualityReview":{"type":"array"}},"required":["action","goalStatus","goalSatisfied","completionEvidence","qualityReview"],"additionalProperties":false},
	{"type":"object","properties":{"action":{"type":"string","enum":["fail"]},"reason":{"type":"string"}},"required":["action","reason"],"additionalProperties":false},
	{"type":"object","properties":{"action":{"type":"string","enum":["set_quality_criteria"]},"qualityCriteria":{"type":"array"}},"required":["action","qualityCriteria"],"additionalProperties":false},
	{"type":"object","properties":{"action":{"type":"string","enum":["call_tool"]},"toolName":{"type":"string","enum":["approval.request"]},"toolInput":{"type":"object","properties":{"message":{"type":"string"},"reason":{"type":"string"}},"required":["message"],"additionalProperties":false}},"required":["action","toolName","toolInput"],"additionalProperties":false},
	{"type":"object","properties":{"action":{"type":"string","enum":["call_tool"]},"toolName":{"type":"string","enum":["calendar.event.add"]},"toolInput":{"type":"object","properties":{"title":{"type":"string"},"description":{"type":"string"},"location":{"type":"string"},"startISO":{"type":"string"},"endISO":{"type":"string"},"timeZone":{"type":"string"},"isAllDay":{"type":"boolean"},"color":{"type":"string"},"people":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"}}]},"reminderLeadHours":{"type":"integer","enum":[1,2,3,6,12,24,48]}},"required":["title","startISO","endISO"],"additionalProperties":false}},"required":["action","toolName","toolInput"],"additionalProperties":false},
	{"type":"object","properties":{"action":{"type":"string","enum":["call_tool"]},"toolName":{"type":"string","enum":["calendar.event.delete"]},"toolInput":{"type":"object","properties":{"eventID":{"type":"string"}},"required":["eventID"],"additionalProperties":false}},"required":["action","toolName","toolInput"],"additionalProperties":false},
	{"type":"object","properties":{"action":{"type":"string","enum":["call_tool"]},"toolName":{"type":"string","enum":["calendar.event.list"]},"toolInput":{"type":"object","properties":{"startISO":{"type":"string"},"endISO":{"type":"string"},"query":{"type":"string"},"limit":{"type":"integer","minimum":1,"maximum":50}},"additionalProperties":false}},"required":["action","toolName","toolInput"],"additionalProperties":false},
	{"type":"object","properties":{"action":{"type":"string","enum":["call_tool"]},"toolName":{"type":"string","enum":["calendar.event.update"]},"toolInput":{"type":"object","properties":{"eventID":{"type":"string"},"title":{"type":"string"},"description":{"type":"string"},"location":{"type":"string"},"startISO":{"type":"string"},"endISO":{"type":"string"},"timeZone":{"type":"string"},"isAllDay":{"type":"boolean"},"color":{"type":"string"},"people":{"oneOf":[{"type":"string"},{"type":"array","items":{"type":"string"}}]},"reminderLeadHours":{"type":"integer","enum":[1,2,3,6,12,24,48]}},"required":["eventID","title","startISO","endISO"],"additionalProperties":false}},"required":["action","toolName","toolInput"],"additionalProperties":false}
]}`

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
