package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol"
	capabilityschema "github.com/yeomyeonggeori/internkim/pkg/capabilityprotocol/jsonschema"
)

type capabilityToolHandler func(Service, context.Context, capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error)

// A route says which handler runs a tool. Everything else a caller wants to know
// about that tool - who it reaches, whether it needs the person's own browser -
// is written on the descriptor, where every other reader can see it too.
type capabilityToolRoute struct {
	ToolName string
	Handler  capabilityToolHandler
}

var capabilityToolRoutes = []capabilityToolRoute{
	{ToolName: "browser_screenshot", Handler: Service.invokeDeviceBrowserTool},
	{ToolName: "browser_open", Handler: Service.invokeDeviceBrowserTool},
	{ToolName: "browser_snapshot", Handler: Service.invokeDeviceBrowserTool},
	{ToolName: "browser_click", Handler: Service.invokeDeviceBrowserTool},
	{ToolName: "browser_fill", Handler: Service.invokeDeviceBrowserTool},
	{ToolName: "browser_select", Handler: Service.invokeDeviceBrowserTool},
	{ToolName: "browser_press", Handler: Service.invokeDeviceBrowserTool},
	{ToolName: "browser_wait", Handler: Service.invokeDeviceBrowserTool},
	{ToolName: "web_search", Handler: Service.invokeWebTool},
	{ToolName: "web_fetch", Handler: Service.invokeWebTool},
	{ToolName: "document_read", Handler: Service.invokeDocumentReadTool},
	{ToolName: "image_read", Handler: Service.invokeImageReadTool},
	{ToolName: "image_generate", Handler: Service.invokeImageGenerateTool},
	{ToolName: "artifact_review", Handler: Service.invokeArtifactReviewTool},
	{ToolName: "schedule_list", Handler: Service.invokeScheduleTool},
	{ToolName: "schedule_create", Handler: Service.invokeScheduleTool},
	{ToolName: "schedule_update", Handler: Service.invokeScheduleTool},
	{ToolName: "schedule_cancel", Handler: Service.invokeScheduleTool},
	{ToolName: "task_label_get", Handler: Service.invokeTaskLabelTool},
	{ToolName: "company_document_classify", Handler: Service.invokeDataRoomClassification},
	{ToolName: "host_version_get", Handler: Service.invokeHostVersionTool},
	{ToolName: "host_diagnostics_get", Handler: Service.invokeHostDiagnosticsTool},
	{ToolName: hostUpdateToolName, Handler: Service.invokeHostUpdateTool},
	{ToolName: "message_context", Handler: Service.invokePlatformMessageTool},
	{ToolName: "message_search", Handler: Service.invokePlatformMessageTool},
	{ToolName: "message_send", Handler: Service.invokePlatformMessageTool},
	{ToolName: "message_update", Handler: Service.invokePlatformMessageTool},
	{ToolName: "message_delete", Handler: Service.invokePlatformMessageTool},
	{ToolName: "mail_mailbox_list", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_list", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_search", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_read", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_send", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_move", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_mark", Handler: Service.invokeMailTool},
	{ToolName: "mail_connection_status", Handler: Service.invokeMailTool},
	{ToolName: "mail_connection_start", Handler: Service.invokeMailTool},
}

var capabilityToolDescriptorsByCanonicalName = buildCapabilityToolDescriptorsByCanonicalName()

type capabilityApprovalFailure struct {
	ErrorCode    string `json:"errorCode"`
	FailureStage string `json:"failureStage"`
	Message      string `json:"message"`
}

func buildCapabilityToolDescriptorsByCanonicalName() map[string]capabilities.Descriptor {
	descriptors := capabilities.DefaultToolDescriptors()
	byCanonicalName := make(map[string]capabilities.Descriptor, len(descriptors))
	for _, descriptor := range descriptors {
		byCanonicalName[descriptor.CanonicalName] = descriptor
	}
	return byCanonicalName
}

func capabilityToolDescriptorFor(toolName string) (capabilities.Descriptor, bool) {
	descriptor, found := capabilityToolDescriptorsByCanonicalName[strings.TrimSpace(toolName)]
	return descriptor, found
}

