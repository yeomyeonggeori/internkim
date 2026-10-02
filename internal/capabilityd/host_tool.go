package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
)

const (
	hostUpdateToolName      = "host_update"
	admindHostVersionPath   = "/host/api/version"
	admindHostUpdatePath    = "/host/api/update"
	admindHostUpdatePlanURL = "/host/api/update/plan"
)

type hostUpdateRequester struct {
	PersonID       string `json:"personID,omitempty"`
	Platform       string `json:"platform,omitempty"`
	ConversationID string `json:"conversationID,omitempty"`
	ReplyTargetID  string `json:"replyTargetID,omitempty"`
}

type hostUpdateStart struct {
	Input      json.RawMessage     `json:"input"`
	Requester  hostUpdateRequester `json:"requester"`
	IsApproved bool                `json:"isApproved"`
}

func (service Service) invokeHostVersionTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	return service.answerThroughAdmindAsTheRequester(ctx, request, admindHostVersionPath, []byte("{}"))
}

func (service Service) invokeHostUpdateTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	body, errorValue := json.Marshal(hostUpdateStart{
		Input:      inputOrEmptyObject(request.Input),
		Requester:  hostUpdateRequesterOf(request.Context),
		IsApproved: isHostUpdateApproved(request),
	})
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return service.answerThroughAdmindAsTheRequester(ctx, request, admindHostUpdatePath, body)
}

func isHostUpdateApproved(request capabilities.ToolInvokeRequest) bool {
	if requesterApprovedThisCall(request.Context) {
		return true
	}
	scheduledCall := request.Context.ScheduledApprovedCall
	return scheduledCall != nil && strings.TrimSpace(scheduledCall.ToolName) == hostUpdateToolName &&
		isSameJSONDocument(scheduledCall.ToolInput, request.Input)
}

func isSameJSONDocument(left json.RawMessage, right json.RawMessage) bool {
	var leftDocument, rightDocument any
	if json.Unmarshal(inputOrEmptyObject(left), &leftDocument) != nil || json.Unmarshal(inputOrEmptyObject(right), &rightDocument) != nil {
		return false
	}
	leftCanonical, _ := json.Marshal(leftDocument)
	rightCanonical, _ := json.Marshal(rightDocument)
	return bytes.Equal(leftCanonical, rightCanonical)
}

func hostUpdateRequesterOf(toolContext capabilities.ToolInvokeContext) hostUpdateRequester {
	return hostUpdateRequester{
		PersonID:       toolContext.RequesterPersonID,
		Platform:       toolContext.Platform,
		ConversationID: toolContext.ConversationID,
		ReplyTargetID:  toolContext.ReplyTargetID,
	}
}

func inputOrEmptyObject(input json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(input)) == 0 {
		return json.RawMessage("{}")
	}
	return input
}

func (service Service) resolveHostUpdateTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, admindRequesterURL(admindHostUpdatePlanURL), bytes.NewReader(inputOrEmptyObject(request.Input)))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, errorValue := service.askAdmindAsTheRequester(httpRequest, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	defer response.Body.Close()
	answer, errorValue := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return capabilityToolFailure(admindToolOrigin, request.ToolName, response.StatusCode, json.RawMessage(answer)), nil
	}
	return hostUpdateTargetResponse(request.ToolName, answer), nil
}

func hostUpdateTargetResponse(toolName string, answer []byte) capabilities.ToolInvokeResponse {
	var target capabilities.ApprovalTarget
	json.Unmarshal(answer, &target)
	return capabilities.ToolInvokeResponse{
		Provider:        admindToolOrigin.Provider,
		SelectedBackend: admindToolOrigin.SelectedBackend,
		ToolName:        strings.TrimSpace(toolName),
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "resolved",
		Content:         target.Title,
		Result:          json.RawMessage(answer),
	}
}
