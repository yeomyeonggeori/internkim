package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

const testCompanionBackend = capabilities.LLMBackendCompanionLocal

func TestCapabilitiesReportCompanionStatus(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/capabilities" {
			t.Fatalf("unexpected companion path: %s", request.URL.Path)
		}
		return jsonResponse(capabilities.RegistryResponse{
			Capabilities: []capabilities.Descriptor{{
				Name:                 "llm.structured",
				Version:              "1",
				PrivacyClass:         "model_input",
				EstimatedLatency:     "low",
				RequiresUserPresence: false,
				WorksOffline:         true,
			}},
		}), nil
	})}

	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test", LocalOnly: true},
		HTTPClient:    httpClient,
	}
	response, errorValue := service.capabilityRegistry(context.Background())
	if errorValue != nil {
		t.Fatalf("expected capability registry: %v", errorValue)
	}

	if response.CompanionStatus != "available" {
		t.Fatalf("expected available companion, got %q", response.CompanionStatus)
	}
	if !response.LocalOnly {
		t.Fatal("expected local-only mode to be reported")
	}
	if len(response.CompanionCapabilities) != 1 || response.CompanionCapabilities[0].Name != "llm.structured" {
		t.Fatalf("unexpected companion capabilities: %+v", response.CompanionCapabilities)
	}
}

func TestCompanionStructuredProviderUsesSharedPrototype(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/jobs" {
			t.Fatalf("unexpected companion path: %s", request.URL.Path)
		}
		var toolRequest capabilities.ToolInvokeRequest
		if errorValue := json.NewDecoder(request.Body).Decode(&toolRequest); errorValue != nil {
			t.Fatalf("expected tool request: %v", errorValue)
		}
		if toolRequest.ToolName != "llm.structured" {
			t.Fatalf("expected structured tool, got %q", toolRequest.ToolName)
		}
		result, _ := json.Marshal(LLMResponse{
			Provider:        "companion",
			Model:           "local-model",
			Content:         `{"reply":"ok"}`,
			SelectedBackend: testCompanionBackend,
			ConstraintMode:  "openai_json_schema",
		})
		return jsonResponse(capabilities.ToolInvokeResponse{ToolName: "llm.structured", Result: result}), nil
	})}

	response, errorValue := (companionProvider{
		BaseURL:    "https://companion.test",
		HTTPClient: httpClient,
	}).CompleteStructured(context.Background(), StructuredLLMRequest{
		Model: "local-model",
		StructuredOutputSchema: StructuredOutputSchema{
			Name:     "reply",
			Document: json.RawMessage(`{"type":"object","required":["reply"]}`),
		},
	})
	if errorValue != nil {
		t.Fatalf("expected companion structured response: %v", errorValue)
	}
	if response.SelectedBackend != testCompanionBackend {
		t.Fatalf("expected companion local backend, got %q", response.SelectedBackend)
	}
}

func TestAutoProviderPrefersCompanionWhenConfigured(t *testing.T) {
	service := Service{Configuration: Configuration{PreferCompanionLLM: true}}
	providers := service.automaticLLMProviders(
		staticLLMProvider{response: LLMResponse{Provider: "litert", SelectedBackend: "cpu", Content: `{"reply":"device"}`}},
		staticLLMProvider{response: LLMResponse{Provider: "companion", SelectedBackend: testCompanionBackend, Content: `{"reply":"ok"}`}},
		staticLLMProvider{response: LLMResponse{Provider: "openrouter", SelectedBackend: capabilities.LLMBackendRemote, Content: `{"reply":"remote"}`}},
	)
	autoProvider := AutoProvider{Providers: providers}

	response, errorValue := autoProvider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue != nil {
		t.Fatalf("expected companion first response: %v", errorValue)
	}
	if response.SelectedBackend != testCompanionBackend {
		t.Fatalf("expected companion backend, got %q", response.SelectedBackend)
	}
}

