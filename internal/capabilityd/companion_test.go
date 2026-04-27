package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/anthropic-lab/internkim/internal/capabilities"
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
			ConstraintMode:  "prompt_validation",
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
	_, errorValue := service.providerForExecutionMode(capabilities.ExecutionModeRemote)
	if errorValue == nil {
		t.Fatal("expected remote execution to fail in local-only mode")
	}
}

func TestCapabilityRouterUsesDescriptors(t *testing.T) {
	router := CapabilityRouter{
		CompanionAvailable: true,
		Descriptors:        capabilities.CompanionToolDescriptors(),
	}
	if !router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "browser.navigate"}) {
		t.Fatal("expected browser tool to route to companion")
	}
	if router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "unknown.tool"}) {
		t.Fatal("expected unknown tool to stay unconfigured")
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
