package capabilityd

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

func TestWebSearchUsesOpenRouterAutoServerTool(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	var requestDocument map[string]any
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.Header.Get("Authorization") != "Bearer sk-web" {
				t.Fatalf("unexpected authorization header: %q", request.Header.Get("Authorization"))
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&requestDocument); errorValue != nil {
				t.Fatalf("expected request body: %v", errorValue)
			}
			return jsonResponseBody(`{"choices":[{"message":{"content":"{\"provider\":\"openrouter\",\"remoteLLMInvolved\":true,\"compatibility\":\"openrouter_server_tool_auto\",\"query\":\"internkim\",\"answer\":\"result\",\"results\":[]}"}}]}`), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim","limit":3}}`))
	if errorValue != nil {
		t.Fatalf("expected web search: %v", errorValue)
	}

	tool := requestDocument["tools"].([]any)[0].(map[string]any)
	parameters := tool["parameters"].(map[string]any)
	if tool["type"] != "openrouter:web_search" || parameters["engine"] != "auto" || parameters["max_results"] != float64(3) {
		t.Fatalf("unexpected web search tool: %+v", tool)
	}
	if response.Provider != "openrouter" || response.SelectedBackend != "remote" || response.ToolName != "web.search" || response.Outcome != capabilities.ToolOutcomeSucceeded || response.IsError {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestWebSearchExtractsJSONFromMarkdownCodeFence(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	content := "```json\n{\"provider\":\"openrouter\",\"remoteLLMInvolved\":true,\"compatibility\":\"openrouter_server_tool_auto\",\"query\":\"internkim\",\"answer\":\"result\",\"results\":[{\"title\":\"InternKim\",\"url\":\"https://internkim.example\",\"snippet\":\"An agent platform\",\"source\":\"internkim.example\"}]}\n```"
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			responseDocument, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
			return jsonResponseBody(string(responseDocument)), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim"}}`))
	if errorValue != nil {
		t.Fatalf("expected web search: %v", errorValue)
	}
	var result struct {
		Query   string `json:"query"`
		Results []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"results"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatalf("expected normalized search result: %v", errorValue)
	}
	if response.IsError || len(result.Results) != 1 || result.Results[0].Title != "InternKim" {
		t.Fatalf("expected fenced JSON search content to succeed, response=%+v result=%+v", response, result)
	}
}

func TestWebSearchExtractsJSONAfterLeadingProse(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	content := "Here are the search results you requested:\n{\"provider\":\"openrouter\",\"remoteLLMInvolved\":true,\"compatibility\":\"openrouter_server_tool_auto\",\"query\":\"internkim\",\"answer\":\"result\",\"results\":[{\"title\":\"InternKim\",\"url\":\"https://internkim.example\",\"snippet\":\"An agent platform\",\"source\":\"internkim.example\"}]}\nHope that helps!"
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			responseDocument, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
			return jsonResponseBody(string(responseDocument)), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim"}}`))
	if errorValue != nil {
		t.Fatalf("expected web search: %v", errorValue)
	}
	var result struct {
		Query   string `json:"query"`
		Results []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"results"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatalf("expected normalized search result: %v", errorValue)
	}
	if response.IsError || len(result.Results) != 1 || result.Results[0].URL != "https://internkim.example" {
		t.Fatalf("expected prose-wrapped JSON search content to succeed, response=%+v result=%+v", response, result)
	}
}

func TestWebSearchWrapsPlainProseContentAsAnswer(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	content := "I could not find structured results, but internkim.com is InternKim's agent platform site."
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return jsonResponseBody(`{"choices":[{"message":{"content":"I could not find structured results, but internkim.com is InternKim's agent platform site."}}]}`), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim"}}`))
	if errorValue != nil {
		t.Fatalf("expected web search: %v", errorValue)
	}
	var result struct {
		Provider      string `json:"provider"`
		Compatibility string `json:"compatibility"`
		Query         string `json:"query"`
		Answer        string `json:"answer"`
		Results       []any  `json:"results"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatalf("expected normalized search result: %v", errorValue)
	}
	if response.IsError || result.Provider != "openrouter" || result.Compatibility != "openrouter_server_tool_content_text" || result.Query != "internkim" || result.Answer != content || len(result.Results) != 0 {
		t.Fatalf("expected plain prose to degrade into a text answer, response=%+v result=%+v", response, result)
	}
}

func TestWebSearchFillsMissingEchoFieldsDeterministically(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	content := `{"results":[{"title":"InternKim","url":"https://internkim.example","snippet":"An agent platform"}],"extraProviderField":"dropped"}`
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			responseDocument, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
			return jsonResponseBody(string(responseDocument)), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim"}}`))
	if errorValue != nil {
		t.Fatalf("expected web search: %v", errorValue)
	}
	var result struct {
		Provider          string `json:"provider"`
		RemoteLLMInvolved bool   `json:"remoteLLMInvolved"`
		Compatibility     string `json:"compatibility"`
		Query             string `json:"query"`
		Answer            string `json:"answer"`
		Results           []struct {
			Title string `json:"title"`
		} `json:"results"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatalf("expected normalized search result: %v", errorValue)
	}
	if response.IsError || result.Provider != "openrouter" || !result.RemoteLLMInvolved || result.Compatibility != "openrouter_server_tool_auto" || result.Query != "internkim" {
		t.Fatalf("expected deterministic echo fills, response=%+v result=%+v", response, result)
	}
	if len(result.Results) != 1 || result.Results[0].Title != "InternKim" || strings.Contains(string(response.Result), "extraProviderField") {
		t.Fatalf("expected provider extras dropped and results preserved, result=%s", string(response.Result))
	}
}

func TestWebFetchRejectsPrivateURLBeforeProviderCall(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	wasCalled := false
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			wasCalled = true
			return jsonResponseBody(`{}`), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.fetch", strings.NewReader(`{"input":{"urls":["http://127.0.0.1:8080"]}}`))
	if errorValue != nil {
		t.Fatalf("expected structured tool error response: %v", errorValue)
	}
	if !response.IsError || wasCalled {
		t.Fatalf("expected private URL rejection before provider call, response=%+v called=%v", response, wasCalled)
	}
}

func TestWebFetchAcceptsPlainTextOpenRouterContent(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	content := "\n  Dawn is a website for planning and operating a business.  \n"
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			responseDocument, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
			return jsonResponseBody(string(responseDocument)), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.fetch", strings.NewReader(`{"input":{"urls":["https://dawn.kim"]}}`))
	if errorValue != nil {
		t.Fatalf("expected web fetch: %v", errorValue)
	}
	if response.IsError {
		t.Fatalf("expected plain text fetch content to succeed, got %+v", response)
	}
	var result struct {
		Provider string `json:"provider"`
		Results  []struct {
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatalf("expected normalized fetch result: %v", errorValue)
	}
	if result.Provider != "openrouter" || len(result.Results) != 1 || result.Results[0].URL != "https://dawn.kim" || result.Results[0].Content != content {
		t.Fatalf("unexpected normalized fetch result: %+v", result)
	}
}

func TestWebFetchWrapsMultiURLPlainTextAsCombinedContent(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return jsonResponseBody(`{"choices":[{"message":{"content":"Combined page content"}}]}`), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.fetch", strings.NewReader(`{"input":{"urls":["https://dawn.kim","https://example.com"]}}`))
	if errorValue != nil {
		t.Fatalf("expected web fetch: %v", errorValue)
	}
	var result struct {
		Compatibility string `json:"compatibility"`
		Results       []struct {
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatalf("expected normalized fetch result: %v", errorValue)
	}
	if response.IsError || result.Compatibility != "openrouter_server_tool_combined_text" || len(result.Results) != 1 || result.Results[0].URL != "" || result.Results[0].Content != "Combined page content" {
		t.Fatalf("expected combined plain text result, response=%+v result=%+v", response, result)
	}
}

func TestWebFetchAcceptsSchemaJSONOpenRouterContent(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	content := `{"provider":"openrouter","remoteLLMInvolved":true,"compatibility":"openrouter_server_tool_auto","results":[{"url":"https://dawn.kim","finalURL":"https://dawn.kim","title":"Dawn","content":"Fetched"}],"errors":[]}`
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			responseDocument, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
			return jsonResponseBody(string(responseDocument)), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.fetch", strings.NewReader(`{"input":{"urls":["https://dawn.kim"]}}`))
	if errorValue != nil {
		t.Fatalf("expected web fetch: %v", errorValue)
	}
	if response.IsError || !json.Valid(response.Result) || !strings.Contains(string(response.Result), `"content":"Fetched"`) {
		t.Fatalf("expected schema JSON fetch content to succeed, got %+v", response)
	}
}

func TestWebFetchWrapsNonSchemaJSONAsRawContent(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	content := `{"provider":"openrouter","note":"not fetch schema"}`
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath:    secretPath,
			OpenRouterWebBaseURL: "https://openrouter.test/chat",
			OpenRouterModel:      "openrouter/search-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			responseDocument, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"message": map[string]string{"content": content}}}})
			return jsonResponseBody(string(responseDocument)), nil
		})},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.fetch", strings.NewReader(`{"input":{"urls":["https://dawn.kim"]}}`))
	if errorValue != nil {
		t.Fatalf("expected web fetch: %v", errorValue)
	}
	var result struct {
		Results []struct {
			Content string `json:"content"`
		} `json:"results"`
	}
	if errorValue := json.Unmarshal(response.Result, &result); errorValue != nil {
		t.Fatalf("expected normalized fetch result: %v", errorValue)
	}
	if response.IsError || len(result.Results) != 1 || result.Results[0].Content != content {
		t.Fatalf("expected non-schema JSON to be preserved as content, response=%+v result=%+v", response, result)
	}
}

func TestWebToolLocalOnlyBlocksOpenRouter(t *testing.T) {
	secretPath := writeOpenRouterSecretForWebToolTest(t, "sk-web")
	service := Service{Configuration: Configuration{OpenRouterKeyPath: secretPath, LocalOnly: true}.WithDefaults()}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim"}}`))
	if errorValue != nil {
		t.Fatalf("expected structured tool error response: %v", errorValue)
	}
	if !response.IsError || response.ErrorCode != "local_only" || response.ToolName != "web.search" || response.Outcome != capabilities.ToolOutcomeFailed {
		t.Fatalf("expected local-only denial, got %+v", response)
	}
}

func TestWebToolMissingOpenRouterKeyDoesNotLeakSecrets(t *testing.T) {
	service := Service{Configuration: Configuration{OpenRouterKeyPath: filepath.Join(t.TempDir(), "missing")}.WithDefaults()}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "web.search", strings.NewReader(`{"input":{"query":"internkim"}}`))
	if errorValue != nil {
		t.Fatalf("expected structured tool error response: %v", errorValue)
	}
	if !response.IsError || strings.Contains(string(response.Result), "sk-") {
		t.Fatalf("expected safe missing key response, got %+v", response)
	}
}

func writeOpenRouterSecretForWebToolTest(t *testing.T, value string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(path, []byte(value), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

func jsonResponseBody(document string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(document)),
		Header:     make(http.Header),
	}
}
