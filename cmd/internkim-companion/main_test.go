package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	browserruntime "gitlab.com/eastriver/internkim/internal/browser"
	"gitlab.com/eastriver/internkim/internal/capabilities"
	companionruntime "gitlab.com/eastriver/internkim/internal/companion"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
)

func newOllamaStubClient(t *testing.T, expectedFormat bool, content string) (*http.Client, *bool) {
	t.Helper()
	called := false
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if !strings.HasSuffix(request.URL.Path, "/api/chat") {
			t.Fatalf("expected /api/chat call, got %s", request.URL.Path)
		}
		var document map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
			t.Fatalf("expected request body: %v", errorValue)
		}
		_, hasFormat := document["format"]
		if hasFormat != expectedFormat {
			t.Fatalf("format field present=%v, expected=%v", hasFormat, expectedFormat)
		}
		called = true
		body := `{"message":{"role":"assistant","content":"` + content + `"}}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
	return client, &called
}

func newLlamaCppStructuredStubClient(t *testing.T, content string) (*http.Client, *bool) {
	t.Helper()
	called := false
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("expected /v1/chat/completions call, got %s", request.URL.Path)
		}
		var document map[string]any
		if errorValue := json.NewDecoder(request.Body).Decode(&document); errorValue != nil {
			t.Fatalf("expected request body: %v", errorValue)
		}
		responseFormat, isFound := document["response_format"].(map[string]any)
		if !isFound || responseFormat["type"] != "json_schema" {
			t.Fatalf("expected json_schema response_format, got %+v", document)
		}
		called = true
		body := `{"choices":[{"message":{"content":"` + content + `"}}]}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
	return client, &called
}

func newLocalLLMTestSettings(configuration llmbackend.LocalProviderConfig) localLLMSettings {
	return localLLMSettings{
		Enabled:       true,
		Configuration: configuration,
		ProviderSet:   llmbackend.BuildLocalProviderSet(configuration),
	}
}

