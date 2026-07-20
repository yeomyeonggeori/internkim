package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gitlab.com/eastriver/internkim/internal/runtime/locallm"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

func TestConfigurationDefaultsIncludeAdmindBaseURL(t *testing.T) {
	configuration := Configuration{}.WithDefaults()

	if configuration.AdmindBaseURL != DefaultConfiguration().AdmindBaseURL {
		t.Fatalf("expected admind base url default, got %q", configuration.AdmindBaseURL)
	}
}

func TestMattermostAskActionURLUsesConfiguredPublicBaseURL(t *testing.T) {
	configuration := Configuration{
		AdmindBaseURL:                "http://127.0.0.1:18080",
		MattermostInteractiveBaseURL: " https://poc0-t15.intern.kim/ ",
	}
	service := Service{Configuration: configuration}

	actualURL := service.mattermostAskActionBuilder().URL
	if actualURL != "https://poc0-t15.intern.kim/_internkim/mattermost/actions" {
		t.Fatalf("ask action URL = %q", actualURL)
	}
}

func TestMattermostHealthRejectsHumanTokenUser(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "mattermost-token")
	if errorValue := os.WriteFile(tokenPath, []byte("human-token"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := Service{
		Configuration: Configuration{MattermostBaseURL: "https://mattermost.test", MattermostTokenPath: tokenPath},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return testJSONResponse(http.StatusOK, map[string]any{"id": "human-1", "username": "internkim", "is_bot": false}), nil
		})},
	}
	service.healthState().Update(func(state *platformHealthState) {
		state.MattermostForwarderRunning = true
		state.MattermostBotUserResolved = true
	})

	health := service.mattermostHealth(context.Background())
	if health["ok"] != false || health["botUserResolved"] != false {
		t.Fatalf("expected human token user to be unhealthy, got %+v", health)
	}
}

func TestDefaultLlamaCppModelMatchesActualDeployedModel(t *testing.T) {
	configuration := DefaultConfiguration()

	deployedModelName := strings.TrimSuffix(locallm.LlamaCppModelFilename, ".gguf")
	if configuration.LlamaCppModel != "local/"+deployedModelName {
		t.Fatalf("expected default llamacpp model label to reflect the actual on-device model %q, got %q", locallm.LlamaCppModelFilename, configuration.LlamaCppModel)
	}
	if strings.Contains(strings.ToLower(configuration.LlamaCppModel), "e4b") {
		t.Fatalf("default llamacpp model label must not reference the E4B variant, which does not fit the 8GB Jetson: got %q", configuration.LlamaCppModel)
	}
}

func TestHTTPClientTimeoutTracksProviderAttemptTimeout(t *testing.T) {
	service := Service{Configuration: Configuration{}.WithDefaults()}

	if service.httpClientTimeout() != 120*time.Second {
		t.Fatalf("expected general HTTP timeout, got %s", service.httpClientTimeout())
	}

	shortTimeoutService := Service{Configuration: Configuration{ProviderAttemptTimeout: 30 * time.Second}.WithDefaults()}
	if shortTimeoutService.httpClientTimeout() != 120*time.Second {
		t.Fatalf("expected minimum http timeout, got %s", shortTimeoutService.httpClientTimeout())
	}
}

func TestProviderHTTPClientHasNoDefaultTimeout(t *testing.T) {
	service := Service{Configuration: Configuration{}.WithDefaults()}
	if service.providerHTTPClient().Timeout != 0 {
		t.Fatalf("expected provider HTTP client without a default timeout, got %s", service.providerHTTPClient().Timeout)
	}
	if service.companionInferenceProvider().httpClient().Timeout != 0 {
		t.Fatalf("expected companion inference client without a default timeout, got %s", service.companionInferenceProvider().httpClient().Timeout)
	}
}