func TestAutoProviderDefaultsToRemoteBeforeLocalFallback(t *testing.T) {
	service := Service{}
	providers := service.automaticLLMProviders(
		staticLLMProvider{response: LLMResponse{Provider: "litert", SelectedBackend: "cpu", Content: `{"reply":"device"}`}},
		staticLLMProvider{response: LLMResponse{Provider: "companion", SelectedBackend: testCompanionBackend, Content: `{"reply":"companion"}`}},
		staticLLMProvider{response: LLMResponse{Provider: "openrouter", SelectedBackend: capabilities.LLMBackendRemote, Content: `{"reply":"remote"}`}},
	)
	autoProvider := AutoProvider{Providers: providers}

	response, errorValue := autoProvider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue != nil {
		t.Fatalf("expected remote first response: %v", errorValue)
	}
	if response.SelectedBackend != capabilities.LLMBackendRemote {
		t.Fatalf("expected remote backend first, got %q", response.SelectedBackend)
	}
}

func TestAutoProviderLocalOnlyBlocksRemoteFallback(t *testing.T) {
	service := Service{Configuration: Configuration{LocalOnly: true}}
	providers := service.automaticLLMProviders(
		staticLLMProvider{errorValue: errTestProviderUnavailable},
		staticLLMProvider{errorValue: errTestProviderUnavailable},
		staticLLMProvider{response: LLMResponse{Provider: "openrouter", SelectedBackend: capabilities.LLMBackendRemote}},
	)
	autoProvider := AutoProvider{Providers: providers}
	response, errorValue := autoProvider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	_ = response
	if errorValue == nil {
		t.Fatal("expected local-only auto provider to avoid remote fallback")
	}
}

func TestRemoteExecutionFailsInLocalOnlyMode(t *testing.T) {
	service := Service{Configuration: Configuration{LocalOnly: true}}
	_, errorValue := service.providerForExecutionMode(capabilities.ExecutionModeRemote, "", "")
	if errorValue == nil {
		t.Fatal("expected remote execution to fail in local-only mode")
	}
}

func TestCapabilityRouterUsesDescriptors(t *testing.T) {
	router := CapabilityRouter{
		CompanionAvailable: true,
		Descriptors:        capabilities.CompanionToolDescriptors(),
	}
	if !router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "browser.open"}) {
		t.Fatal("expected browser tool to route to companion")
	}
	if router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "unknown.tool"}) {
		t.Fatal("expected unknown tool to stay unconfigured")
	}
}

func TestPreferCompanionBrowserRoutesGenericBrowserTool(t *testing.T) {
	router := CapabilityRouter{
		CompanionAvailable:     true,
		PreferCompanionBrowser: true,
		Descriptors:            []capabilities.Descriptor{},
	}
	if !router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "browser.navigate"}) {
		t.Fatal("expected browser tool to route to companion when PreferCompanionBrowser is true")
	}
	if router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "flow.task.add"}) {
		t.Fatal("expected non-browser tool to stay device-side")
	}
}

func TestPreferCompanionBrowserDoesNotRouteWithoutCompanionAvailable(t *testing.T) {
	router := CapabilityRouter{
		CompanionAvailable:     false,
		PreferCompanionBrowser: true,
		Descriptors:            []capabilities.Descriptor{},
	}
	if router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "browser.navigate"}) {
		t.Fatal("expected device fallback when companion is unavailable")
	}
}

func TestScreenshotDoesNotFallbackWhenCompanionUnavailable(t *testing.T) {
	commandWasCalled := false
	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		})},
		RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
			commandWasCalled = true
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.screenshot", strings.NewReader(`{"input":{}}`))
	if errorValue != nil {
		t.Fatalf("expected structured screenshot denial: %v", errorValue)
	}
	if !response.IsError || response.Status != "denied" || !strings.Contains(string(response.Result), capabilities.CapabilityNotConnected) {
		t.Fatalf("expected companion-required denial, got %+v result=%s", response, response.Result)
	}
	if commandWasCalled {
		t.Fatal("expected screenshot not to fallback to device browser")
	}
}