func TestLLMHandlerTextRoutesToOllamaBackend(t *testing.T) {
	httpClient, called := newOllamaStubClient(t, false, "hello from ollama")
	settings := newLocalLLMTestSettings(llmbackend.LocalProviderConfig{
		ProviderOrder: []string{"ollama"},
		OllamaBaseURL: "https://ollama.test",
		OllamaModel:   "gemma3:1b",
		HTTPClient:    httpClient,
	})
	handler := llmHandler(newDynamicLocalLLM(settings), false, false)

	body := `{"messages":[{"role":"user","content":"hi"}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/text", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	var result map[string]any
	if errorValue := json.Unmarshal(response.Body.Bytes(), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result["content"] != "hello from ollama" {
		t.Fatalf("expected ollama content, got %+v", result)
	}
	if result["selectedBackend"] != capabilities.LLMBackendCompanionLocal {
		t.Fatalf("expected companion-local selection, got %+v", result)
	}
	if !*called {
		t.Fatal("expected ollama backend to be invoked")
	}
}

func TestLLMHandlerHonorsRequestedProvider(t *testing.T) {
	httpClient, called := newOllamaStubClient(t, false, "hello from requested ollama")
	settings := newLocalLLMTestSettings(llmbackend.LocalProviderConfig{
		ProviderOrder:   []string{"llamacpp", "ollama"},
		OllamaBaseURL:   "https://ollama.test",
		OllamaModel:     "gemma3:1b",
		LlamaCppBaseURL: "https://llamacpp.test",
		LlamaCppModel:   "local/gemma",
		HTTPClient:      httpClient,
	})
	handler := llmHandler(newDynamicLocalLLM(settings), false, false)

	body := `{"provider":"ollama","messages":[{"role":"user","content":"hi"}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/text", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	if !*called {
		t.Fatal("expected requested ollama backend to be invoked")
	}
}

func TestLLMHandlerStructuredEnforcesSchema(t *testing.T) {
	httpClient, called := newLlamaCppStructuredStubClient(t, `{\"reply\":\"ok\"}`)
	settings := newLocalLLMTestSettings(llmbackend.LocalProviderConfig{
		ProviderOrder:   []string{"llamacpp"},
		LlamaCppBaseURL: "https://llamacpp.test",
		LlamaCppModel:   "local/gemma",
		HTTPClient:      httpClient,
	})
	handler := llmHandler(newDynamicLocalLLM(settings), false, true)

	body := `{"messages":[{"role":"user","content":"hi"}],"structuredOutputSchema":{"name":"reply","document":{"type":"object","required":["reply"]}}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/structured", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", response.Code, response.Body.String())
	}
	var result map[string]any
	if errorValue := json.Unmarshal(response.Body.Bytes(), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if result["constraintMode"] != "llama_json_schema" {
		t.Fatalf("expected llama JSON schema mode, got %+v", result)
	}
	if !*called {
		t.Fatal("expected llama.cpp backend to be invoked")
	}
}

func TestLLMHandlerReturnsServiceUnavailableWhenAllBackendsFail(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		_ = request
		return nil, errSimulatedTransport
	})}
	settings := newLocalLLMTestSettings(llmbackend.LocalProviderConfig{
		ProviderOrder: []string{"ollama"},
		OllamaBaseURL: "https://ollama.test",
		OllamaModel:   "gemma3:1b",
		HTTPClient:    httpClient,
	})
	handler := llmHandler(newDynamicLocalLLM(settings), false, false)

	body := `{"messages":[{"role":"user","content":"hi"}]}`
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/text", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", response.Code)
	}
	var result map[string]any
	if errorValue := json.Unmarshal(response.Body.Bytes(), &result); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(result["hint"].(string), "ollama") {
		t.Fatalf("expected hint to mention ollama, got %+v", result)
	}
}

func TestLLMHandlerWithoutLocalEnabledReturnsNotImplemented(t *testing.T) {
	handler := llmHandler(newDynamicLocalLLM(localLLMSettings{}), false, false)
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/text", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	handler(response, request)
	if response.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501 when local LLM disabled, got %d", response.Code)
	}
}

func TestLLMStreamHandlerEmitsTokensAsSSE(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		_ = request
		body := strings.Join([]string{
			`{"message":{"role":"assistant","content":"he"},"done":false}`,
			`{"message":{"role":"assistant","content":"llo"},"done":false}`,
			`{"message":{"role":"assistant","content":""},"done":true}`,
		}, "\n")
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}
	providerSet := llmbackend.BuildLocalProviderSet(llmbackend.LocalProviderConfig{
		ProviderOrder: []string{"ollama"},
		OllamaBaseURL: "https://ollama.test",
		OllamaModel:   "gemma3:1b",
		HTTPClient:    httpClient,
	})
	handler := llmStreamHandler(newDynamicLocalLLM(localLLMSettings{
		Enabled:     true,
		ProviderSet: providerSet,
	}))

	request := httptest.NewRequest(http.MethodPost, "/v1/llm/stream", strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("expected SSE content type, got %q", response.Header().Get("Content-Type"))
	}
	body := response.Body.String()
	if !strings.Contains(body, `"token":"he"`) || !strings.Contains(body, `"token":"llo"`) {
		t.Fatalf("expected token frames, got %q", body)
	}
	if !strings.Contains(body, "event: done") {
		t.Fatalf("expected done event, got %q", body)
	}
}

func TestCompanionDefaultLocalProviderOrderStartsWithLlamaCpp(t *testing.T) {
	flags := flag.NewFlagSet("test", flag.ContinueOnError)
	localFlags := registerLocalLLMFlags(flags)
	if errorValue := flags.Parse(nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	settings := localFlags.settings(http.DefaultClient)
	expected := []string{"llamacpp", "ollama", "mlx"}
	for index, backend := range settings.ProviderSet.Backends {
		if backend.Name() != expected[index] {
			t.Fatalf("expected %s at index %d, got %s", expected[index], index, backend.Name())
		}
	}
}

func TestSharedLocalProviderSetBuildsBackendsInRequestedOrder(t *testing.T) {
	providerSet := llmbackend.BuildLocalProviderSet(llmbackend.LocalProviderConfig{
		ProviderOrder:   []string{"llamacpp", "ollama", "mlx"},
		OllamaBaseURL:   "http://o",
		LlamaCppBaseURL: "http://l",
		MLXBaseURL:      "http://m",
	})
	backends := providerSet.Backends
	if len(backends) != 3 {
		t.Fatalf("expected three backends, got %d", len(backends))
	}
	expected := []string{"llamacpp", "ollama", "mlx"}
	for index, backend := range backends {
		if backend.Name() != expected[index] {
			t.Fatalf("expected %s at index %d, got %s", expected[index], index, backend.Name())
		}
	}
}

func TestSharedLocalProviderSetSkipsUnknownProviderNames(t *testing.T) {
	providerSet := llmbackend.BuildLocalProviderSet(llmbackend.LocalProviderConfig{
		ProviderOrder: []string{"unknown", "ollama"},
	})
	backends := providerSet.Backends
	if len(backends) != 1 {
		t.Fatalf("expected single ollama backend, got %d", len(backends))
	}
	if backends[0].Name() != "ollama" {
		t.Fatalf("expected ollama backend, got %s", backends[0].Name())
	}
}

func TestLLMHandlerStructuredReturnsUnsupportedForOllamaOnly(t *testing.T) {
	settings := newLocalLLMTestSettings(llmbackend.LocalProviderConfig{
		ProviderOrder: []string{"ollama"},
	})
	handler := llmHandler(newDynamicLocalLLM(settings), false, true)

	body := `{"messages":[{"role":"user","content":"hi"}],"structuredOutputSchema":{"name":"reply","document":{"type":"object","required":["reply"]}}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/llm/structured", strings.NewReader(body))
	response := httptest.NewRecorder()
	handler(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d body=%s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "deterministic structured output") {
		t.Fatalf("expected deterministic structured output error, got %s", response.Body.String())
	}
}

var errSimulatedTransport = simulatedError("simulated transport failure")

type simulatedError string

func (errorValue simulatedError) Error() string { return string(errorValue) }

var _ llmbackend.Backend = llmbackend.OllamaBackend{}

func TestDefaultCapabilitiesAdvertiseLLMOnlyInDevelopmentMockMode(t *testing.T) {
	withoutMockLLM := companionruntime.DefaultCapabilities(true, false)
	withMockLLM := companionruntime.DefaultCapabilities(true, true)

	if hasCapability(withoutMockLLM, "llm.structured") {
		t.Fatal("expected LLM capability to be hidden without development mock mode")
	}
	if !hasCapability(withMockLLM, "llm.structured") {
		t.Fatal("expected LLM capability in development mock mode")
	}
	if !hasCapability(withoutMockLLM, "browser_open") {
		t.Fatal("expected browser capability to be advertised")
	}
}

func TestMainSubcommandsReturnAfterExecution(t *testing.T) {
	for _, commandName := range []string{"pair", "run", "status"} {
		if !isCompanionSubcommand(commandName) {
			t.Fatalf("expected %s to be a subcommand", commandName)
		}
	}
	if isCompanionSubcommand("serve") {
		t.Fatal("serve should fall through to default server mode")
	}
}

func TestRunServerRejectsNonLoopbackListenAddress(t *testing.T) {
	errorValue := runServer([]string{"--listen", "0.0.0.0:0"})
	if errorValue == nil {
		t.Fatal("expected non-loopback listen address to be rejected")
	}
	if !strings.Contains(errorValue.Error(), "must listen on loopback") {
		t.Fatalf("expected loopback rejection error, got %v", errorValue)
	}
}

func hasCapability(descriptors []capabilities.Descriptor, name string) bool {
	for _, descriptor := range descriptors {
		if descriptor.Name == name {
			return true
		}
	}
	return false
}

func TestPairSavesState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/_internkim/companion/pair" {
			t.Fatalf("unexpected pair path: %s", request.URL.Path)
		}
		return textResponse(http.StatusOK, `{"companionID":"companion-1","token":"token-1"}`), nil
	})}

	secureStore := companionruntime.NewMemorySecureStore()
	errorValue := runPairWithStore([]string{"--device-url", "https://device.example.test", "--code", "ABCD-1234", "--state", statePath, "--local-only", "--dev-mock-llm"}, httpClient, secureStore)
	if errorValue != nil {
		t.Fatalf("expected pair success: %v", errorValue)
	}
	state, errorValue := loadState(statePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.CompanionID != "companion-1" || state.Token != "token-1" || !state.LocalOnly {
		t.Fatalf("unexpected state: %+v", state)
	}
	if state.PrivateKey != "" || state.PrivateKeyID == "" {
		t.Fatalf("expected state to keep only private key reference: %+v", state)
	}
	if _, errorValue := secureStore.Get(nilContext(), state.PrivateKeyID); errorValue != nil {
		t.Fatalf("expected private key in secure store: %v", errorValue)
	}
	if !hasCapability(state.Capabilities, "llm.structured") {
		t.Fatal("expected development LLM capability to be stored")
	}
}

