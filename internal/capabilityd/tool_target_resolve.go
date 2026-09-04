package capabilityd

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	capabilityschema "gitlab.com/eastriver/internkim/pkg/capabilityprotocol/jsonschema"
)

type capabilityToolTargetRoute struct {
	ToolName string
	Resolver capabilityToolHandler
}

var capabilityToolTargetRoutes = []capabilityToolTargetRoute{
	{ToolName: "message_delete", Resolver: Service.resolveMessageDeleteTarget},
}

func capabilityToolTargetRouteFor(toolName string) (capabilityToolTargetRoute, bool) {
	trimmedToolName := strings.TrimSpace(toolName)
	if theRecordAnswers(trimmedToolName) {
		return capabilityToolTargetRoute{ToolName: trimmedToolName, Resolver: Service.previewRecordToolTarget}, true
	}
	for _, route := range capabilityToolTargetRoutes {
		if route.ToolName == trimmedToolName {
			return route, true
		}
	}
	return capabilityToolTargetRoute{}, false
}

func (service Service) resolveCapabilityToolTarget(ctx context.Context, toolName string, reader io.Reader) (capabilities.ToolInvokeResponse, error) {
	request, errorValue := decodeToolInvokeRequest(toolName, reader)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	descriptor, hasDescriptor := capabilityToolDescriptorFor(request.ToolName)
	targetRoute, hasTargetRoute := capabilityToolTargetRouteFor(request.ToolName)
	if !hasDescriptor || !hasTargetRoute {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	if errorValue := capabilityschema.ValidateInput(descriptor.InputSchema, request.Input); errorValue != nil {
		return capabilityInvalidInputResponse(request.ToolName, errorValue), nil
	}
	return targetRoute.Resolver(service, ctx, request)
}

func capabilityToolTargetResponse(toolName string, target capabilities.ApprovalTarget) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(target)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        strings.TrimSpace(toolName),
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "resolved",
		Content:         target.Title,
		Result:          result,
	}
}

func capabilityToolWithoutTargetResponse(toolName string) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        strings.TrimSpace(toolName),
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "no_target",
		Result:          json.RawMessage(`{}`),
	}
}

const messageDeletePreviewMessageLimit = 5
const messageDeletePreviewCharacterLimit = 160

// A deletion is approved against the words it removes, so the approval
// question needs the messages' own text, not their IDs. The preview never
// narrows the replayed input; a lookup that fails resolves to no target and
// the question falls back to what the input alone can say.
func (service Service) resolveMessageDeleteTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodePlatformMessageDeleteInput(request.Input)
	if errorValue != nil {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	messageTexts := service.messageTextsForApproval(ctx, request.Context, input.MessageIDs)
	if len(messageTexts) == 0 {
		return capabilityToolWithoutTargetResponse(request.ToolName), nil
	}
	return capabilityToolTargetResponse(request.ToolName, capabilities.ApprovalTarget{
		Preview: messageDeletePreview(messageTexts, len(input.MessageIDs)),
	}), nil
}

func (service Service) messageTextsForApproval(ctx context.Context, toolContext capabilities.ToolInvokeContext, messageIDs []string) []string {
	limitedMessageIDs := messageIDs
	if len(limitedMessageIDs) > messageDeletePreviewMessageLimit {
		limitedMessageIDs = limitedMessageIDs[:messageDeletePreviewMessageLimit]
	}
	return service.chatdMessageTexts(ctx, limitedMessageIDs)
}

func (service Service) chatdMessageTexts(ctx context.Context, messageIDs []string) []string {
	var response chatdMessageSearchResponse
	searchRequest := chatdMessageSearchRequest{MessageIDs: messageIDs, Queries: []string{}, Limit: len(messageIDs)}
	if service.chatdRequest(ctx, "message.search", searchRequest, &response) != nil {
		return nil
	}
	messageTexts := []string{}
	for _, candidate := range response.Candidates {
		if text := strings.TrimSpace(candidate.Text); text != "" {
			messageTexts = append(messageTexts, text)
		}
	}
	return messageTexts
}

func messageDeletePreview(messageTexts []string, totalMessageCount int) string {
	previews := make([]string, 0, len(messageTexts))
	for _, text := range messageTexts {
		previews = append(previews, "“"+clippedMessagePreview(text)+"”")
	}
	preview := strings.Join(previews, "\n")
	if totalMessageCount > len(messageTexts) {
		preview += "\n(+" + strconv.Itoa(totalMessageCount-len(messageTexts)) + ")"
	}
	return preview
}

func clippedMessagePreview(text string) string {
	runes := []rune(strings.Join(strings.Fields(text), " "))
	if len(runes) <= messageDeletePreviewCharacterLimit {
		return string(runes)
	}
	return string(runes[:messageDeletePreviewCharacterLimit]) + "…"
}
