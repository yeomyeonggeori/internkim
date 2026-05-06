package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/internal/llmbackend"
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

func (service Service) capabilityRegistry(ctx context.Context) (capabilities.RegistryResponse, error) {
	response := capabilities.RegistryResponse{
		LocalOnly:             service.Configuration.LocalOnly,
		RoutingCandidates:     capabilities.RoutingCandidates(),
		DeviceCapabilities:    capabilities.DeviceDescriptors(),
		CompanionStatus:       "not_configured",
		CompanionCapabilities: []capabilities.Descriptor{},
	}
	if strings.TrimSpace(service.Configuration.CompanionBaseURL) == "" {
		return response, nil
	}

	provider := service.companionProvider()
	companionCapabilities, errorValue := provider.capabilities(ctx)
	if errorValue != nil {
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

	router := CapabilityRouter{
		CompanionAvailable:     strings.TrimSpace(service.Configuration.CompanionBaseURL) != "",
		PreferCompanionBrowser: service.Configuration.PreferCompanionBrowser,
		Descriptors:            capabilities.CompanionToolDescriptors(),
	}
	if router.ShouldRouteToCompanion(request) {
		response, errorValue := service.companionProvider().InvokeTool(ctx, request)
		if errorValue == nil {
			return response, nil
		}
		if isCompanionRequiredBrowserRequest(request) {
			return companionRequiredBrowserResponse(request.ToolName, companionRequiredBrowserCode(request.ToolName), companionRequiredBrowserConstraint(request.ToolName)), nil
		}
		if request.RequiresUserPresence || isCompanionOnlyExecutionMode(request.ExecutionMode) || !isDeviceBrowserTool(request.ToolName) {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
	}
	if isCompanionRequiredBrowserRequest(request) {
		return companionRequiredBrowserResponse(request.ToolName, companionRequiredBrowserCode(request.ToolName), companionRequiredBrowserConstraint(request.ToolName)), nil
	}
	if isDeviceBrowserTool(request.ToolName) {
		return service.invokeDeviceBrowserTool(ctx, request)
	}
	if request.ToolName == "flow.task.add" {
		return service.invokeFlowTaskAdd(ctx, request)
	}
	if isGoogleWorkspaceTool(request.ToolName) {
		return service.invokeGoogleWorkspaceTool(ctx, request)
	}
	return capabilities.ToolInvokeResponse{}, errors.New("capability tool is not configured: " + request.ToolName)
}

func decodeToolInvokeRequest(toolName string, reader io.Reader) (capabilities.ToolInvokeRequest, error) {
	request := capabilities.ToolInvokeRequest{ToolName: toolName}
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
	if strings.TrimSpace(request.ToolName) == "" {
		request.ToolName = toolName
	}
	return request, nil
}

func (router CapabilityRouter) ShouldRouteToCompanion(request capabilities.ToolInvokeRequest) bool {
	if !router.CompanionAvailable {
		return false
	}
	executionMode := strings.ToLower(strings.TrimSpace(request.ExecutionMode))
	if executionMode == capabilities.ExecutionModeCompanion {
		return true
	}
	if request.RequiresUserPresence {
		return true
	}
	if router.PreferCompanionBrowser && isDeviceBrowserTool(request.ToolName) {
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

func isDeviceBrowserTool(toolName string) bool {
	return strings.HasPrefix(strings.TrimSpace(toolName), "browser.")
}

func isCompanionRequiredBrowserTool(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "browser.screenshot", "browser.handoff":
		return true
	default:
		return false
	}
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

func companionRequiredBrowserCode(toolName string) string {
	switch strings.TrimSpace(toolName) {
	case "browser.screenshot":
		return "companion_required_for_screenshot"
	case "browser.handoff":
		return "companion_required_for_handoff"
	default:
		return "companion_required_for_browser"
	}
}

func companionRequiredBrowserConstraint(toolName string) string {
	switch strings.TrimSpace(toolName) {
	case "browser.screenshot":
		return "Do not claim the screenshot was captured. Ask the user to run /connect before retrying."
	case "browser.handoff":
		return "Do not claim the browser opened. Ask the user to run /connect before retrying."
	default:
		return "Do not claim the browser task succeeded. Ask the user to run /connect before retrying."
	}
}

func companionRequiredBrowserUserReason(toolName string) string {
	switch strings.TrimSpace(toolName) {
	case "browser.screenshot":
		return "Companion is not connected, so the screenshot was not captured. Ask the user to run /connect before retrying."
	case "browser.handoff", "browser.open":
		return "Companion is not connected, so the browser was not opened. Ask the user to run /connect before retrying."
	default:
		return "Companion is not connected, so the browser task did not run. Ask the user to run /connect before retrying."
	}
}

func companionRequiredBrowserResponse(toolName string, code string, suggestedConstraint string) capabilities.ToolInvokeResponse {
	userReason := companionRequiredBrowserUserReason(toolName)
	result, _ := json.Marshal(capabilities.DenialResult{
		Status:              "denied",
		Code:                code,
		ToolName:            toolName,
		UserReason:          userReason,
		SuggestedConstraint: suggestedConstraint,
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "device",
		SelectedBackend: capabilities.LLMBackendDevice,
		ToolName:        toolName,
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

func (provider companionProvider) CompleteStructured(ctx context.Context, request StructuredLLMRequest) (LLMResponse, error) {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	toolResponse, errorValue := provider.InvokeTool(ctx, capabilities.ToolInvokeRequest{
		ToolName:      "llm.structured",
		Input:         document,
		Context:       toolInvokeContextFromLLMRequest(request.Context),
		ExecutionMode: capabilities.ExecutionModeCompanion,
		PrivacyClass:  "model_input",
	})
	if errorValue != nil {
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
	return response, nil
}

func (provider companionProvider) CompleteText(ctx context.Context, request TextLLMRequest) (LLMResponse, error) {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return LLMResponse{}, errorValue
	}
	toolResponse, errorValue := provider.InvokeTool(ctx, capabilities.ToolInvokeRequest{
		ToolName:      "llm.text",
		Input:         document,
		Context:       toolInvokeContextFromLLMRequest(request.Context),
		ExecutionMode: capabilities.ExecutionModeCompanion,
		PrivacyClass:  "model_input",
	})
	if errorValue != nil {
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
	return response, nil
}

func (provider companionProvider) CreateEmbedding(ctx context.Context, request EmbeddingRequest) (EmbeddingResponse, error) {
	document, errorValue := json.Marshal(request)
	if errorValue != nil {
		return EmbeddingResponse{}, errorValue
	}
	toolResponse, errorValue := provider.InvokeTool(ctx, capabilities.ToolInvokeRequest{
		ToolName:      "embedding.create",
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
	if response.Provider == "" {
		response.Provider = "companion"
	}
	if response.SelectedBackend == "" {
		response.SelectedBackend = capabilities.LLMBackendCompanionLocal
	}
	if response.ToolName == "" {
		response.ToolName = request.ToolName
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

func (provider companionProvider) capabilities(ctx context.Context) ([]capabilities.Descriptor, error) {
	if provider.BaseURL == "" {
		return nil, errors.New("companion base url is not configured")
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, provider.BaseURL+"/capabilities", nil)
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
