package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

func TestDecodeToolInvokeRequestRequiresRequesterPersonIDForTrustedFlags(t *testing.T) {
	_, errorValue := decodeToolInvokeRequest("message_send", strings.NewReader(`{
		"context": {
			"isScheduledRun": true
		}
	}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "requesterPersonID is required") {
		t.Fatalf("expected requesterPersonID requirement, got %v", errorValue)
	}
}

func TestDecodeToolInvokeRequestRejectsReservedRequesterPersonID(t *testing.T) {
	_, errorValue := decodeToolInvokeRequest("message_send", strings.NewReader(`{
		"context": {
			"requesterPersonID": "blueclaw"
		}
	}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "requesterPersonID is invalid") {
		t.Fatalf("expected invalid requesterPersonID, got %v", errorValue)
	}
}

func TestDecodeToolInvokeRequestRejectsMalformedRequesterPersonID(t *testing.T) {
	_, errorValue := decodeToolInvokeRequest("message_send", strings.NewReader(`{
		"context": {
			"requesterPersonID": "../person-1"
		}
	}`))
	if errorValue == nil || !strings.Contains(errorValue.Error(), "requesterPersonID is invalid") {
		t.Fatalf("expected invalid requesterPersonID, got %v", errorValue)
	}
}

func TestDecodeToolInvokeRequestAcceptsPlausibleRequesterPersonID(t *testing.T) {
	request, errorValue := decodeToolInvokeRequest("message_send", strings.NewReader(`{
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
	_, errorValue := decodeToolInvokeRequest("task_list", strings.NewReader(`{
		"toolName": "task_delete",
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
		{toolName: "task_list", input: `{"status":"archived"}`},
		{toolName: "task_list", input: `{"status":"completed","slug":"demo"}`},
		{toolName: "task_delete", input: `{"taskHint":"task-1","confirm":"DELETE"}`},
		{toolName: "task_delete", input: `{"taskHint":"task-1","userConfirmed":true}`},
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
	descriptor, found := capabilityToolDescriptorFor("task_delete")
	if !found {
		t.Fatal("expected task_delete descriptor")
	}
	if descriptor.CanonicalName != "task_delete" {
		t.Fatalf("expected canonical task_delete descriptor, got %+v", descriptor)
	}
	if descriptor.PolicyResource != "tool:task_delete" {
		t.Fatalf("expected descriptor policy resource, got %q", descriptor.PolicyResource)
	}
	if !descriptor.RequiresApproval {
		t.Fatal("expected task_delete descriptor to require approval")
	}
	if descriptor.RequiresUserPresence {
		t.Fatal("task_delete executes on the device and must not ask for the requester to be present")
	}
	if _, found := capabilityToolDescriptorFor("task."); found {
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

func TestValidateContractedCapabilityResponseRejectsContractViolations(t *testing.T) {
	descriptor, found := capabilityToolDescriptorFor("task_add")
	if !found {
		t.Fatal("expected task_add descriptor")
	}

	validResult := json.RawMessage(`{"taskID":"task-1"}`)
	validEffects := []capabilities.ResourceEffect{{ObjectType: "task", Effect: "created", ID: "task-1"}}
	testCases := []struct {
		name     string
		response capabilities.ToolInvokeResponse
		message  string
	}{
		{
			name: "missing identity",
			response: capabilities.ToolInvokeResponse{
				ToolName: "task_add",
				Outcome:  capabilities.ToolOutcomeSucceeded,
				Result:   validResult,
			},
			message: "provider and selectedBackend are required",
		},
		{
			name: "wrong tool name",
			response: capabilities.ToolInvokeResponse{
				Provider:        "internkim",
				SelectedBackend: "device",
				ToolName:        "task_update",
				Outcome:         capabilities.ToolOutcomeSucceeded,
				Result:          validResult,
				Effects:         validEffects,
			},
			message: "toolName does not match",
		},
		{
			name: "mismatched effects",
			response: capabilities.ToolInvokeResponse{
				Provider:        "internkim",
				SelectedBackend: "device",
				ToolName:        "task_add",
				Outcome:         capabilities.ToolOutcomeSucceeded,
				Result:          validResult,
				Effects:         []capabilities.ResourceEffect{{ObjectType: "task", Effect: "updated", ID: "task-1"}},
			},
			message: "effects do not match",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			errorValue := validateContractedCapabilityResponse(descriptor, testCase.response, "", "")
			if errorValue == nil || !strings.Contains(errorValue.Error(), testCase.message) {
				t.Fatalf("expected %q, got %v", testCase.message, errorValue)
			}
		})
	}
}

// A tool this machine answers is written and checked in the same release, so a
// contract this side can act on is held on this side. task_add is answered on
// the plane and is carried instead, which the resource-effect suite covers.
func TestValidateContractedCapabilityResponseRejectsAnAnswerThisMachineWrote(t *testing.T) {
	descriptor, found := capabilityToolDescriptorFor("message_send")
	if !found {
		t.Fatal("message_send has no descriptor")
	}
	errorValue := validateContractedCapabilityResponse(descriptor, capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        "message_send",
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Result:          json.RawMessage(`{"deliveryStatus":"sent"}`),
	}, "", "")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "result.messageIDs is required and is missing") {
		t.Fatalf("expected the contract to name the missing field, got %v", errorValue)
	}
}

func TestAutoProviderDefaultsToRemoteBeforeLocalFallback(t *testing.T) {
	service := Service{}
	providers := service.automaticLLMProviders(
		staticLLMProvider{response: LLMResponse{Provider: "litert", SelectedBackend: "cpu", Content: `{"reply":"device"}`}},
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

func TestInvokeCapabilityToolRequiresDescriptorApproval(t *testing.T) {
	service := Service{}
	toolNames := []string{
		"message_send",
		"message_delete",
		"task_delete",
		"event_delete",
		"mail_connection_start",
		"mail_message_send",
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
				ToolName: "message_send",
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
				assertCapabilityApprovalRequired(t, response, "message_send")
			}
		})
	}
}

func TestMessageSendSelfDirectMessageApprovalGate(t *testing.T) {
	requesterContext := capabilities.ToolInvokeContext{
		RequesterPersonID: "person-yesi",
		ConversationID:    "conversation-1",
	}

	t.Run("directMessage resolving to the requester is pre-approved", func(t *testing.T) {
		service := platformDMResolverTestService(t, platformDMResolvedSampleResponse())
		request := capabilities.ToolInvokeRequest{
			ToolName: "message_send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"샘플"}`),
			Context:  requesterContext,
		}
		descriptor := descriptorForCapabilityToolTest(t, request.ToolName)
		response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptor)
		if isDenied {
			t.Fatalf("expected self direct message to be pre-approved, got %+v", response)
		}
	})

	t.Run("directMessage resolving to a different person still requires approval", func(t *testing.T) {
		service := platformDMResolverTestService(t, platformDMResolvedSampleResponse())
		request := capabilities.ToolInvokeRequest{
			ToolName: "message_send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"샘플"}`),
			Context:  capabilities.ToolInvokeContext{RequesterPersonID: "person-someone-else", ConversationID: "conversation-1"},
		}
		descriptor := descriptorForCapabilityToolTest(t, request.ToolName)
		response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptor)
		if !isDenied {
			t.Fatal("expected direct message to a different person to require approval")
		}
		assertCapabilityApprovalRequired(t, response, "message_send")
	})

	t.Run("directMessage broadcast with personHints still requires approval", func(t *testing.T) {
		service := platformDMResolverTestService(t, platformDMResolvedSampleResponse())
		request := capabilities.ToolInvokeRequest{
			ToolName: "message_send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"샘플","personHints":["샘플"]}`),
			Context:  requesterContext,
		}
		descriptor := descriptorForCapabilityToolTest(t, request.ToolName)
		response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptor)
		if !isDenied {
			t.Fatal("expected multi-recipient directMessage to require approval")
		}
		assertCapabilityApprovalRequired(t, response, "message_send")
	})

	t.Run("recipient resolution failure still requires approval", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {}))
		server.Close()
		service := Service{Configuration: Configuration{BlueclawBaseURL: server.URL}}
		request := capabilities.ToolInvokeRequest{
			ToolName: "message_send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"샘플"}`),
			Context:  requesterContext,
		}
		descriptor := descriptorForCapabilityToolTest(t, request.ToolName)
		response, isDenied := service.capabilityToolApprovalDeniedResponse(context.Background(), request, descriptor)
		if !isDenied {
			t.Fatal("expected resolution failure to require approval")
		}
		assertCapabilityApprovalRequired(t, response, "message_send")
	})

	t.Run("scheduled run never qualifies for the self direct-message pre-approval", func(t *testing.T) {
		service := platformDMResolverTestService(t, platformDMResolvedSampleResponse())
		request := capabilities.ToolInvokeRequest{
			ToolName: "message_send",
			Input:    json.RawMessage(`{"targetType":"directMessage","personHint":"샘플"}`),
			Context:  capabilities.ToolInvokeContext{RequesterPersonID: "person-yesi", ConversationID: "conversation-1", IsScheduledRun: true},
		}
		if service.isPreApprovedSelfDirectMessageSend(context.Background(), request) {
			t.Fatal("expected scheduled runs to never qualify for the self direct-message pre-approval")
		}
	})
}

func TestMessageSendPreApprovalExcludesScheduledRuns(t *testing.T) {
	request := capabilities.ToolInvokeRequest{
		ToolName: "message_send",
		Input:    json.RawMessage(`{"targetType":"currentThread"}`),
		Context:  capabilities.ToolInvokeContext{ConversationID: "conversation-1", IsScheduledRun: true},
	}
	if isPreApprovedCurrentConversationMessageSend(request) {
		t.Fatal("expected scheduled runs to never qualify for the current-conversation pre-approval")
	}
}

func TestUnconfiguredToolFailsWithoutLeakingASecret(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "sk-must-not-leak")
	service := Service{Configuration: DefaultConfiguration()}
	_, errorValue := service.invokeCapabilityTool(context.Background(), "user_confirm", strings.NewReader(`{"requiresUserPresence":true,"input":{"message":"continue?"}}`))
	if errorValue == nil {
		t.Fatal("expected an unconfigured tool to fail")
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