func TestMattermostAskMenuOptionsUseShortLabelsAndEncodedSelection(t *testing.T) {
	options := mattermostAskMenuOptions([]platformAskChoiceOption{{
		Key:        "A",
		Label:      "최대한 가독성 있게",
		ShortLabel: "가독성",
	}, {
		Key:   "B",
		Label: "verylongsinglewordlabel",
	}})

	if len(options) != 2 {
		t.Fatalf("expected two menu options, got %+v", options)
	}
	if options[0].Text != "가독성" {
		t.Fatalf("expected short menu label, got %+v", options[0])
	}
	var selectedChoice map[string]string
	if errorValue := json.Unmarshal([]byte(options[0].Value), &selectedChoice); errorValue != nil {
		t.Fatalf("expected encoded menu value: %v", errorValue)
	}
	if selectedChoice["key"] != "A" || selectedChoice["label"] != "가독성" {
		t.Fatalf("expected encoded key and label, got %+v", selectedChoice)
	}
	if options[1].Text != "verylong" {
		t.Fatalf("expected truncated fallback menu label, got %+v", options[1])
	}
}

func TestLocalStructuredCompletionUsesRequestedAccelerator(t *testing.T) {
	setLiteRTConstrainedRunnerPath(t, createLiteRTConstrainedRunner(t))
	configuration := DefaultConfiguration()
	configuration.LocalBackendOrder = []string{"litert"}
	service := Service{
		Configuration: configuration,
		RunCommand: func(ctx context.Context, executablePath string, arguments []string, standardInput []byte) ([]byte, error) {
			_ = ctx
			_ = executablePath
			_ = arguments
			document := string(standardInput)
			if strings.Contains(document, `"accelerator":"cpu"`) &&
				strings.Contains(document, `"constrainedDecoding"`) {
				return []byte(`{"content":"{\"content\":\"ok\"}","constraintMode":"litert_llguidance_json_schema"}`), nil
			}
			t.Fatalf("unexpected wrapper request: %s", document)
			return nil, nil
		},
	}

	response, errorValue := service.completeStructured(context.Background(), StructuredLLMRequest{
		ExecutionMode: "device",
		Accelerator:   "cpu",
		Messages:      []LLMMessage{{Role: "user", Content: "hello"}},
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "plain_text_response",
			Document: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected local completion to succeed: %v", errorValue)
	}

	if response.SelectedBackend != "cpu" {
		t.Fatalf("expected cpu backend, got %q", response.SelectedBackend)
	}
	if response.ConstraintMode != "litert_llguidance_json_schema" {
		t.Fatalf("expected LiteRT constrained decoding mode, got %q", response.ConstraintMode)
	}
}

func TestCompanionInferenceModeStopsOnlyJetsonGenerationService(t *testing.T) {
	commands := []string{}
	service := Service{
		Configuration: Configuration{LocalInferenceMode: "companion_only"},
		RunCommand: func(_ context.Context, executablePath string, arguments []string, _ []byte) ([]byte, error) {
			commands = append(commands, executablePath+" "+strings.Join(arguments, " "))
			return []byte("ok"), nil
		},
	}
	service.applyLocalInferenceMode(context.Background())

	expectedCommands := []string{
		"systemctl stop internkim-llamacpp.service",
		"systemctl disable internkim-llamacpp.service",
	}
	if !reflect.DeepEqual(commands, expectedCommands) {
		t.Fatalf("expected generation service only commands, got %+v", commands)
	}
}

func TestLocalStructuredCompletionRejectsInvalidStructuredOutput(t *testing.T) {
	setLiteRTConstrainedRunnerPath(t, createLiteRTConstrainedRunner(t))
	service := Service{
		Configuration: DefaultConfiguration(),
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"content":"plain text","constraintMode":"litert_llguidance_json_schema"}`), nil
		},
	}

	_, errorValue := service.completeStructured(context.Background(), StructuredLLMRequest{
		ExecutionMode: "device",
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "plain_text_response",
			Document: json.RawMessage(`{"type":"object","properties":{"content":{"type":"string"}},"required":["content"],"additionalProperties":false}`),
		},
	})
	if errorValue == nil {
		t.Fatalf("expected invalid structured output to fail")
	}
}

func TestTextCompletionReturnsPlainContent(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.LocalBackendOrder = []string{"litert"}
	service := Service{
		Configuration: configuration,
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"content":"plain reply"}`), nil
		},
	}

	response, errorValue := service.completeText(context.Background(), TextLLMRequest{
		ExecutionMode: "device",
		Messages:      []LLMMessage{{Role: "user", Content: "hello"}},
	})
	if errorValue != nil {
		t.Fatalf("expected text completion to succeed: %v", errorValue)
	}
	if response.Content != "plain reply" {
		t.Fatalf("expected plain text content, got %q", response.Content)
	}
}