func TestPairAcceptsDeepLinkArgument(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://device.example.test/_internkim/companion/pair" {
			t.Fatalf("unexpected pair endpoint: %s", request.URL.String())
		}
		return textResponse(http.StatusOK, `{"companionID":"companion-1","token":"token-1"}`), nil
	})}

	secureStore := companionruntime.NewMemorySecureStore()
	errorValue := runPairWithStore([]string{"--state", statePath, "internkim://pair?device_url=https%3A%2F%2Fdevice.example.test&code=ABCD-1234"}, httpClient, secureStore)
	if errorValue != nil {
		t.Fatalf("expected deep link pair success: %v", errorValue)
	}
	state, errorValue := loadState(statePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if state.DeviceURL != "https://device.example.test" {
		t.Fatalf("unexpected device URL: %s", state.DeviceURL)
	}
}

func TestPairReportsHTMLInsteadOfRawJSONDecodeError(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return textResponse(http.StatusOK, `<html>Cloudflare Access</html>`), nil
	})}

	secureStore := companionruntime.NewMemorySecureStore()
	errorValue := runPairWithStore([]string{"--device-url", "https://device.example.test", "--code", "ABCD-1234", "--state", statePath}, httpClient, secureStore)
	if errorValue == nil {
		t.Fatal("expected pair to fail")
	}
	if !strings.Contains(errorValue.Error(), "returned HTML instead of JSON") {
		t.Fatalf("expected helpful HTML response error, got %v", errorValue)
	}
}

