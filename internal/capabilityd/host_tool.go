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

	publicAPIHeldUpdateRefusal = `{"errorCode":"scheduled_start_unsupported","failureStage":"precondition","message":"a host update called through the API starts when it is called, and the API holds none for later; leave startsAt out to update now, or ask the agent, which offers a later time"}`
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
	if asksThePublicAPIToHoldTheUpdate(request) {
		return capabilityToolFailure(admindToolOrigin, request.ToolName, http.StatusBadRequest, json.RawMessage(publicAPIHeldUpdateRefusal)), nil
	}
	return service.answerThroughAdmindAsTheRequester(ctx, request, admindHostUpdatePath, hostUpdateBody(request))
}

func asksThePublicAPIToHoldTheUpdate(request capabilities.ToolInvokeRequest) bool {
	if request.Context.TaskSource != capabilities.TaskSourcePublicAPI {
		return false
	}
	var input struct {
		StartsAt string `json:"startsAt"`
	}
	json.Unmarshal(request.Input, &input)
	return strings.TrimSpace(input.StartsAt) != ""
}

func hostUpdateBody(request capabilities.ToolInvokeRequest) []byte {
	input := request.Input
	if len(bytes.TrimSpace(input)) == 0 {
		input = json.RawMessage("{}")
	}
	scheduledCall := request.Context.ScheduledApprovedCall
	body, _ := json.Marshal(hostUpdateStart{
		Input: input,
		Requester: hostUpdateRequester{
			PersonID:       request.Context.RequesterPersonID,
			Platform:       request.Context.Platform,
			ConversationID: request.Context.ConversationID,
			ReplyTargetID:  request.Context.ReplyTargetID,
		},
		IsApproved: requesterApprovedThisCall(request.Context) || (scheduledCall != nil && strings.TrimSpace(scheduledCall.ToolName) == hostUpdateToolName),
	})
	return body
}

func (service Service) resolveHostUpdateTarget(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, admindRequesterURL(admindHostUpdatePlanURL), bytes.NewReader(hostUpdateBody(request)))
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
	var target capabilities.ApprovalTarget
	json.Unmarshal(answer, &target)
	resolved := capabilityToolTargetResponse(request.ToolName, target)
	resolved.Result = json.RawMessage(answer)
	return resolved, nil
}