func capabilityToolRouteFor(toolName string) (capabilityToolRoute, bool) {
	if theRecordAnswers(toolName) {
		return capabilityToolRoute{ToolName: strings.TrimSpace(toolName), Handler: Service.invokeRecordTool}, true
	}
	for _, route := range capabilityToolRoutes {
		if route.matches(toolName) {
			return route, true
		}
	}
	return capabilityToolRoute{}, false
}

func (route capabilityToolRoute) matches(toolName string) bool {
	matchedToolName := route.matchableToolName(toolName)
	if route.ToolName != "" {
		return route.ToolName == matchedToolName
	}
	return false
}

func (route capabilityToolRoute) matchableToolName(toolName string) string {
	return strings.TrimSpace(toolName)
}

func (service Service) capabilityRegistry(_ context.Context) (capabilities.RegistryResponse, error) {
	return capabilities.RegistryResponse{
		ProtocolIdentity:   capabilityprotocol.GeneratedProtocolIdentity(),
		LocalOnly:          service.Configuration.LocalOnly,
		RoutingCandidates:  capabilities.RoutingCandidates(),
		DeviceCapabilities: capabilities.DefaultToolDescriptors(),
	}, nil
}

func (service Service) invokeCapabilityTool(ctx context.Context, toolName string, reader io.Reader) (capabilities.ToolInvokeResponse, error) {
	request, errorValue := decodeToolInvokeRequest(toolName, reader)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	descriptor, hasDescriptor := capabilityToolDescriptorFor(request.ToolName)
	if !hasDescriptor {
		return capabilities.ToolInvokeResponse{}, errors.New("capability tool is not configured: " + request.ToolName)
	}
	if response, isDenied := service.capabilityToolApprovalDeniedResponse(ctx, request, descriptor); isDenied {
		return response, nil
	}
	if errorValue := capabilityschema.ValidateInput(descriptor.InputSchema, request.Input); errorValue != nil {
		return capabilityInvalidInputResponse(request.ToolName, errorValue), nil
	}
	toolRoute, hasToolRoute := capabilityToolRouteFor(descriptor.CanonicalName)

	if hasToolRoute {
		response, errorValue := toolRoute.Handler(service, ctx, request)
		if errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
		if errorValue := validateContractedCapabilityResponse(descriptor, response, "", ""); errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
		return response, nil
	}
	return capabilities.ToolInvokeResponse{}, errors.New("capability tool is not configured: " + request.ToolName)
}

func validateContractedCapabilityResponse(descriptor capabilities.Descriptor, response capabilities.ToolInvokeResponse, expectedProvider string, expectedBackend string) error {
	if descriptor.ResultContract == nil || capabilityResponseIsFailure(response) {
		return nil
	}
	if errorValue := validateCapabilityResponseIdentity(descriptor, response, expectedProvider, expectedBackend); errorValue != nil {
		return errorValue
	}
	if errorValue := holdResultToContract(descriptor, response.Result); errorValue != nil {
		return errorValue
	}
	expectedEffects, errorValue := capabilities.ProjectResourceEffects(descriptor.ResultContract, response.Result)
	if errorValue != nil {
		return errorValue
	}
	if !resourceEffectsMatch(expectedEffects, response.Effects) {
		return errors.New("capability result effects do not match the result contract")
	}
	return nil
}

func validateCapabilityResponseIdentity(descriptor capabilities.Descriptor, response capabilities.ToolInvokeResponse, expectedProvider string, expectedBackend string) error {
	if strings.TrimSpace(response.Provider) == "" || strings.TrimSpace(response.SelectedBackend) == "" {
		return errors.New("capability result provider and selectedBackend are required")
	}
	if expectedProvider != "" && strings.TrimSpace(response.Provider) != expectedProvider {
		return fmt.Errorf("capability result provider does not match %s", expectedProvider)
	}
	if expectedBackend != "" && strings.TrimSpace(response.SelectedBackend) != expectedBackend {
		return fmt.Errorf("capability result selectedBackend does not match %s", expectedBackend)
	}
	if strings.TrimSpace(response.ToolName) != descriptor.CanonicalName {
		return errors.New("capability result toolName does not match the invoked operation")
	}
	return nil
}