func TestSimpleBrowserToolFallsBackWhenCompanionBrowserNotReady(t *testing.T) {
	commandWasCalled := false
	denialResult, _ := json.Marshal(capabilities.DenialResult{
		Status:     "denied",
		Code:       capabilities.CapabilityNotReady,
		ToolName:   "browser.open",
		UserReason: "Companion은 연결되어 있지만 브라우저 런타임이 준비되지 않았습니다.",
	})
	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/jobs" {
				t.Fatalf("unexpected companion path: %s", request.URL.Path)
			}
			return jsonResponse(capabilities.ToolInvokeResponse{
				Provider: "companion",
				ToolName: "browser.open",
				Status:   "denied",
				IsError:  true,
				Result:   denialResult,
			}), nil
		})},
		RunCommand: func(_ context.Context, _ string, commandArguments []string, _ []byte) ([]byte, error) {
			commandWasCalled = true
			if slices.Contains(commandArguments, "get") && slices.Contains(commandArguments, "url") {
				return []byte("https://example.com\n"), nil
			}
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.open", strings.NewReader(`{"input":{"url":"https://example.com"}}`))
	if errorValue != nil {
		t.Fatalf("expected device fallback response: %v", errorValue)
	}
	if response.Provider != "device" || !commandWasCalled {
		t.Fatalf("expected browser.open to fallback to device, got response=%+v commandWasCalled=%v", response, commandWasCalled)
	}
}

func TestSimpleBrowserToolFallsBackToDeviceWhenCompanionUnavailable(t *testing.T) {
	commandWasCalled := false
	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		})},
		RunCommand: func(_ context.Context, _ string, commandArguments []string, _ []byte) ([]byte, error) {
			commandWasCalled = true
			if slices.Contains(commandArguments, "get") && slices.Contains(commandArguments, "url") {
				return []byte("https://example.com\n"), nil
			}
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.open", strings.NewReader(`{"input":{"url":"https://example.com"}}`))
	if errorValue != nil {
		t.Fatalf("expected device fallback response: %v", errorValue)
	}
	if response.Provider != "device" || !commandWasCalled {
		t.Fatalf("expected browser.open to fallback to device, got response=%+v commandWasCalled=%v", response, commandWasCalled)
	}
}

