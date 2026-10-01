package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const (
	tellDirectMessagePath     = "/tell/api/direct-message"
	tellDirectMessageToolName = "message_send"
	tellDirectMessageSource   = "plane_telling"
	tellDirectMessageCeiling  = 1 << 18
)

type tellDirectMessageRequest struct {
	RecipientEmail string `json:"recipientEmail"`
	Message        string `json:"message"`
}

func (service *Service) handleTellDirectMessage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(responseWriter, "a telling is delivered with POST", http.StatusMethodNotAllowed)
		return
	}
	if !arrivedOnRequesterSocket(request) {
		http.Error(responseWriter, "a telling is delivered over the socket the relay holds", http.StatusForbidden)
		return
	}
	var telling tellDirectMessageRequest
	body := http.MaxBytesReader(responseWriter, request.Body, tellDirectMessageCeiling)
	if errorValue := decodeOptionalJSONBody(body, &telling); errorValue != nil {
		http.Error(responseWriter, "that is not a telling this device can read", http.StatusBadRequest)
		return
	}
	telling.RecipientEmail = strings.ToLower(strings.TrimSpace(telling.RecipientEmail))
	telling.Message = strings.TrimSpace(telling.Message)
	if telling.RecipientEmail == "" || telling.Message == "" {
		http.Error(responseWriter, "a telling names a recipient and carries a message", http.StatusBadRequest)
		return
	}

	toolRequest, isMember, errorValue := service.tellDirectMessageToolRequest(request.Context(), telling)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if !isMember {
		http.Error(responseWriter, telling.RecipientEmail+" is no active member this company can tell", http.StatusNotFound)
		return
	}

	response, errorValue := service.invokeCapabilityTool(request.Context(), toolRequest)
	service.writeCapabilityResponse(responseWriter, response, errorValue)
}

func (service *Service) tellDirectMessageToolRequest(ctx context.Context, telling tellDirectMessageRequest) (capabilities.ToolInvokeRequest, bool, error) {
	actor, isMember, errorValue := service.resolveUserActorByEmail(ctx, telling.RecipientEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeRequest{}, false, errorValue
	}
	if !isMember || strings.TrimSpace(actor.UserID) == "" {
		return capabilities.ToolInvokeRequest{}, false, nil
	}
	descriptor, isServed, errorValue := service.publicToolDescriptor(ctx, tellDirectMessageToolName)
	if errorValue != nil {
		return capabilities.ToolInvokeRequest{}, false, errorValue
	}
	if !isServed {
		return capabilities.ToolInvokeRequest{}, false, fmt.Errorf("%s is not a tool this device serves", tellDirectMessageToolName)
	}
	input, errorValue := json.Marshal(map[string]string{
		"targetType": "directMessage",
		"personHint": telling.RecipientEmail,
		"message":    telling.Message,
	})
	if errorValue != nil {
		return capabilities.ToolInvokeRequest{}, false, errorValue
	}
	toolActor := tellDirectMessageActorContext(actor)
	return capabilities.ToolInvokeRequest{
		ToolName:             tellDirectMessageToolName,
		Input:                input,
		Actor:                toolActor,
		Context:              tellDirectMessageInvokeContext(toolActor),
		PrivacyClass:         descriptor.PrivacyClass,
		RequiresUserPresence: descriptor.RequiresUserPresence,
	}, true, nil
}

func tellDirectMessageActorContext(actor userActor) capabilities.ActorContext {
	return capabilities.ActorContext{
		PersonID:    strings.TrimSpace(actor.UserID),
		Email:       strings.ToLower(strings.TrimSpace(actor.Email)),
		DisplayName: strings.TrimSpace(actor.Name),
		Source:      tellDirectMessageSource,
		Scopes:      normalizePublicAPIPermissions([]string{publicAPIPermissionWrite}),
		IsAdmin:     actor.isAdmin(),
	}
}

func tellDirectMessageInvokeContext(actor capabilities.ActorContext) capabilities.ToolInvokeContext {
	return capabilities.ToolInvokeContext{
		RequesterPersonID:      actor.PersonID,
		RequesterEmail:         actor.Email,
		RequesterName:          actor.DisplayName,
		TaskSource:             tellDirectMessageSource,
		IsApprovalContinuation: true,
	}
}
