package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
	capabilityschema "gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
)

type CapabilityRouter struct {
	CompanionAvailable     bool
	PreferCompanionBrowser bool
	Descriptors            []capabilities.Descriptor
}

type companionProvider struct {
	BaseURL    string
	HTTPClient *http.Client
}

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
	{ToolName: "message_context", Handler: Service.invokePlatformMessageTool},
	{ToolName: "message_search", Handler: Service.invokePlatformMessageTool},
	{ToolName: "message_send", Handler: Service.invokePlatformMessageTool},
	{ToolName: "message_update", Handler: Service.invokePlatformMessageTool},
	{ToolName: "message_delete", Handler: Service.invokePlatformMessageTool},
	{ToolName: "mail_message_list", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_search", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_read", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_send", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_move", Handler: Service.invokeMailTool},
	{ToolName: "mail_message_mark", Handler: Service.invokeMailTool},
	{ToolName: "mail_connection_status", Handler: Service.invokeMailTool},
	{ToolName: "mail_connection_start", Handler: Service.invokeMailTool},
	{ToolName: "site_serve", Handler: Service.invokeSiteAppTool},
	{ToolName: "site_list", Handler: Service.invokeSiteAppTool},
	{ToolName: "site_unserve", Handler: Service.invokeSiteAppTool},
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

// The browser namespace is served by the device browser runtime. Which subsystem
// owns a tool is identity the descriptor states, not something to read off a name.
var companionToolNamespaces = buildCompanionToolNamespaces()

func buildCompanionToolNamespaces() map[string]string {
	namespaceByToolName := map[string]string{}
	for _, descriptor := range capabilities.CompanionToolDescriptors() {
		namespaceByToolName[descriptor.Name] = descriptor.Namespace
	}
	return namespaceByToolName
}

func isDeviceBrowserTool(toolName string) bool {
	return companionToolNamespaces[strings.TrimSpace(toolName)] == browserToolNamespace
}

const browserToolNamespace = "browser"

