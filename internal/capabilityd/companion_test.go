package capabilityd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
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
	expectedIdentity := capabilityprotocol.GeneratedProtocolIdentity()
	if response.ProtocolVersion != expectedIdentity.ProtocolVersion || response.AggregateProtocolHash != expectedIdentity.AggregateProtocolHash {
		t.Fatalf("expected generated protocol identity, got protocolVersion=%q aggregateProtocolHash=%q", response.ProtocolVersion, response.AggregateProtocolHash)
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

func TestCompanionStructuredProviderRejectsEmptyContent(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		result, _ := json.Marshal(LLMResponse{})
		return jsonResponse(capabilities.ToolInvokeResponse{ToolName: "llm.structured", Result: result}), nil
	})}

	_, errorValue := (companionProvider{
		BaseURL:    "https://companion.test",
		HTTPClient: httpClient,
	}).CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "empty or invalid json") {
		t.Fatalf("expected empty structured content error, got %v", errorValue)
	}
}

func TestDecodeToolInvokeRequestRequiresRequesterPersonIDForTrustedFlags(t *testing.T) {
	_, errorValue := decodeToolInvokeRequest("message.send", strings.NewReader(`{
		"context": {
			"isScheduledRun": true
		}
	}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "requesterPersonID is required") {
		t.Fatalf("expected requesterPersonID requirement, got %v", errorValue)
	}
}

func TestDecodeToolInvokeRequestRejectsReservedRequesterPersonID(t *testing.T) {
	_, errorValue := decodeToolInvokeRequest("message.send", strings.NewReader(`{
		"context": {
			"requesterPersonID": "blueclaw"
		}
	}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "requesterPersonID is invalid") {
		t.Fatalf("expected invalid requesterPersonID, got %v", errorValue)
	}
}

func TestDecodeToolInvokeRequestRejectsMalformedRequesterPersonID(t *testing.T) {
	_, errorValue := decodeToolInvokeRequest("message.send", strings.NewReader(`{
		"context": {
			"requesterPersonID": "../person-1"
		}
	}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "requesterPersonID is invalid") {
		t.Fatalf("expected invalid requesterPersonID, got %v", errorValue)
	}
}

func TestDecodeToolInvokeRequestAcceptsPlausibleRequesterPersonID(t *testing.T) {
	request, errorValue := decodeToolInvokeRequest("message.send", strings.NewReader(`{
		"context": {
			"requesterPersonID": " person-1 ",
			"isApprovalContinuation": true
		}
	}`))
	if errorValue != nil {
		t.Fatalf("expected valid requesterPersonID: %v", errorValue)
	}
	if request.Context.RequesterPersonID != "person-1" {
		t.Fatalf("expected trimmed requesterPersonID, got %q", request.Context.RequesterPersonID)
	}
}

func TestDecodeToolInvokeRequestRejectsOperationMismatch(t *testing.T) {
	_, errorValue := decodeToolInvokeRequest("site.status", strings.NewReader(`{
		"toolName": "site.delete",
		"input": {}
	}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "tool name mismatch") {
		t.Fatalf("expected URL/body operation mismatch, got %v", errorValue)
	}
}

func TestInvokeCapabilityToolRejectsInputOutsideDescriptorSchema(t *testing.T) {
	testCases := []struct {
		toolName string
		input    string
	}{
		{toolName: "site.publish", input: `{"siteID":42}`},
		{toolName: "site.publish", input: `{"siteID":" site-1 "}`},
		{toolName: "site.delete", input: `{"siteID":"site-1","confirm":"DELETE"}`},
		{toolName: "site.delete", input: `{"siteID":"site-1","userConfirmed":true}`},
	}
	for _, testCase := range testCases {
		response, errorValue := (Service{}).invokeCapabilityTool(
			context.Background(),
			testCase.toolName,
			strings.NewReader(`{"context":{"requesterPersonID":"person-1","isApprovalContinuation":true},"input":`+testCase.input+`}`),
		)
		if errorValue != nil {
			t.Fatalf("%s returned an unexpected error: %v", testCase.toolName, errorValue)
		}
		if !response.IsError || response.ErrorCode != "invalid_input" || response.FailureStage != "input_schema" {
			t.Fatalf("%s accepted input %s: %+v", testCase.toolName, testCase.input, response)
		}
	}
}

func TestCapabilityToolDescriptorRequiresExactCanonicalName(t *testing.T) {
	descriptor, found := capabilityToolDescriptorFor("site.delete")
	if !found {
		t.Fatal("expected site.delete descriptor")
	}
	if descriptor.CanonicalName != "site.delete" {
		t.Fatalf("expected canonical site.delete descriptor, got %+v", descriptor)
	}
	if descriptor.PolicyResource != "tool:site.delete" {
		t.Fatalf("expected descriptor policy resource, got %q", descriptor.PolicyResource)
	}
	if !descriptor.RequiresApproval {
		t.Fatal("expected site.delete descriptor to require approval")
	}
	if !descriptor.RequiresUserPresence {
		t.Fatal("expected site.delete descriptor to require user presence")
	}
	if _, found := capabilityToolDescriptorFor("site."); found {
		t.Fatal("expected prefix-only operation to have no descriptor")
	}
}

func TestInvokeCapabilityToolRejectsUnknownPrefixOperation(t *testing.T) {
	service := Service{}
	_, errorValue := service.invokeCapabilityTool(context.Background(), "company.unknown", strings.NewReader(`{"input":{}}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "capability tool is not configured") {
		t.Fatalf("expected unknown prefix operation to be rejected, got %v", errorValue)
	}
}

func TestCompanionStructuredProviderRejectsDeniedToolResponse(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		result, _ := json.Marshal(capabilities.DenialResult{
			Status:   "denied",
			Code:     capabilities.CapabilityNotConnected,
			ToolName: "llm.structured",
		})
		return jsonResponse(capabilities.ToolInvokeResponse{
			ToolName: "llm.structured",
			Status:   "denied",
			Content:  "Companion이 연결되어 있지 않습니다.",
			IsError:  true,
			Result:   result,
		}), nil
	})}

	_, errorValue := (companionProvider{
		BaseURL:    "https://companion.test",
		HTTPClient: httpClient,
	}).CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "Companion이 연결되어 있지 않습니다.") {
		t.Fatalf("expected companion denial error, got %v", errorValue)
	}
}

func TestCompanionStructuredProviderRejectsDeniedStatusWithoutErrorFlag(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		result, _ := json.Marshal(capabilities.DenialResult{
			Status:   "denied",
			Code:     capabilities.CapabilityNotAllowed,
			ToolName: "llm.structured",
		})
		return jsonResponse(capabilities.ToolInvokeResponse{
			ToolName: "llm.structured",
			Status:   "denied",
			Content:  "이 요청을 실행할 수 있는 Companion 권한이 없습니다.",
			Result:   result,
		}), nil
	})}

	_, errorValue := (companionProvider{
		BaseURL:    "https://companion.test",
		HTTPClient: httpClient,
	}).CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "Companion 권한") {
		t.Fatalf("expected companion denied status error, got %v", errorValue)
	}
}

func TestCompanionStructuredProviderRejectsEmptyResult(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return jsonResponse(capabilities.ToolInvokeResponse{ToolName: "llm.structured"}), nil
	})}

	_, errorValue := (companionProvider{
		BaseURL:    "https://companion.test",
		HTTPClient: httpClient,
	}).CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "result was empty") {
		t.Fatalf("expected empty result error, got %v", errorValue)
	}
}

func TestCompanionTextProviderRejectsEmptyContent(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		result, _ := json.Marshal(LLMResponse{})
		return jsonResponse(capabilities.ToolInvokeResponse{ToolName: "llm.text", Result: result}), nil
	})}

	_, errorValue := (companionProvider{
		BaseURL:    "https://companion.test",
		HTTPClient: httpClient,
	}).CompleteText(context.Background(), TextLLMRequest{})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "text llm response was empty") {
		t.Fatalf("expected empty text content error, got %v", errorValue)
	}
}

func TestAutoProviderPrefersCompanionWhenConfigured(t *testing.T) {
	service := Service{Configuration: Configuration{PreferCompanionLLM: true}}
	providers := service.automaticLLMProviders(
		staticLLMProvider{response: LLMResponse{Provider: "litert", SelectedBackend: "cpu", Content: `{"reply":"device"}`}},
		staticLLMProvider{response: LLMResponse{Provider: "companion", SelectedBackend: testCompanionBackend, Content: `{"reply":"ok"}`}},
		staticLLMProvider{response: LLMResponse{Provider: "openrouter", SelectedBackend: capabilities.LLMBackendRemote, Content: `{"reply":"remote"}`}},
	)
	autoProvider := AutoProvider{Providers: providers, AllowStructuredFallback: true}

	response, errorValue := autoProvider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue != nil {
		t.Fatalf("expected companion first response: %v", errorValue)
	}
	if response.SelectedBackend != testCompanionBackend {
		t.Fatalf("expected companion backend, got %q", response.SelectedBackend)
	}
}

func TestAutoProviderConfiguredCompanionFallsBackToRemote(t *testing.T) {
	service := Service{Configuration: Configuration{PreferCompanionLLM: true}}
	providers := service.automaticLLMProviders(
		staticLLMProvider{response: LLMResponse{Provider: "litert", SelectedBackend: "cpu", Content: `{"reply":"device"}`}},
		staticLLMProvider{errorValue: errTestProviderUnavailable},
		staticLLMProvider{response: LLMResponse{Provider: "openrouter", SelectedBackend: capabilities.LLMBackendRemote, Content: `{"reply":"remote"}`}},
	)
	autoProvider := AutoProvider{Providers: providers, AllowStructuredFallback: true}

	response, errorValue := autoProvider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue != nil {
		t.Fatalf("expected remote fallback response: %v", errorValue)
	}
	if response.SelectedBackend != capabilities.LLMBackendRemote {
		t.Fatalf("expected remote backend after companion failure, got %q", response.SelectedBackend)
	}
}

func TestAutoProviderConfiguredCompanionFallsBackToLocalAfterRemote(t *testing.T) {
	service := Service{Configuration: Configuration{PreferCompanionLLM: true}}
	providers := service.automaticLLMProviders(
		staticLLMProvider{response: LLMResponse{Provider: "litert", SelectedBackend: "cpu", Content: `{"reply":"device"}`}},
		staticLLMProvider{errorValue: errTestProviderUnavailable},
		staticLLMProvider{errorValue: errTestProviderUnavailable},
	)
	autoProvider := AutoProvider{Providers: providers, AllowStructuredFallback: true}

	response, errorValue := autoProvider.CompleteStructured(context.Background(), StructuredLLMRequest{})
	if errorValue != nil {
		t.Fatalf("expected local fallback response: %v", errorValue)
	}
	if response.SelectedBackend != "cpu" {
		t.Fatalf("expected local backend after companion and remote failure, got %q", response.SelectedBackend)
	}
}

func TestAutoProviderDefaultsToRemoteBeforeLocalFallback(t *testing.T) {
	service := Service{Configuration: Configuration{CompanionBaseURL: "https://companion.test"}}
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
	_, errorValue := service.providerForExecutionMode(context.Background(), "llm.structured", capabilities.ExecutionModeRemote, "", "")
	if errorValue == nil {
		t.Fatal("expected remote execution to fail in local-only mode")
	}
}

func TestAutoProviderSkipsDisconnectedCompanion(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/capabilities" {
			t.Fatalf("unexpected companion path: %s", request.URL.Path)
		}
		return jsonResponse(capabilities.RegistryResponse{CompanionStatus: "unavailable"}), nil
	})}

	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient:    httpClient,
	}
	if provider := service.companionLLMProviderForAuto(context.Background(), "llm.structured"); provider != nil {
		t.Fatalf("expected disconnected companion to be skipped, got %T", provider)
	}
}

func TestAutoProviderUsesConnectedCompanionWithStructuredCapability(t *testing.T) {
	httpClient := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/capabilities" {
			t.Fatalf("unexpected companion path: %s", request.URL.Path)
		}
		return jsonResponse(capabilities.RegistryResponse{
			CompanionStatus: "available",
			Capabilities: []capabilities.Descriptor{{
				Name: "llm.structured",
			}},
		}), nil
	})}

	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient:    httpClient,
	}
	if provider := service.companionLLMProviderForAuto(context.Background(), "llm.structured"); provider == nil {
		t.Fatal("expected connected structured companion provider")
	}
}

func TestCapabilityRouterUsesDescriptors(t *testing.T) {
	router := CapabilityRouter{
		CompanionAvailable: true,
		Descriptors:        capabilities.CompanionToolDescriptors(),
	}
	if router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "browser.open"}) {
		t.Fatal("expected browser tool to stay device-side")
	}
	if router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "unknown.tool"}) {
		t.Fatal("expected unknown tool to stay unconfigured")
	}
}

func TestInvokeCapabilityToolRequiresDescriptorApproval(t *testing.T) {
	service := Service{}
	toolNames := []string{
		"message.send",
		"message.update",
		"message.delete",
		"channel.update",
		"task.delete",
		"calendar.delete",
		"mail.connection.start",
		"mail.message.send",
		"site.delete",
		"google.gmail.send",
	}
	for _, toolName := range toolNames {
		t.Run(toolName, func(t *testing.T) {
			response, errorValue := service.invokeCapabilityTool(context.Background(), toolName, strings.NewReader(`{"input":{}}`))
			if errorValue != nil {
				t.Fatal(errorValue)
			}
			assertCapabilityApprovalRequired(t, response, toolName)
		})
	}
}

func TestInvokeCapabilityToolSiteDeleteInjectsInternalApprovalProof(t *testing.T) {
	service := Service{
		Configuration: Configuration{AdmindBaseURL: "http://admind.local"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			var input map[string]any
			if errorValue := json.NewDecoder(request.Body).Decode(&input); errorValue != nil {
				t.Fatal(errorValue)
			}
			if input["confirm"] != "DELETE" || input["userConfirmed"] != true {
				t.Fatalf("expected internal approval proof, got %+v", input)
			}
			return siteToolJSONResponse(`{"siteID":"site-1","status":"deleted"}`), nil
		})},
	}
	response, errorValue := service.invokeCapabilityTool(context.Background(), "site.delete", strings.NewReader(`{
		"input":{"siteID":"site-1","reason":"Remove obsolete launch page"},
		"context":{"requesterPersonID":"person-1","isApprovalContinuation":true}
	}`))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if response.Outcome != capabilities.ToolOutcomeSucceeded || string(response.Result) != `{"deleted":true,"siteID":"site-1"}` {
		t.Fatalf("unexpected delete response %+v", response)
	}
}

func TestMessageSendCurrentConversationApprovalGate(t *testing.T) {
	testCases := []struct {
		name             string
		input            string
		context          capabilities.ToolInvokeContext
		requiresApproval bool
	}{
		{
			name:             "directMessage still requires approval",
			input:            `{"targetType":"directMessage"}`,
			context:          capabilities.ToolInvokeContext{ConversationID: "conversation-1"},
			requiresApproval: true,
		},
		{
			name:             "currentThread in the originating conversation is pre-approved",
			input:            `{"targetType":"currentThread"}`,
			context:          capabilities.ToolInvokeContext{ConversationID: "conversation-1"},
			requiresApproval: false,
		},
		{
			name:             "currentChannel in the originating conversation is pre-approved",
			input:            `{"targetType":"currentChannel"}`,
			context:          capabilities.ToolInvokeContext{ConversationID: "conversation-1"},
			requiresApproval: false,
		},
		{
			name:             "currentThread without a trusted originating conversation still requires approval",
			input:            `{"targetType":"currentThread"}`,
			context:          capabilities.ToolInvokeContext{ConversationID: ""},
			requiresApproval: true,
		},
		{
			name:             "named channel still requires approval",
			input:            `{"targetType":"channel","channelName":"general"}`,
			context:          capabilities.ToolInvokeContext{ConversationID: "conversation-1"},
			requiresApproval: true,
		},
		{
			name:             "malformed input falls through to approval required",
			input:            `{"targetType":`,
			context:          capabilities.ToolInvokeContext{ConversationID: "conversation-1"},
			requiresApproval: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			request := capabilities.ToolInvokeRequest{
				ToolName: "message.send",
				Input:    json.RawMessage(testCase.input),
				Context:  testCase.context,
			}
			service := Service{}
			descriptor := descriptorForCapabilityToolTest(t, request.ToolName)
			response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptor)
			if isDenied != testCase.requiresApproval {
				t.Fatalf("expected requiresApproval=%v, got isDenied=%v response=%+v", testCase.requiresApproval, isDenied, response)
			}
			if isDenied {
				assertCapabilityApprovalRequired(t, response, "message.send")
			}
		})
	}
}

func TestMessageSendSelfDirectMessageApprovalGate(t *testing.T) {
	requesterContext := capabilities.ToolInvokeContext{
		RequesterPersonID: "person-dongha",
		ConversationID:    "conversation-1",
	}

	t.Run("directMessage resolving to the requester is pre-approved", func(t *testing.T) {
		service := platformDMResolverTestService(t, platformDMResolvedDonghaResponse())
		request := capabilities.ToolInvokeRequest{
			ToolName: "message.send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"동하"}`),
			Context:  requesterContext,
		}
		descriptor := descriptorForCapabilityToolTest(t, request.ToolName)
		response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptor)
		if isDenied {
			t.Fatalf("expected self direct message to be pre-approved, got %+v", response)
		}
	})

	t.Run("directMessage resolving to a different person still requires approval", func(t *testing.T) {
		service := platformDMResolverTestService(t, platformDMResolvedDonghaResponse())
		request := capabilities.ToolInvokeRequest{
			ToolName: "message.send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"동하"}`),
			Context:  capabilities.ToolInvokeContext{RequesterPersonID: "person-someone-else", ConversationID: "conversation-1"},
		}
		descriptor := descriptorForCapabilityToolTest(t, request.ToolName)
		response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptor)
		if !isDenied {
			t.Fatal("expected direct message to a different person to require approval")
		}
		assertCapabilityApprovalRequired(t, response, "message.send")
	})

	t.Run("directMessage broadcast with personHints still requires approval", func(t *testing.T) {
		service := platformDMResolverTestService(t, platformDMResolvedDonghaResponse())
		request := capabilities.ToolInvokeRequest{
			ToolName: "message.send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"동하","personHints":["동하"]}`),
			Context:  requesterContext,
		}
		descriptor := descriptorForCapabilityToolTest(t, request.ToolName)
		response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptor)
		if !isDenied {
			t.Fatal("expected multi-recipient directMessage to require approval")
		}
		assertCapabilityApprovalRequired(t, response, "message.send")
	})

	t.Run("recipient resolution failure still requires approval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {}))
		server.Close()
		service := Service{Configuration: Configuration{BlueclawBaseURL: server.URL}}
		request := capabilities.ToolInvokeRequest{
			ToolName: "message.send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"동하"}`),
			Context:  requesterContext,
		}
		descriptor := descriptorForCapabilityToolTest(t, request.ToolName)
		response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptor)
		if !isDenied {
			t.Fatal("expected resolution failure to require approval")
		}
		assertCapabilityApprovalRequired(t, response, "message.send")
	})

	t.Run("scheduled run never qualifies for the self direct-message pre-approval", func(t *testing.T) {
		service := platformDMResolverTestService(t, platformDMResolvedDonghaResponse())
		request := capabilities.ToolInvokeRequest{
			ToolName: "message.send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"동하"}`),
			Context:  capabilities.ToolInvokeContext{RequesterPersonID: "person-dongha", ConversationID: "conversation-1", IsScheduledRun: true},
		}
		if service.isPreApprovedSelfDirectMessageSend(context.Background(), request) {
			t.Fatal("expected scheduled runs to never qualify for the self direct-message pre-approval")
		}
	})
}