func capabilityResponseIsFailure(response capabilities.ToolInvokeResponse) bool {
	if response.IsError {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(response.Status)) {
	case "denied", "failed", "error":
		return true
	default:
		return response.Outcome == capabilities.ToolOutcomeFailed || response.Outcome == capabilities.ToolOutcomeDenied
	}
}

func resourceEffectsMatch(expectedEffects []capabilities.ResourceEffect, actualEffects []capabilities.ResourceEffect) bool {
	if len(expectedEffects) != len(actualEffects) {
		return false
	}
	remainingEffects := make(map[capabilities.ResourceEffect]int, len(actualEffects))
	for _, actualEffect := range actualEffects {
		remainingEffects[actualEffect]++
	}
	for _, expectedEffect := range expectedEffects {
		if remainingEffects[expectedEffect] == 0 {
			return false
		}
		remainingEffects[expectedEffect]--
	}
	return true
}

func capabilityInvalidInputResponse(toolName string, errorValue error) capabilities.ToolInvokeResponse {
	message := errorValue.Error()
	result, _ := json.Marshal(map[string]string{
		"errorCode":    "invalid_input",
		"failureStage": "input_schema",
		"message":      message,
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        strings.TrimSpace(toolName),
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         message,
		IsError:         true,
		Message:         message,
		ErrorCode:       "invalid_input",
		FailureStage:    "input_schema",
		Result:          result,
	}
}

func (service Service) capabilityToolApprovalDeniedResponse(ctx context.Context, request capabilities.ToolInvokeRequest, descriptor capabilities.Descriptor) (capabilities.ToolInvokeResponse, bool) {
	if !descriptor.RequiresApproval {
		return capabilities.ToolInvokeResponse{}, false
	}
	if requesterApprovedThisCall(request) {
		return capabilities.ToolInvokeResponse{}, false
	}
	if sendsIntoTheAnsweredConversation(request) {
		return capabilities.ToolInvokeResponse{}, false
	}
	toolName := strings.TrimSpace(request.ToolName)
	message := toolName + " requires approval before execution"
	result, _ := json.Marshal(capabilityApprovalFailure{
		ErrorCode:    "approval_required",
		FailureStage: "authorization",
		Message:      message,
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeDenied,
		Status:          "denied",
		Content:         message,
		IsError:         true,
		Message:         message,
		ErrorCode:       "approval_required",
		FailureStage:    "authorization",
		Result:          result,
	}, true
}

func sendsIntoTheAnsweredConversation(request capabilities.ToolInvokeRequest) bool {
	if request.ToolName != "message_send" || request.Context.IsScheduledRun {
		return false
	}
	if strings.TrimSpace(request.Context.ConversationID) == "" {
		return false
	}
	input, errorValue := decodePlatformMessageSendInput(request.Input)
	if errorValue != nil {
		return false
	}
	return landsInTheAnsweredConversation(request.Context, input.DeliveryTarget)
}

func decodeToolInvokeRequest(toolName string, reader io.Reader) (capabilities.ToolInvokeRequest, error) {
	canonicalToolName := strings.TrimSpace(toolName)
	request := capabilities.ToolInvokeRequest{ToolName: canonicalToolName}
	if reader == nil {
		return request, nil
	}
	document, errorValue := io.ReadAll(reader)
	if errorValue != nil {
		return capabilities.ToolInvokeRequest{}, errorValue
	}
	if len(bytes.TrimSpace(document)) == 0 {
		return request, nil
	}
	if errorValue := json.Unmarshal(document, &request); errorValue != nil {
		return capabilities.ToolInvokeRequest{}, errorValue
	}
	bodyToolName := strings.TrimSpace(request.ToolName)
	if bodyToolName != "" && bodyToolName != canonicalToolName {
		return capabilities.ToolInvokeRequest{}, fmt.Errorf("tool name mismatch: URL operation %q does not match body operation %q", canonicalToolName, bodyToolName)
	}
	request.ToolName = canonicalToolName
	toolContext, errorValue := validateToolInvokeContext(request.Context)
	if errorValue != nil {
		return capabilities.ToolInvokeRequest{}, errorValue
	}
	request.Context = toolContext
	return request, nil
}

func validateToolInvokeContext(toolContext capabilities.ToolInvokeContext) (capabilities.ToolInvokeContext, error) {
	requesterPersonID := strings.TrimSpace(toolContext.RequesterPersonID)
	if requesterPersonID == "" {
		if toolContext.IsScheduledRun {
			return capabilities.ToolInvokeContext{}, errors.New("requesterPersonID is required for scheduled runs")
		}
		return toolContext, nil
	}
	if !isPlausibleRequesterPersonID(requesterPersonID) {
		return capabilities.ToolInvokeContext{}, errors.New("requesterPersonID is invalid")
	}
	toolContext.RequesterPersonID = requesterPersonID
	return toolContext, nil
}

func isPlausibleRequesterPersonID(requesterPersonID string) bool {
	if len(requesterPersonID) > 128 {
		return false
	}
	if !isRequesterPersonIDAlphanumeric(rune(requesterPersonID[0])) {
		return false
	}
	normalizedRequesterPersonID := strings.ToLower(requesterPersonID)
	switch normalizedRequesterPersonID {
	case "root", "blueclaw", "system", "admin", "unknown":
		return false
	}
	for _, character := range requesterPersonID {
		if isRequesterPersonIDCharacter(character) {
			continue
		}
		return false
	}
	return true
}

func isRequesterPersonIDAlphanumeric(character rune) bool {
	if character >= 'a' && character <= 'z' {
		return true
	}
	if character >= 'A' && character <= 'Z' {
		return true
	}
	return character >= '0' && character <= '9'
}

func isRequesterPersonIDCharacter(character rune) bool {
	if isRequesterPersonIDAlphanumeric(character) {
		return true
	}
	return character == '-' || character == '_' || character == '.'
}

func capabilityUnavailableResponse(toolName string, code string) capabilities.ToolInvokeResponse {
	userReason := capabilities.CapabilityUnavailableUserReason(toolName, code)
	var recovery *capabilities.RecoveryAction
	result, _ := json.Marshal(capabilities.DenialResult{
		Status:              "denied",
		Code:                code,
		ToolName:            toolName,
		UserReason:          userReason,
		SuggestedConstraint: userReason,
		Recovery:            recovery,
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeDenied,
		Status:          "denied",
		Content:         userReason,
		IsError:         true,
		Result:          result,
	}
}

func requesterApprovedThisCall(request capabilities.ToolInvokeRequest) bool {
	toolContext := request.Context
	if strings.TrimSpace(toolContext.HoldID) != "" {
		return true
	}
	if toolContext.TaskSource == capabilities.TaskSourcePublicAPI || toolContext.TaskSource == capabilities.TaskSourcePlaneTelling {
		return true
	}
	return isScheduledRunOfExactlyThisCall(request)
}

func isScheduledRunOfExactlyThisCall(request capabilities.ToolInvokeRequest) bool {
	approvedCall := request.Context.ScheduledApprovedCall
	if !request.Context.IsScheduledRun || approvedCall == nil {
		return false
	}
	if strings.TrimSpace(approvedCall.ToolName) != strings.TrimSpace(request.ToolName) {
		return false
	}
	return isSameJSON(approvedCall.ToolInput, request.Input)
}

func isSameJSON(first json.RawMessage, second json.RawMessage) bool {
	var firstValue, secondValue any
	if json.Unmarshal(emptyObjectIfBlank(first), &firstValue) != nil || json.Unmarshal(emptyObjectIfBlank(second), &secondValue) != nil {
		return false
	}
	return reflect.DeepEqual(firstValue, secondValue)
}

func emptyObjectIfBlank(document json.RawMessage) []byte {
	if len(bytes.TrimSpace(document)) == 0 {
		return []byte("{}")
	}
	return document
}