func TestRunOnceCompletesMockLLMJob(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, true, true)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	seenComplete := false
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/_internkim/companion/heartbeat":
			return textResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textResponse(http.StatusOK, `{"jobID":"job-1","status":"running","request":{"toolName":"llm.structured","input":{"structuredOutputSchema":{"document":{"required":["reply"]}}}}}`), nil
		case "/_internkim/companion/jobs/job-1/complete":
			seenComplete = true
			return textResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected run path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	errorValue := runCompanionWithStore([]string{"--state", statePath, "--once", "--dev-mock-llm"}, httpClient, secureStore)
	if errorValue != nil {
		t.Fatalf("expected run once success: %v", errorValue)
	}
	if !seenComplete {
		t.Fatal("expected companion to complete the job")
	}
}

func TestRunOnceCompletesShellBridgeConfirmJob(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, true, false)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	seenComplete := false
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v1/user/confirm":
			return textResponse(http.StatusOK, `{"confirmed":true}`), nil
		case "/_internkim/companion/heartbeat":
			return textResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textResponse(http.StatusOK, `{"jobID":"job-1","status":"running","request":{"toolName":"user_confirm","input":{"message":"continue?"}}}`), nil
		case "/_internkim/companion/jobs/job-1/complete":
			seenComplete = true
			return textResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected run path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	errorValue := runCompanionWithStore([]string{"--state", statePath, "--once", "--shell-bridge-url", "http://127.0.0.1:1234"}, httpClient, secureStore)
	if errorValue != nil {
		t.Fatalf("expected run once success: %v", errorValue)
	}
	if !seenComplete {
		t.Fatal("expected companion to complete the bridge job")
	}
}

func TestDefaultBrowserExecutablePathUsesEnvironmentOverride(t *testing.T) {
	executablePath := filepath.Join(t.TempDir(), "chrome")
	if errorValue := os.WriteFile(executablePath, []byte("#!/bin/sh\n"), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Setenv("INTERNKIM_BROWSER_EXECUTABLE_PATH", executablePath)
	t.Setenv("AGENT_BROWSER_EXECUTABLE_PATH", "")

	if actual := defaultBrowserExecutablePath(); actual != executablePath {
		t.Fatalf("browser executable path = %q, want %q", actual, executablePath)
	}
}

func TestRunOnceDeniesBrowserJobWithReason(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, true, false)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	seenDeny := false
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v1/security/approval":
			return textResponse(http.StatusOK, `{"allowed":false,"userReason":"not now","suggestedConstraint":"ask for text"}`), nil
		case "/_internkim/companion/heartbeat":
			return textResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textResponse(http.StatusOK, `{"jobID":"job-1","status":"running","toolName":"browser_navigate","resourceScope":{"kind":"web_origin","value":"https://github.com"},"request":{"toolName":"browser_navigate","input":{"url":"https://github.com"}}}`), nil
		case "/_internkim/companion/jobs/job-1/deny":
			seenDeny = true
			return textResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected run path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	errorValue := runCompanionWithStore([]string{"--state", statePath, "--once", "--shell-bridge-url", "http://127.0.0.1:1234"}, httpClient, secureStore)
	if errorValue == nil {
		t.Fatal("expected denied browser job to return denial error")
	}
	if !seenDeny {
		t.Fatal("expected companion to send a denial result")
	}
}