func TestStructuredEndpointReturnsConstrainedContent(t *testing.T) {
	setLiteRTConstrainedRunnerPath(t, createLiteRTConstrainedRunner(t))
	configuration := DefaultConfiguration()
	configuration.LocalBackendOrder = []string{"litert"}
	service := Service{
		Configuration: configuration,
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"content":"{\"reply\":\"hello\"}","constraintMode":"litert_llguidance_json_schema"}`), nil
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/structured", strings.NewReader(`{
		"model":"local/gemma",
		"executionMode":"device",
		"messages":[{"role":"user","content":"hello"}],
		"structuredOutputSchema":{
			"name":"reply",
			"document":{"type":"object","properties":{"reply":{"type":"string"}},"required":["reply"],"additionalProperties":false},
			"isStrictlyEnforced":true
		}
	}`))
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected structured endpoint success, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response LLMResponse
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.Content != `{"reply":"hello"}` {
		t.Fatalf("expected structured content, got %q", response.Content)
	}
	if response.ConstraintMode != "litert_llguidance_json_schema" {
		t.Fatalf("expected LiteRT constrained decoding mode, got %q", response.ConstraintMode)
	}
}

func TestChatEndpointReturnsNativeToolCalls(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var receivedDocument map[string]any
	service := Service{
		Configuration: Configuration{
			OpenRouterKeyPath: secretPath,
			OpenRouterBaseURL: "https://openrouter.test/api/v1/chat/completions",
			OpenRouterModel:   "configured-model",
		}.WithDefaults(),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/api/v1/chat/completions" {
				t.Fatalf("expected OpenRouter chat path, got %s", request.URL.Path)
			}
			if request.Header.Get("Authorization") != "Bearer sk-test" {
				t.Fatalf("expected authorization header, got %q", request.Header.Get("Authorization"))
			}
			if errorValue := json.NewDecoder(request.Body).Decode(&receivedDocument); errorValue != nil {
				t.Fatalf("expected request document: %v", errorValue)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"call-1","type":"function","function":{"name":"lookup","arguments":"{\"query\":\"status\"}"}}]}}],"usage":{"prompt_tokens":5,"completion_tokens":2,"total_tokens":7}}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/chat", strings.NewReader(`{
		"model":"default",
		"executionMode":"remote",
		"messages":[{"role":"user","content":"check status"}],
		"tools":[{"type":"function","function":{"name":"lookup","description":"Lookup status","parameters":{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}}}],
		"toolChoice":{"type":"function","function":{"name":"lookup"}},
		"parallelToolCalls":false
	}`))
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected chat endpoint success, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	if receivedDocument["model"] != "configured-model" {
		t.Fatalf("expected configured model, got %+v", receivedDocument)
	}
	toolChoice := receivedDocument["tool_choice"].(map[string]any)
	function := toolChoice["function"].(map[string]any)
	if toolChoice["type"] != "function" || function["name"] != "lookup" {
		t.Fatalf("expected tool choice object, got %+v", toolChoice)
	}
	if receivedDocument["parallel_tool_calls"] != false {
		t.Fatalf("expected parallel tool calls false, got %+v", receivedDocument)
	}
	var response ChatLLMResponse
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.FinishReason != "tool_calls" {
		t.Fatalf("expected tool_calls finish reason, got %q", response.FinishReason)
	}
	if len(response.Message.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %+v", response.Message.ToolCalls)
	}
	toolCall := response.Message.ToolCalls[0]
	if toolCall.Function.Name != "lookup" || toolCall.Function.Arguments != `{"query":"status"}` {
		t.Fatalf("expected tool call arguments string, got %+v", toolCall)
	}
	if response.Usage.TotalTokens != 7 {
		t.Fatalf("expected usage to round trip, got %+v", response.Usage)
	}
}

func TestTextEndpointReturnsPlainContent(t *testing.T) {
	configuration := DefaultConfiguration()
	configuration.LocalBackendOrder = []string{"litert"}
	service := Service{
		Configuration: configuration,
		RunCommand: func(context.Context, string, []string, []byte) ([]byte, error) {
			return []byte(`{"content":"plain endpoint reply"}`), nil
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/text", strings.NewReader(`{
		"executionMode":"device",
		"messages":[{"role":"user","content":"hello"}]
	}`))
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusOK {
		t.Fatalf("expected text endpoint success, got %d: %s", responseRecorder.Code, responseRecorder.Body.String())
	}
	var response LLMResponse
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("expected response to decode: %v", errorValue)
	}
	if response.Content != "plain endpoint reply" {
		t.Fatalf("expected plain response content, got %q", response.Content)
	}
}

func TestHealthIncludesLiteRTProviderAvailability(t *testing.T) {
	setLiteRTConstrainedRunnerPath(t, filepath.Join(t.TempDir(), "missing-constrained-runner"))
	service := Service{Configuration: DefaultConfiguration()}
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	var response map[string]any
	if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&response); errorValue != nil {
		t.Fatalf("expected health response to decode: %v", errorValue)
	}
	providers, hasProviders := response["providers"].(map[string]any)
	if !hasProviders {
		t.Fatalf("expected providers health section, got %+v", response)
	}
	liteRT, hasLiteRT := providers["litert"].(map[string]any)
	if !hasLiteRT {
		t.Fatalf("expected LiteRT health section, got %+v", providers)
	}
	if liteRT["available"] != false {
		t.Fatalf("expected LiteRT to be unavailable, got %+v", liteRT)
	}
	if liteRT["reason"] != "constrained runner not installed" {
		t.Fatalf("expected constrained runner reason, got %+v", liteRT)
	}
}

func TestHealthRequiresMatchingLLMDProtocolIdentity(t *testing.T) {
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	validDocument, errorValue := json.Marshal(map[string]string{
		"status":                "ok",
		"protocolVersion":       expectedIdentity.ProtocolVersion,
		"aggregateProtocolHash": expectedIdentity.AggregateProtocolHash,
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	cases := []struct {
		name           string
		llmdDocument   string
		llmdError      error
		expectedStatus int
	}{
		{name: "matching", llmdDocument: string(validDocument), expectedStatus: http.StatusOK},
		{name: "missing identity", llmdDocument: `{"status":"ok"}`, expectedStatus: http.StatusServiceUnavailable},
		{name: "malformed", llmdDocument: `not-json`, expectedStatus: http.StatusServiceUnavailable},
		{name: "unavailable", llmdError: errors.New("LLMD unavailable"), expectedStatus: http.StatusServiceUnavailable},
		{name: "mismatched identity", llmdDocument: `{"status":"ok","protocolVersion":"0.4.0","aggregateProtocolHash":"0000000000000000000000000000000000000000000000000000000000000000"}`, expectedStatus: http.StatusServiceUnavailable},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newHealthTestService(t, testCase.llmdDocument, testCase.llmdError)
			request := httptest.NewRequest(http.MethodGet, "/health", nil)
			responseRecorder := httptest.NewRecorder()

			service.router().ServeHTTP(responseRecorder, request)

			if responseRecorder.Code != testCase.expectedStatus {
				t.Fatalf("expected health status %d, got %d: %s", testCase.expectedStatus, responseRecorder.Code, responseRecorder.Body.String())
			}
			var response map[string]any
			if errorValue := json.NewDecoder(responseRecorder.Body).Decode(&response); errorValue != nil {
				t.Fatalf("expected health response to decode: %v", errorValue)
			}
			if response["protocolVersion"] != expectedIdentity.ProtocolVersion || response["aggregateProtocolHash"] != expectedIdentity.AggregateProtocolHash {
				t.Fatalf("expected capabilityd protocol identity, got %+v", response)
			}
			if testCase.expectedStatus == http.StatusOK && response["status"] != "ok" {
				t.Fatalf("expected healthy status, got %+v", response)
			}
			if testCase.expectedStatus != http.StatusOK && response["status"] != "unhealthy" {
				t.Fatalf("expected unhealthy status, got %+v", response)
			}
		})
	}
}

func newHealthTestService(t *testing.T, llmdDocument string, llmdError error) Service {
	t.Helper()
	tokenPath := filepath.Join(t.TempDir(), "mattermost-token")
	if errorValue := os.WriteFile(tokenPath, []byte("test-token"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return Service{
		Configuration: Configuration{
			MattermostBaseURL:   "https://mattermost.test",
			MattermostTokenPath: tokenPath,
			LLMDSocketPath:      "/tmp/llmd-health-test.sock",
		},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return testJSONResponse(http.StatusOK, map[string]any{"id": "bot-1", "is_bot": true}), nil
		})},
		LLMDHTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if llmdError != nil {
				return nil, llmdError
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(llmdDocument)),
			}, nil
		})},
		HealthState: &platformHealthState{MattermostForwarderRunning: true},
	}
}

func TestToolInvokeDoesNotExposeSecretsForUnconfiguredTool(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-must-not-leak")
	service := Service{Configuration: DefaultConfiguration()}
	request := httptest.NewRequest(http.MethodPost, "/v1/tools/google.search/invoke", strings.NewReader(`{"input":{"query":"hello"}}`))
	responseRecorder := httptest.NewRecorder()

	service.router().ServeHTTP(responseRecorder, request)

	if responseRecorder.Code != http.StatusBadGateway {
		t.Fatalf("expected unconfigured tool to fail safely, got %d", responseRecorder.Code)
	}
	responseBody := responseRecorder.Body.String()
	if strings.Contains(responseBody, "sk-must-not-leak") {
		t.Fatalf("expected tool error to omit secrets, got %q", responseBody)
	}
	if !strings.Contains(responseBody, "capability tool is not configured") {
		t.Fatalf("expected safe tool error, got %q", responseBody)
	}
}

func TestEmbeddingCreateUsesOpenRouterSecretWithoutReturningIt(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	service := Service{Configuration: Configuration{
		OpenRouterKeyPath:          secretPath,
		OpenRouterEmbeddingBaseURL: "https://example.test/embeddings",
		OpenRouterEmbeddingModel:   "embedding-model",
	}}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer sk-test" {
			t.Fatalf("unexpected authorization header: %q", request.Header.Get("Authorization"))
		}
		var document map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
			t.Fatal(errorValue)
		}
		if document["model"] != "embedding-model" {
			t.Fatalf("unexpected model: %v", document["model"])
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"model":"embedding-model","data":[{"embedding":[0.1,0.2]}]}`)),
			Header:     make(http.Header),
		}, nil
	})}
	response, errorValue := service.createEmbedding(context.Background(), EmbeddingRequest{Input: "hello", ExecutionMode: "remote"})
	if errorValue != nil {
		t.Fatalf("expected embedding creation to succeed: %v", errorValue)
	}

	responseDocument, errorValue := json.Marshal(response)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(responseDocument), "sk-test") {
		t.Fatal("expected embedding response to omit the API key")
	}
	if !strings.Contains(string(responseDocument), `"embedding":[0.1,0.2]`) {
		t.Fatalf("unexpected embedding response: %s", responseDocument)
	}
}

func TestRemoteEmbeddingModePreservesRequestedEmbeddingModel(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "openrouter-api-key")
	if errorValue := os.WriteFile(secretPath, []byte("sk-test"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	service := Service{Configuration: Configuration{
		OpenRouterKeyPath:          secretPath,
		OpenRouterEmbeddingBaseURL: "https://example.test/embeddings",
		LocalInferenceMode:         "remote",
	}}
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		var document map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
			t.Fatal(errorValue)
		}
		if document["model"] != "embeddinggemma" {
			t.Fatalf("expected requested embedding model, got %v", document["model"])
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"model":"embeddinggemma","data":[{"embedding":[0.1,0.2]}]}`)),
			Header:     make(http.Header),
		}, nil
	})}

	_, errorValue := service.createEmbedding(context.Background(), EmbeddingRequest{Input: "hello", Model: "embeddinggemma", ExecutionMode: "auto"})
	if errorValue != nil {
		t.Fatalf("expected remote embedding creation to succeed: %v", errorValue)
	}
}