func TestMessageSendPreApprovalExcludesScheduledRuns(t *testing.T) {
	request := capabilities.ToolInvokeRequest{
		ToolName: "message.send",
		Input:    json.RawMessage(`{"targetType":"currentThread"}`),
		Context:  capabilities.ToolInvokeContext{ConversationID: "conversation-1", IsScheduledRun: true},
	}
	if isPreApprovedCurrentConversationMessageSend(request) {
		t.Fatal("expected scheduled runs to never qualify for the current-conversation pre-approval")
	}
}

func TestPreferCompanionBrowserRoutesGenericBrowserTool(t *testing.T) {
	router := CapabilityRouter{
		CompanionAvailable:     true,
		PreferCompanionBrowser: true,
		Descriptors:            []capabilities.Descriptor{},
	}
	if router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: "browser.navigate"}) {
		t.Fatal("expected browser tool to stay device-side regardless of PreferCompanionBrowser")
	}
	for _, toolName := range []string{"task.add", "task.list", "task.update", "task.delete"} {
		if router.ShouldRouteToCompanion(capabilities.ToolInvokeRequest{ToolName: toolName}) {
			t.Fatalf("expected %s to stay device-side", toolName)
		}
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
	companionWasCalled := false
	commandWasCalled := false
	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			companionWasCalled = true
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
		t.Fatalf("expected structured denial: %v", errorValue)
	}
	if response.Status != "denied" || !strings.Contains(string(response.Result), capabilities.CapabilityNotConnected) {
		t.Fatalf("expected not_connected denial, got %+v", response)
	}
	if companionWasCalled {
		t.Fatal("expected companion not to be called for browser tools")
	}
	if commandWasCalled {
		t.Fatal("expected device browser not to run for companion-mode request")
	}
}