func TestRunOnceCancelsApprovalAtJobExpiry(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, true, false)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	seenFail := false
	seenApprovalTimeout := false
	expiresAt := time.Now().UTC().Add(1200 * time.Millisecond).Format(time.RFC3339Nano)
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v1/security/approval":
			var payload companionruntime.ApprovalRequest
			if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
				t.Fatalf("expected approval request: %v", errorValue)
			}
			if payload.TimeoutSeconds < 1 || payload.TimeoutSeconds > 2 {
				t.Fatalf("expected job-scoped approval timeout, got %+v", payload)
			}
			select {
			case <-request.Context().Done():
				seenApprovalTimeout = true
				return nil, request.Context().Err()
			case <-time.After(3 * time.Second):
				t.Fatal("approval request was not canceled by job expiry")
				return nil, nil
			}
		case "/_internkim/companion/heartbeat":
			return textResponse(http.StatusOK, `{}`), nil
		case "/_internkim/companion/jobs/next":
			return textResponse(http.StatusOK, `{"jobID":"job-1","status":"running","toolName":"browser_open","resourceScope":{"kind":"web_origin","value":"https://github.com"},"expiresAt":"`+expiresAt+`","request":{"toolName":"browser_open","input":{"url":"https://github.com"},"resourceScope":{"kind":"web_origin","value":"https://github.com"}}}`), nil
		case "/_internkim/companion/jobs/job-1/fail":
			seenFail = true
			return textResponse(http.StatusOK, `{}`), nil
		default:
			t.Fatalf("unexpected run path: %s", request.URL.Path)
			return nil, nil
		}
	})}

	startedAt := time.Now()
	errorValue := runCompanionWithStore([]string{"--state", statePath, "--once", "--shell-bridge-url", "http://127.0.0.1:1234"}, httpClient, secureStore)

	if errorValue == nil {
		t.Fatal("expected approval cancellation error")
	}
	if time.Since(startedAt) > 2500*time.Millisecond {
		t.Fatalf("expected run loop to unblock near job expiry, took %s", time.Since(startedAt))
	}
	if !seenApprovalTimeout || !seenFail {
		t.Fatalf("expected approval timeout and failed job, seenApprovalTimeout=%v seenFail=%v", seenApprovalTimeout, seenFail)
	}
}

func TestLegacyPrivateKeyStateMigratesToSecureStore(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	state := companionruntime.State{
		DeviceURL:    "https://device.example.test",
		CompanionID:  "companion-1",
		Token:        "token-1",
		PublicKey:    keyPair.PublicKey,
		PrivateKey:   keyPair.PrivateKey,
		LocalOnly:    true,
		Capabilities: companionruntime.DefaultCapabilities(true, false),
	}
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	secureStore := companionruntime.NewMemorySecureStore()

	migratedState, errorValue := loadStateAndMigrateSecrets(nilContext(), statePath, secureStore)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if migratedState.PrivateKey != "" || migratedState.PrivateKeyID == "" {
		t.Fatalf("expected migrated state without raw private key: %+v", migratedState)
	}
	storedPrivateKey, errorValue := secureStore.Get(nilContext(), migratedState.PrivateKeyID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if storedPrivateKey != keyPair.PrivateKey {
		t.Fatal("expected secure store to contain migrated private key")
	}
	reloadedState, errorValue := loadState(statePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if reloadedState.PrivateKey != "" {
		t.Fatal("expected saved state to remove legacy private key")
	}
}

func TestControlHandlerListsAndRevokesGrants(t *testing.T) {
	grantStore := companionruntime.NewMemoryGrantStore()
	approvalHandler := companionruntime.ApprovalHandler(companionApprovalHandler{allowed: true})
	request := capabilities.ToolInvokeRequest{
		ToolName:      "browser_navigate",
		ResourceScope: capabilities.ResourceScope{Kind: "web_origin", Value: "https://github.com"},
	}
	if errorValue := grantStore.Authorize(context.Background(), companionruntime.JobEnvelope{
		JobID:         "job-1",
		ToolName:      "browser_navigate",
		ResourceScope: request.ResourceScope,
	}, request, approvalHandler); errorValue != nil {
		t.Fatal(errorValue)
	}
	runtimeStatus := &runtimeState{}
	runtimeStatus.recordHeartbeat(nil)
	handler := controlHandler(grantStore, companionruntime.NewMountStore(""), companionruntime.NewBrowserHandoffStore(), nil, nil, runtimeStatus, newDynamicLocalLLM(localLLMSettings{}), http.DefaultClient)

	listRequest := httptest.NewRequest(http.MethodGet, "/v1/security/grants", nil)
	listRequest.RemoteAddr = "127.0.0.1:1234"
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("expected grant list success, got %d", listResponse.Code)
	}
	var listDocument grantListDocument
	if errorValue := json.Unmarshal(listResponse.Body.Bytes(), &listDocument); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(listDocument.Grants) != 1 {
		t.Fatalf("expected one grant, got %d", len(listDocument.Grants))
	}

	runtimeRequest := httptest.NewRequest(http.MethodGet, "/v1/runtime/status", nil)
	runtimeRequest.RemoteAddr = "127.0.0.1:1234"
	runtimeResponse := httptest.NewRecorder()
	handler.ServeHTTP(runtimeResponse, runtimeRequest)
	if runtimeResponse.Code != http.StatusOK || !strings.Contains(runtimeResponse.Body.String(), "lastHeartbeatAt") {
		t.Fatalf("expected runtime status, got %d %s", runtimeResponse.Code, runtimeResponse.Body.String())
	}

	revokeRequest := httptest.NewRequest(http.MethodPost, "/v1/security/grants/"+listDocument.Grants[0].GrantID+"/revoke", nil)
	revokeRequest.RemoteAddr = "127.0.0.1:1234"
	revokeResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokeResponse, revokeRequest)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("expected revoke success, got %d", revokeResponse.Code)
	}
	if len(grantStore.ListActive()) != 0 {
		t.Fatal("expected revoked grant to be inactive")
	}
}