func TestCompanionOnlyBrowserToolPreservesNotReadyDenial(t *testing.T) {
	commandWasCalled := false
	denialResult, _ := json.Marshal(capabilities.DenialResult{
		Status:     "denied",
		Code:       capabilities.CapabilityNotReady,
		ToolName:   "browser.open",
		UserReason: "Companion은 연결되어 있지만 브라우저 런타임이 준비되지 않았습니다.",
	})
	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return jsonResponse(capabilities.ToolInvokeResponse{
				Provider: "companion",
				ToolName: "browser.open",
				Status:   "denied",
				Content:  "Companion은 연결되어 있지만 브라우저 런타임이 준비되지 않았습니다.",
				IsError:  true,
				Result:   denialResult,
			}), nil
		})},
		RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
			commandWasCalled = true
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.open", strings.NewReader(`{
		"requiresUserPresence":true,
		"executionMode":"companion",
		"input":{"url":"https://console.cloud.google.com/apis/credentials"}
	}`))
	if errorValue != nil {
		t.Fatalf("expected structured not_ready denial: %v", errorValue)
	}
	if response.Status != "denied" || !strings.Contains(string(response.Result), capabilities.CapabilityNotReady) || strings.Contains(response.Content, "/connect") {
		t.Fatalf("expected not_ready denial without reconnect advice, got %+v", response)
	}
	if commandWasCalled {
		t.Fatal("expected companion-only browser.open not to fallback to device browser")
	}
}

func TestUserPresenceBrowserToolDoesNotFallbackToDeviceWhenCompanionUnavailable(t *testing.T) {
	commandWasCalled := false
	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, io.ErrUnexpectedEOF
		})},
		RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
			commandWasCalled = true
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.open", strings.NewReader(`{
		"requiresUserPresence":true,
		"executionMode":"companion",
		"input":{"url":"https://console.cloud.google.com/apis/credentials"}
	}`))

	if errorValue != nil {
		t.Fatalf("expected structured companion-required denial: %v", errorValue)
	}
	if !response.IsError || response.Status != "denied" || !strings.Contains(string(response.Result), capabilities.CapabilityNotConnected) || strings.Contains(response.Content, "/connect") {
		t.Fatalf("expected not_connected denial, got %+v", response)
	}
	if commandWasCalled {
		t.Fatal("expected user-presence browser.open not to fallback to device browser")
	}
}

func TestUserPresenceBrowserToolRequiresConnectWhenCompanionNotConfigured(t *testing.T) {
	commandWasCalled := false
	service := Service{RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
		commandWasCalled = true
		return nil, nil
	}}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.open", strings.NewReader(`{
		"requiresUserPresence":true,
		"executionMode":"companion",
		"input":{"url":"https://console.cloud.google.com/apis/credentials"}
	}`))

	if errorValue != nil {
		t.Fatalf("expected structured companion-required denial: %v", errorValue)
	}
	if !response.IsError || response.Status != "denied" || !strings.Contains(string(response.Result), capabilities.CapabilityNotConnected) {
		t.Fatalf("expected no-success denial, got %+v", response)
	}
	if commandWasCalled {
		t.Fatal("expected companion-only browser.open not to fallback to device browser")
	}
}

func TestBrowserToolUsesCompanionBeforeDeviceFallback(t *testing.T) {
	commandWasCalled := false
	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if request.URL.Path != "/jobs" {
				t.Fatalf("unexpected companion path: %s", request.URL.Path)
			}
			return jsonResponse(capabilities.ToolInvokeResponse{
				Provider: "companion",
				ToolName: "browser.open",
				Status:   "ok",
				Result:   json.RawMessage(`{"url":"https://example.com"}`),
			}), nil
		})},
		RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
			commandWasCalled = true
			return nil, nil
		},
	}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.open", strings.NewReader(`{"input":{"url":"https://example.com"}}`))
	if errorValue != nil {
		t.Fatalf("expected companion browser response: %v", errorValue)
	}
	if response.Provider != "companion" || commandWasCalled {
		t.Fatalf("expected browser.open to use companion only, got response=%+v commandWasCalled=%v", response, commandWasCalled)
	}
}

func TestBrowserHandoffRequiresCompanion(t *testing.T) {
	commandWasCalled := false
	service := Service{RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
		commandWasCalled = true
		return nil, nil
	}}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.handoff", strings.NewReader(`{"input":{"url":"https://example.com"}}`))
	if errorValue != nil {
		t.Fatalf("expected handoff denial: %v", errorValue)
	}
	if !response.IsError || response.Status != "denied" || !strings.Contains(string(response.Result), capabilities.CapabilityNotConnected) {
		t.Fatalf("expected companion-required handoff denial, got %+v result=%s", response, response.Result)
	}
	if commandWasCalled {
		t.Fatal("expected browser.handoff not to fallback to device browser")
	}
}

func TestHumanInputToolRoutesToCompanion(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/jobs" {
			t.Fatalf("unexpected companion path: %s", request.URL.Path)
		}
		var forwardedRequest capabilities.ToolInvokeRequest
		if errorValue := json.NewDecoder(request.Body).Decode(&forwardedRequest); errorValue != nil {
			t.Fatalf("expected forwarded tool request: %v", errorValue)
		}
		if forwardedRequest.ToolName != "user.confirm" {
			t.Fatalf("expected tool name to be forwarded, got %q", forwardedRequest.ToolName)
		}
		return jsonResponse(capabilities.ToolInvokeResponse{
			Provider:        "companion",
			SelectedBackend: testCompanionBackend,
			ToolName:        "user.confirm",
			Result:          json.RawMessage(`{"confirmed":true}`),
		}), nil
	})}

	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient:    httpClient,
	}
	response, errorValue := service.invokeCapabilityTool(context.Background(), "user.confirm", strings.NewReader(`{"requiresUserPresence":true,"input":{"message":"continue?"}}`))
	if errorValue != nil {
		t.Fatalf("expected companion tool response: %v", errorValue)
	}
	if response.SelectedBackend != testCompanionBackend {
		t.Fatalf("expected companion backend, got %q", response.SelectedBackend)
	}
}

func TestHumanInputToolFailsCleanlyWithoutCompanion(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-must-not-leak")
	service := Service{Configuration: DefaultConfiguration()}
	_, errorValue := service.invokeCapabilityTool(context.Background(), "user.confirm", strings.NewReader(`{"requiresUserPresence":true}`))
	if errorValue == nil {
		t.Fatal("expected missing companion to fail")
	}
	if strings.Contains(errorValue.Error(), "sk-must-not-leak") {
		t.Fatalf("expected error to omit secrets, got %q", errorValue.Error())
	}
}

func jsonResponse(response any) *http.Response {
	document, _ := json.Marshal(response)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(document))),
		Header:     make(http.Header),
	}
}