func TestCompanionRequiredBrowserJobExpiryReportsNotReady(t *testing.T) {
	companionWasCalled := false
	commandWasCalled := false
	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			companionWasCalled = true
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader(`companion job expired`)),
				Header:     make(http.Header),
			}, nil
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
		t.Fatalf("expected structured denial: %v", errorValue)
	}
	var denial capabilities.DenialResult
	if errorValue := json.Unmarshal(response.Result, &denial); errorValue != nil {
		t.Fatal(errorValue)
	}
	if denial.Code != capabilities.CapabilityNotConnected || denial.Recovery != nil {
		t.Fatalf("expected not_connected denial without recovery, got response=%+v denial=%+v", response, denial)
	}
	if companionWasCalled {
		t.Fatal("expected companion not to be called for browser tools")
	}
	if commandWasCalled {
		t.Fatal("expected device browser not to run for companion-mode request")
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

func TestCompanionRequiredBrowserDenialIncludesConnectRecovery(t *testing.T) {
	service := Service{}

	response, errorValue := service.invokeCapabilityTool(context.Background(), "browser.open", strings.NewReader(`{
		"requiresUserPresence":true,
		"executionMode":"companion",
		"input":{"url":"https://example.com/login"}
	}`))

	if errorValue != nil {
		t.Fatalf("expected structured denial: %v", errorValue)
	}
	var denial capabilities.DenialResult
	if errorValue := json.Unmarshal(response.Result, &denial); errorValue != nil {
		t.Fatal(errorValue)
	}
	if denial.Code != capabilities.CapabilityNotConnected || denial.Recovery != nil {
		t.Fatalf("expected not_connected denial without recovery, got response=%+v denial=%+v", response, denial)
	}
}

func TestBrowserToolUsesCompanionBeforeDeviceFallback(t *testing.T) {
	companionWasCalled := false
	deviceCommandCalled := false
	service := Service{
		Configuration: Configuration{CompanionBaseURL: "https://companion.test"},
		HTTPClient: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			companionWasCalled = true
			return jsonResponse(capabilities.ToolInvokeResponse{
				Provider: "companion",
				ToolName: "browser.open",
				Status:   "ok",
				Result:   json.RawMessage(`{"url":"https://example.com"}`),
			}), nil
		})},
		RunCommand: func(_ context.Context, _ string, _ []string, _ []byte) ([]byte, error) {
			deviceCommandCalled = true
			return nil, errors.New("device browser not configured in test")
		},
	}

	service.invokeCapabilityTool(context.Background(), "browser.open", strings.NewReader(`{"input":{"url":"https://example.com"}}`))
	if companionWasCalled {
		t.Fatal("expected browser.open to use device-only path, not companion")
	}
	if !deviceCommandCalled {
		t.Fatal("expected device browser command to be called")
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
	_, errorValue := service.invokeCapabilityTool(context.Background(), "user.confirm", strings.NewReader(`{"requiresUserPresence":true,"input":{"message":"continue?"}}`))
	if errorValue == nil {
		t.Fatal("expected missing companion to fail")
	}
	if strings.Contains(errorValue.Error(), "sk-must-not-leak") {
		t.Fatalf("expected error to omit secrets, got %q", errorValue.Error())
	}
}