func TestControlHandlerCompletesBrowserHandoff(t *testing.T) {
	handoffStore := companionruntime.NewBrowserHandoffStore()
	handoff, errorValue := handoffStore.Begin(companionruntime.BrowserHandoffRequest{URL: "https://example.com/login", Message: "done?"}, "internkim")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	handler := controlHandler(
		companionruntime.NewMemoryGrantStore(),
		companionruntime.NewMountStore(""),
		handoffStore,
		nil,
		nil,
		&runtimeState{},
		newDynamicLocalLLM(localLLMSettings{}),
		http.DefaultClient,
	)

	remoteRequest := httptest.NewRequest(http.MethodGet, "/v1/browser/handoff", nil)
	remoteRequest.RemoteAddr = "198.51.100.10:1234"
	remoteResponse := httptest.NewRecorder()
	handler.ServeHTTP(remoteResponse, remoteRequest)
	if remoteResponse.Code != http.StatusForbidden {
		t.Fatalf("expected remote bridge request to fail, got %d", remoteResponse.Code)
	}

	completeRequest := httptest.NewRequest(http.MethodPost, "/v1/browser/handoff/complete", strings.NewReader(`{"handoffID":"`+handoff.HandoffID+`","sessionID":"internkim","url":"https://example.com/app"}`))
	completeRequest.RemoteAddr = "127.0.0.1:1234"
	completeResponse := httptest.NewRecorder()
	handler.ServeHTTP(completeResponse, completeRequest)
	if completeResponse.Code != http.StatusOK {
		t.Fatalf("expected complete success, got %d %s", completeResponse.Code, completeResponse.Body.String())
	}

	wrongRequest := httptest.NewRequest(http.MethodPost, "/v1/browser/handoff/complete", strings.NewReader(`{"handoffID":"wrong","sessionID":"internkim","url":"https://example.com/app"}`))
	wrongRequest.RemoteAddr = "127.0.0.1:1234"
	wrongResponse := httptest.NewRecorder()
	handler.ServeHTTP(wrongResponse, wrongRequest)
	if wrongResponse.Code != http.StatusForbidden {
		t.Fatalf("expected wrong handoff to fail, got %d", wrongResponse.Code)
	}
}