func (service Service) capabilityRegistry(ctx context.Context) (capabilities.RegistryResponse, error) {
	response := capabilities.RegistryResponse{
		ProtocolIdentity:      capabilityprotocol.GeneratedProtocolIdentity(),
		LocalOnly:             service.Configuration.LocalOnly,
		RoutingCandidates:     capabilities.RoutingCandidates(),
		DeviceCapabilities:    capabilities.DefaultToolDescriptors(),
		CompanionStatus:       "not_configured",
		CompanionCapabilities: []capabilities.Descriptor{},
	}
	if strings.TrimSpace(service.Configuration.CompanionBaseURL) == "" {
		return response, nil
	}

	provider := service.companionProvider()
	companionCapabilities, errorValue := provider.capabilities(ctx)
	if errorValue != nil || len(companionCapabilities) == 0 {
		response.CompanionStatus = "unavailable"
		return response, nil
	}
	response.CompanionStatus = "available"
	response.CompanionCapabilities = companionCapabilities
	return response, nil
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

	router := CapabilityRouter{
		CompanionAvailable:     strings.TrimSpace(service.Configuration.CompanionBaseURL) != "",
		PreferCompanionBrowser: service.Configuration.PreferCompanionBrowser,
		Descriptors:            capabilities.CompanionToolDescriptors(),
	}
	if router.ShouldRouteToCompanion(request) {
		response, errorValue := service.companionProvider().InvokeTool(ctx, request)
		if errorValue == nil {
			if shouldFallbackToDeviceBrowser(request, response) {
				deviceResponse, errorValue := service.invokeDeviceBrowserTool(ctx, request)
				if errorValue != nil {
					return capabilities.ToolInvokeResponse{}, errorValue
				}
				if errorValue := validateContractedCapabilityResponse(descriptor, deviceResponse, "", ""); errorValue != nil {
					return capabilities.ToolInvokeResponse{}, errorValue
				}
				return deviceResponse, nil
			}
			if errorValue := validateContractedCapabilityResponse(descriptor, response, "companion", capabilities.LLMBackendCompanionLocal); errorValue != nil {
				return capabilities.ToolInvokeResponse{}, errorValue
			}
			return response, nil
		}
		if isCompanionRequiredBrowserRequest(request) {
			return capabilityUnavailableResponse(request.ToolName, companionRequiredBrowserErrorCode(errorValue)), nil
		}
		if request.RequiresUserPresence || isCompanionOnlyExecutionMode(request.ExecutionMode) || !isDeviceBrowserTool(request.ToolName) {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
	}
	if isCompanionRequiredBrowserRequest(request) {
		return capabilityUnavailableResponse(request.ToolName, capabilities.CapabilityNotConnected), nil
	}
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
	if requesterApprovedThisCall(request.Context) || request.Context.IsScheduledRun {
		return capabilities.ToolInvokeResponse{}, false
	}
	if isPreApprovedCurrentConversationMessageSend(request) {
		return capabilities.ToolInvokeResponse{}, false
	}
	if service.isPreApprovedSelfDirectMessageSend(ctx, request) {
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

// A reply into the same thread or channel the user is already talking in
// carries no more authority than the message that prompted it, so it does
// not need a separate approval step. Scheduled/proactive runs are excluded
// because there is no live user turn granting that authority in the moment.
func isPreApprovedCurrentConversationMessageSend(request capabilities.ToolInvokeRequest) bool {
	if request.ToolName != "message_send" {
		return false
	}
	if request.Context.IsScheduledRun {
		return false
	}
	if strings.TrimSpace(request.Context.ConversationID) == "" {
		return false
	}
	deliveryTargetType := decodeMessageSendDeliveryTarget(request.Input).Type
	return deliveryTargetType == "currentThread" || deliveryTargetType == "currentChannel"
}

// A direct message the bot sends back to the same person who asked for it
// carries no more authority than a reply in their current thread/channel:
// no third party is involved, only the delivery channel differs. Broadcasts
// with multiple recipient hints are excluded because that is a different
// trust shape.
func (service Service) isPreApprovedSelfDirectMessageSend(ctx context.Context, request capabilities.ToolInvokeRequest) bool {
	if request.ToolName != "message_send" {
		return false
	}
	if request.Context.IsScheduledRun {
		return false
	}
	deliveryTarget := decodeMessageSendDeliveryTarget(request.Input)
	if deliveryTarget.Type != "directMessage" {
		return false
	}
	personHint := strings.TrimSpace(deliveryTarget.PersonHint)
	if personHint == "" || len(deliveryTarget.PersonHints) > 0 {
		return false
	}
	recipient, _, hasFailure := service.resolvePlatformDMRecipient(ctx, personHint, request.Context.ResponseLanguage)
	if hasFailure {
		return false
	}
	return isPlatformDMSelfRecipient(request.Context, recipient)
}

type messageSendDeliveryTarget struct {
	Type        string   `json:"type"`
	PersonHint  string   `json:"personHint"`
	PersonHints []string `json:"personHints"`
}

func decodeMessageSendDeliveryTarget(input json.RawMessage) messageSendDeliveryTarget {
	var decodedInput struct {
		TargetType  string   `json:"targetType"`
		PersonHint  string   `json:"personHint"`
		PersonHints []string `json:"personHints"`
	}
	if errorValue := json.Unmarshal(input, &decodedInput); errorValue != nil {
		return messageSendDeliveryTarget{}
	}
	return messageSendDeliveryTarget{
		Type:        decodedInput.TargetType,
		PersonHint:  decodedInput.PersonHint,
		PersonHints: decodedInput.PersonHints,
	}
}

func companionRequiredBrowserErrorCode(errorValue error) string {
	if errorValue == nil {
		return capabilities.CapabilityNotConnected
	}
	errorMessage := strings.ToLower(errorValue.Error())
	if strings.Contains(errorMessage, "expired") || strings.Contains(errorMessage, "timed out") || strings.Contains(errorMessage, "context deadline exceeded") {
		return capabilities.CapabilityNotReady
	}
	return capabilities.CapabilityNotConnected
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
		if toolContext.IsScheduledRun || toolContext.IsApprovalContinuation {
			return capabilities.ToolInvokeContext{}, errors.New("requesterPersonID is required for scheduled runs and approval continuations")
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

func (router CapabilityRouter) ShouldRouteToCompanion(request capabilities.ToolInvokeRequest) bool {
	if !router.CompanionAvailable {
		return false
	}
	if isDeviceBrowserTool(request.ToolName) {
		return false
	}
	executionMode := strings.ToLower(strings.TrimSpace(request.ExecutionMode))
	if executionMode == capabilities.ExecutionModeCompanion {
		return true
	}
	if request.RequiresUserPresence {
		return true
	}
	return router.isCompanionCapability(request.ToolName)
}

func (router CapabilityRouter) isCompanionCapability(toolName string) bool {
	trimmedToolName := strings.TrimSpace(toolName)
	for _, descriptor := range router.Descriptors {
		if descriptor.Name == trimmedToolName {
			return true
		}
	}
	return false
}

func isCompanionOnlyExecutionMode(executionMode string) bool {
	normalizedExecutionMode := strings.ToLower(strings.TrimSpace(executionMode))
	return normalizedExecutionMode == capabilities.ExecutionModeCompanion
}

var companionBrowserToolNames = buildCompanionBrowserToolNames()

func buildCompanionBrowserToolNames() map[string]bool {
	requiredByToolName := map[string]bool{}
	for _, descriptor := range capabilities.CompanionToolDescriptors() {
		requiredByToolName[descriptor.Name] = descriptor.RequiresCompanionBrowser
	}
	return requiredByToolName
}

func isCompanionRequiredBrowserTool(toolName string) bool {
	return companionBrowserToolNames[strings.TrimSpace(toolName)]
}

func isCompanionRequiredBrowserRequest(request capabilities.ToolInvokeRequest) bool {
	if isCompanionRequiredBrowserTool(request.ToolName) {
		return true
	}
	if !isDeviceBrowserTool(request.ToolName) {
		return false
	}
	return request.RequiresUserPresence || isCompanionOnlyExecutionMode(request.ExecutionMode)
}

func shouldFallbackToDeviceBrowser(request capabilities.ToolInvokeRequest, response capabilities.ToolInvokeResponse) bool {
	if !isDeviceBrowserTool(request.ToolName) || isCompanionRequiredBrowserRequest(request) {
		return false
	}
	return isCapabilityUnavailableResponse(response)
}

func isCapabilityUnavailableResponse(response capabilities.ToolInvokeResponse) bool {
	var denial capabilities.DenialResult
	if response.Status != "denied" || json.Unmarshal(response.Result, &denial) != nil {
		return false
	}
	switch denial.Code {
	case capabilities.CapabilityNotConnected, capabilities.CapabilityNotReady, capabilities.CapabilityNotAllowed:
		return true
	default:
		return false
	}
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

func (service Service) companionProvider() companionProvider {
	return companionProvider{
		BaseURL:    strings.TrimRight(strings.TrimSpace(service.Configuration.CompanionBaseURL), "/"),
		HTTPClient: service.httpClient(),
	}
}

func (service Service) companionInferenceProvider() companionProvider {
	return companionProvider{
		BaseURL:    strings.TrimRight(strings.TrimSpace(service.Configuration.CompanionBaseURL), "/"),
		HTTPClient: service.providerHTTPClient(),
	}
}

func (provider companionProvider) CompleteStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	toolResponse, errorValue := provider.InvokeTool(ctx, capabilities.ToolInvokeRequest{
		ToolName:      "llm_structured",
		Input:         document,
		Context:       toolInvokeContextFromLLMRequest(request.Context),
		ExecutionMode: capabilities.ExecutionModeCompanion,
		PrivacyClass:  "model_input",
	})
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	if errorValue := validateCompanionToolResponse(toolResponse); errorValue != nil {
		return LLMResponse{}, errorValue
	}
	var response LLMResponse
	if errorValue := json.Unmarshal(toolResponse.Result, &response); errorValue != nil {
		return LLMResponse{}, errorValue
	}
	if response.SelectedBackend == "" {
		response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	}
	if response.Provider == "" {
		response.Provider = "companion"
	}
	if !llmbackend.ValidateStructuredJSON(response.Content) {
		return LLMResponse{}, errors.New("companion structured llm response was empty or invalid json")
	}
	return response, nil
}

func (provider companionProvider) CompleteText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	toolResponse, errorValue := provider.InvokeTool(ctx, capabilities.ToolInvokeRequest{
		ToolName:      "llm_text",
		Input:         document,
		Context:       toolInvokeContextFromLLMRequest(request.Context),
		ExecutionMode: capabilities.ExecutionModeCompanion,
		PrivacyClass:  "model_input",
	})
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	if errorValue := validateCompanionToolResponse(toolResponse); errorValue != nil {
		return LLMResponse{}, errorValue
	}
	var response LLMResponse
	if errorValue := json.Unmarshal(toolResponse.Result, &response); errorValue != nil {
		return LLMResponse{}, errorValue
	}
	if response.SelectedBackend == "" {
		response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	}
	if response.Provider == "" {
		response.Provider = "companion"
	}
	if strings.TrimSpace(response.Content) == "" {
		return LLMResponse{}, errors.New("companion text llm response was empty")
	}
	return response, nil
}

func (provider companionProvider) CompleteChat(ctx context.Context, request ChatLLMRequest) (ChatLLMResponse, error) {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return ChatLLMResponse{}, errorValue
	}
	toolResponse, errorValue := provider.InvokeTool(ctx, capabilities.ToolInvokeRequest{
		ToolName:      "llm.chat",
		Input:         document,
		Context:       toolInvokeContextFromLLMRequest(request.Context),
		ExecutionMode: capabilities.ExecutionModeCompanion,
		PrivacyClass:  "model_input",
	})
	if errorValue != nil {
		return ChatLLMResponse{}, errorValue
	}
	if errorValue := validateCompanionToolResponse(toolResponse); errorValue != nil {
		return ChatLLMResponse{}, errorValue
	}
	var response ChatLLMResponse
	if errorValue := json.Unmarshal(toolResponse.Result, &response); errorValue != nil {
		return ChatLLMResponse{}, errorValue
	}
	if response.Provider == "" {
		response.Provider = "companion"
	}
	return response, nil
}

func validateCompanionToolResponse(response capabilities.ToolInvokeResponse) error {
	if response.IsError {
		return errors.New(firstNonEmpty(response.Content, response.Status, "companion tool returned an error"))
	}
	status := strings.ToLower(strings.TrimSpace(response.Status))
	if status == "denied" || status == "failed" || status == "error" {
		return errors.New(firstNonEmpty(response.Content, response.Status, "companion tool returned an error"))
	}
	result := bytes.TrimSpace(response.Result)
	if len(result) == 0 || bytes.Equal(result, []byte("null")) {
		return errors.New("companion tool response result was empty")
	}
	return nil
}

func (provider companionProvider) CreateEmbedding(ctx context.Context, request EmbeddingRequest) (EmbeddingResponse, error) {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	toolResponse, errorValue := provider.InvokeTool(ctx, capabilities.ToolInvokeRequest{
		ToolName:      "embedding_create",
		Input:         document,
		ExecutionMode: capabilities.ExecutionModeCompanion,
		PrivacyClass:  "model_input",
	})
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	var response EmbeddingResponse
	if errorValue := json.Unmarshal(toolResponse.Result, &response); errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	if response.SelectedBackend == "" {
		response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	}
	if response.Provider == "" {
		response.Provider = "companion"
	}
	return response, nil
}

func (provider companionProvider) InvokeTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	if provider.BaseURL == "" {
		return capabilities.ToolInvokeResponse{}, errors.New("companion capability is unavailable: " + request.ToolName)
	}
	if len(bytes.TrimSpace(request.Input)) == 0 {
		request.Input = json.RawMessage(`{}`)
	}
	var response capabilities.ToolInvokeResponse
	if errorValue := provider.postJSON(ctx, "/jobs", request, &response); errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return response, nil
}

func toolInvokeContextFromLLMRequest(requestContext llmbackend.RequestContext) capabilities.ToolInvokeContext {
	return capabilities.ToolInvokeContext{
		RequesterPersonID:       strings.TrimSpace(requestContext.RequesterPersonID),
		RequesterEmail:          strings.ToLower(strings.TrimSpace(requestContext.RequesterEmail)),
		RequesterName:           strings.TrimSpace(requestContext.RequesterName),
		RequesterPlatformUserID: strings.TrimSpace(requestContext.RequesterPlatformUserID),
		ConversationID:          strings.TrimSpace(requestContext.ConversationID),
		Platform:                strings.TrimSpace(requestContext.Platform),
	}
}

const companionCapabilitiesProbeTimeout = 3 * time.Second

func (provider companionProvider) capabilities(ctx context.Context) ([]capabilities.Descriptor, error) {
	if provider.BaseURL == "" {
		return nil, errors.New("companion base url is not configured")
	}
	probeContext, cancelProbe := context.WithTimeout(ctx, companionCapabilitiesProbeTimeout)
	defer cancelProbe()
	httpRequest, errorValue := http.NewRequestWithContext(probeContext, http.MethodGet, provider.BaseURL+"/capabilities", nil)
	if errorValue != nil {
		return nil, errorValue
	}
	httpResponse, errorValue := provider.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(httpResponse.Body)
		return nil, errors.New(string(body))
	}
	var response capabilities.RegistryResponse
	if errorValue := json.NewDecoder(httpResponse.Body).Decode(&response); errorValue != nil {
		return nil, errorValue
	}
	return response.Capabilities, nil
}

func (provider companionProvider) postJSON(ctx context.Context, path string, request any, response any) error {
	if provider.BaseURL == "" {
		return errors.New("companion base url is not configured")
	}
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, provider.BaseURL+path, bytes.NewReader(document))
	if errorValue != nil {
		return errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := provider.httpClient().Do(httpRequest)
	if errorValue != nil {
		return errorValue
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(httpResponse.Body)
		return errors.New(string(body))
	}
	return json.NewDecoder(httpResponse.Body).Decode(response)
}

func (provider companionProvider) httpClient() *http.Client {
	if provider.HTTPClient != nil {
		return provider.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// The requester approved this call. A turn that carries out a call held on an
// earlier turn says so as a property of the turn; a call approved inside the
// turn it was made in names the held call it spends.
func requesterApprovedThisCall(toolContext capabilities.ToolInvokeContext) bool {
	return toolContext.IsApprovalContinuation || strings.TrimSpace(toolContext.ApprovedCallID) != ""
}