func assertCapabilityApprovalRequired(t *testing.T, response capabilities.ToolInvokeResponse, toolName string) {
	t.Helper()
	if response.Status != "denied" || response.Outcome != capabilities.ToolOutcomeDenied || !response.IsError {
		t.Fatalf("expected approval denial, got %+v", response)
	}
	if response.ToolName != toolName || response.ErrorCode != "approval_required" || response.FailureStage != "authorization" {
		t.Fatalf("unexpected approval response for %s: %+v", toolName, response)
	}
	var failure capabilityApprovalFailure
	if errorValue := json.Unmarshal(response.Result, &failure); errorValue != nil {
		t.Fatal(errorValue)
	}
	expectedMessage := toolName + " requires approval before execution"
	if failure.ErrorCode != response.ErrorCode || failure.FailureStage != response.FailureStage || failure.Message != expectedMessage {
		t.Fatalf("unexpected approval failure: %+v response=%+v", failure, response)
	}
}

func descriptorForCapabilityToolTest(t *testing.T, toolName string) capabilities.Descriptor {
	t.Helper()
	descriptor, found := capabilityToolDescriptorFor(toolName)
	if !found {
		t.Fatalf("expected descriptor for %s", toolName)
	}
	return descriptor
}

func jsonResponse(response any) *http.Response {
	document, _ := json.Marshal(response)
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(string(document))),
		Header:     make(http.Header),
	}
}