func TestControlHandlerUpdatesLocalLLMWithoutRestart(t *testing.T) {
	modelServer := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/tags" {
			t.Fatalf("expected ollama tags request, got %s", request.URL.Path)
		}
		responseWriter.WriteHeader(http.StatusOK)
		_, _ = responseWriter.Write([]byte(`{"models":[{"name":"gemma3:1b"}]}`))
	}))
	defer modelServer.Close()

	runtimeStatus := &runtimeState{}
	localLLM := newDynamicLocalLLM(localLLMSettings{})
	handler := controlHandler(
		companionruntime.NewMemoryGrantStore(),
		companionruntime.NewMountStore(""),
		companionruntime.NewBrowserHandoffStore(),
		nil,
		nil,
		runtimeStatus,
		localLLM,
		modelServer.Client(),
	)
	body := `{"enableLocalLLM":true,"localBackendOrder":["ollama"],"ollama":{"baseURL":"` + modelServer.URL + `","model":"gemma3:1b"},"llamacpp":{"baseURL":"","model":""},"mlx":{"baseURL":"","model":""}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/runtime/local-llm", strings.NewReader(body))
	request.RemoteAddr = "127.0.0.1:1234"
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected local LLM update success, got %d body=%s", response.Code, response.Body.String())
	}
	settings := localLLM.currentSettings()
	if !settings.Enabled || settings.Configuration.OllamaModel != "gemma3:1b" {
		t.Fatalf("expected live ollama model update, got %+v", settings.Configuration)
	}
	var status localLLMStatus
	if errorValue := json.Unmarshal(response.Body.Bytes(), &status); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(status.Backends) != 1 || status.Backends[0].Model != "gemma3:1b" || !status.Backends[0].Available {
		t.Fatalf("expected ready ollama status, got %+v", status)
	}
}

func TestStatusRequiresPairedState(t *testing.T) {
	errorValue := runStatus([]string{"--state", filepath.Join(t.TempDir(), "missing.json")}, http.DefaultClient, companionruntime.NewMemorySecureStore())
	if errorValue == nil {
		t.Fatal("expected missing state to fail")
	}
}

func TestStatusJSONReportsUnpairedState(t *testing.T) {
	output := captureStdout(t, func() {
		errorValue := runStatus([]string{"--state", filepath.Join(t.TempDir(), "missing.json"), "--json", "--verify-auth"}, http.DefaultClient, companionruntime.NewMemorySecureStore())
		if errorValue != nil {
			t.Fatalf("expected missing JSON status to succeed: %v", errorValue)
		}
	})
	var document companionStatusDocument
	if errorValue := json.Unmarshal([]byte(output), &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document.AuthStatus != companionAuthStatusUnpaired {
		t.Fatalf("expected unpaired auth status, got %+v", document)
	}
}

func TestStatusJSONReportsMissingSigningKey(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, _ := testCompanionState(t, false, false)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	output := captureStdout(t, func() {
		errorValue := runStatus([]string{"--state", statePath, "--json", "--verify-auth"}, http.DefaultClient, companionruntime.NewMemorySecureStore())
		if errorValue != nil {
			t.Fatalf("expected missing key JSON status to succeed: %v", errorValue)
		}
	})
	var document companionStatusDocument
	if errorValue := json.Unmarshal([]byte(output), &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document.AuthStatus != companionAuthStatusMissingSigningKey {
		t.Fatalf("expected missing signing key auth status, got %+v", document)
	}
}

func TestStatusJSONReportsReconnectRequired(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, false, false)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/_internkim/companion/auth/check" {
			t.Fatalf("unexpected auth check path: %s", request.URL.Path)
		}
		return textResponse(http.StatusForbidden, "companion auth required\n"), nil
	})}
	output := captureStdout(t, func() {
		errorValue := runStatus([]string{"--state", statePath, "--json", "--verify-auth"}, httpClient, secureStore)
		if errorValue != nil {
			t.Fatalf("expected stale auth JSON status to succeed: %v", errorValue)
		}
	})
	var document companionStatusDocument
	if errorValue := json.Unmarshal([]byte(output), &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document.AuthStatus != companionAuthStatusReconnectRequired {
		t.Fatalf("expected reconnect required auth status, got %+v", document)
	}
}

func TestStatusJSONReportsVerifiedAuth(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, false, false)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/_internkim/companion/auth/check" {
			t.Fatalf("unexpected auth check path: %s", request.URL.Path)
		}
		if request.Header.Get(companionruntime.SignatureHeader) == "" {
			t.Fatal("expected signed auth check request")
		}
		return textResponse(http.StatusOK, `{"status":"ok","companionID":"companion-1"}`), nil
	})}
	output := captureStdout(t, func() {
		errorValue := runStatus([]string{"--state", statePath, "--json", "--verify-auth"}, httpClient, secureStore)
		if errorValue != nil {
			t.Fatalf("expected verified JSON status to succeed: %v", errorValue)
		}
	})
	var document companionStatusDocument
	if errorValue := json.Unmarshal([]byte(output), &document); errorValue != nil {
		t.Fatal(errorValue)
	}
	if document.AuthStatus != companionAuthStatusVerified {
		t.Fatalf("expected verified auth status, got %+v", document)
	}
}

func TestDisconnectRevokesRemoteAndClearsLocalPairing(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	state, secureStore := testCompanionState(t, false, false)
	if errorValue := saveState(statePath, state); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(defaultMountStatePath(statePath), []byte("{}"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(defaultHandoffStatePath(statePath), []byte("{}"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	var requestedPath string
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		requestedPath = request.URL.Path
		if request.Header.Get(companionruntime.SignatureHeader) == "" {
			t.Fatal("expected signed disconnect request")
		}
		return textResponse(http.StatusOK, `{"status":"disconnected"}`), nil
	})}

	errorValue := runDisconnect([]string{"--state", statePath}, httpClient, secureStore)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if requestedPath != "/_internkim/companion/disconnect" {
		t.Fatalf("unexpected disconnect path: %s", requestedPath)
	}
	if _, errorValue := os.Stat(statePath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("expected state file to be removed, got %v", errorValue)
	}
	if _, errorValue := os.Stat(defaultMountStatePath(statePath)); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("expected mount state to be removed, got %v", errorValue)
	}
	if _, errorValue := os.Stat(defaultHandoffStatePath(statePath)); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("expected handoff state to be removed, got %v", errorValue)
	}
	if _, errorValue := secureStore.Get(nilContext(), state.PrivateKeyID); errorValue == nil {
		t.Fatal("expected private key to be removed")
	}
}

func TestRemoteModelAuthErrorsUseReconnectMessage(t *testing.T) {
	errorValue := companionruntime.DecodeJSONResponse("https://device.example.test/_internkim/runtime/remote-model", textResponse(http.StatusForbidden, "companion auth required\n"), nil)
	if errorValue != nil {
		if !strings.Contains(errorValue.Error(), companionruntime.CompanionPairingExpiredMessage) {
			t.Fatalf("expected pairing expired message, got %v", errorValue)
		}
		return
	}
	t.Fatal("expected auth error")
}

func TestCompanionStatusFromState(t *testing.T) {
	state := companionruntime.State{
		DeviceURL:   "https://device.example.test",
		CompanionID: "companion-1",
		Token:       "token-1",
		LocalOnly:   true,
	}

	document, errorValue := json.Marshal(companionStatusFromState(state, browserruntime.RuntimeReadiness{Status: "ready"}, companionAuthStatusVerified))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(document), `"paired":true`) {
		t.Fatalf("expected paired JSON, got %s", string(document))
	}
	if strings.Contains(string(document), "privateKey") || strings.Contains(string(document), "token-1") {
		t.Fatalf("expected status JSON to omit secrets, got %s", string(document))
	}
}

func TestCompanionStatusFiltersBrowserCapabilitiesWhenRuntimeUnavailable(t *testing.T) {
	state := companionruntime.State{
		DeviceURL:    "https://device.example.test",
		CompanionID:  "companion-1",
		Token:        "token-1",
		Capabilities: companionruntime.DefaultCapabilities(false, false),
	}

	document := companionStatusFromState(state, browserruntime.RuntimeReadiness{
		Status: "unavailable",
		Error:  "companion browser runtime unavailable",
	}, companionAuthStatusVerified)

	if hasCapability(document.Capabilities, "browser_navigate") {
		t.Fatal("expected browser capabilities to be hidden when runtime is unavailable")
	}
	if document.ExtensionAutomationStatus != "unavailable" {
		t.Fatalf("unexpected browser runtime status: %s", document.ExtensionAutomationStatus)
	}
	if strings.Contains(document.ExtensionAutomationError, "token-1") {
		t.Fatalf("expected sanitized browser runtime error, got %s", document.ExtensionAutomationError)
	}
}

func TestResolveAgentBrowserPathUsesFlagBeforeEnvironment(t *testing.T) {
	t.Setenv("INTERNKIM_AGENT_BROWSER_PATH", "/env/agent-browser")

	path := resolveAgentBrowserPath("/flag/agent-browser")

	if path != "/flag/agent-browser" {
		t.Fatalf("expected flag path, got %s", path)
	}
}

func TestResolveAgentBrowserPathUsesEnvironmentWhenFlagEmpty(t *testing.T) {
	t.Setenv("INTERNKIM_AGENT_BROWSER_PATH", "/env/agent-browser")

	path := resolveAgentBrowserPath("")

	if path != "/env/agent-browser" {
		t.Fatalf("expected environment path, got %s", path)
	}
}

func testCompanionState(t *testing.T, localOnly bool, devMockLLM bool) (companionruntime.State, *companionruntime.MemorySecureStore) {
	t.Helper()
	keyPair, errorValue := companionruntime.GenerateKeyPair()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	secureStore := companionruntime.NewMemorySecureStore()
	privateKeyID := companionPrivateKeyID("companion-1")
	if errorValue := secureStore.Put(nilContext(), privateKeyID, keyPair.PrivateKey); errorValue != nil {
		t.Fatal(errorValue)
	}
	return companionruntime.State{
		DeviceURL:    "https://device.example.test",
		CompanionID:  "companion-1",
		Token:        "token-1",
		PublicKey:    keyPair.PublicKey,
		PrivateKeyID: privateKeyID,
		LocalOnly:    localOnly,
		Capabilities: companionruntime.DefaultCapabilities(localOnly, devMockLLM),
	}, secureStore
}

func nilContext() context.Context {
	return context.Background()
}

type companionApprovalHandler struct {
	allowed bool
}

func (handler companionApprovalHandler) Approve(ctx context.Context, request companionruntime.ApprovalRequest) (companionruntime.ApprovalDecision, error) {
	_ = ctx
	_ = request
	return companionruntime.ApprovalDecision{Allowed: handler.allowed}, nil
}

func textResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func captureStdout(t *testing.T, function func()) string {
	t.Helper()
	originalStdout := os.Stdout
	reader, writer, errorValue := os.Pipe()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	os.Stdout = writer
	defer func() {
		os.Stdout = originalStdout
	}()
	function()
	if errorValue := writer.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	output, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(output)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
