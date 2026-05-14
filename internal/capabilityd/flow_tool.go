package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type flowTaskAddInput struct {
	Prompt           string `json:"prompt"`
	TargetPersonHint string `json:"targetPersonHint"`
	WeekCode         string `json:"weekCode"`
	AllowDuplicate   bool   `json:"allowDuplicate"`
}

type flowSummaryForTool struct {
	Members []flowMemberForTool `json:"members"`
}

type flowMemberForTool struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	MattermostUsername string `json:"mattermostUsername"`
}

const flowRequesterEmailHeader = "X-InternKim-Requester-Email"

func (service Service) invokeFlowTaskAdd(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFlowTaskAddInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	members, errorValue := service.fetchFlowMembers(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	ownerResolution := resolveFlowOwner(input, request.Context.RequesterEmail, members)
	if ownerResolution.Failure != nil {
		return flowTaskAddErrorResponse(request.ToolName, *ownerResolution.Failure), nil
	}
	payload := map[string]any{
		"prompt":         input.Prompt,
		"ownerID":        ownerResolution.OwnerID,
		"weekCode":       input.WeekCode,
		"requesterEmail": request.Context.RequesterEmail,
		"source":         "chat",
		"allowDuplicate": input.AllowDuplicate,
	}
	result, errorValue := service.postFlowTask(ctx, payload, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        request.ToolName,
		Status:          flowTaskAddResponseStatus(result),
		Result:          result,
	}, nil
}

type flowTaskAddFailure struct {
	ErrorCode    string                 `json:"errorCode"`
	FailureStage string                 `json:"failureStage"`
	Message      string                 `json:"message"`
	Candidates   []flowTaskAddCandidate `json:"candidates,omitempty"`
	Retryable    bool                   `json:"retryable"`
	SafeRetry    bool                   `json:"safeRetry"`
}

type flowTaskAddCandidate struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	MattermostUsername string `json:"mattermostUsername,omitempty"`
	Mention            string `json:"mention,omitempty"`
}

func flowTaskAddResponseStatus(result json.RawMessage) string {
	var response struct {
		Status string `json:"status"`
	}
	if errorValue := json.Unmarshal(result, &response); errorValue != nil {
		return "ok"
	}
	if strings.TrimSpace(response.Status) == "" {
		return "ok"
	}
	return strings.TrimSpace(response.Status)
}

func decodeFlowTaskAddInput(document json.RawMessage) (flowTaskAddInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return flowTaskAddInput{}, fmt.Errorf("flow.task.add input is required")
	}
	var input flowTaskAddInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return flowTaskAddInput{}, errorValue
	}
	input.Prompt = strings.TrimSpace(input.Prompt)
	input.TargetPersonHint = strings.TrimSpace(input.TargetPersonHint)
	input.WeekCode = strings.TrimSpace(input.WeekCode)
	if input.Prompt == "" {
		return flowTaskAddInput{}, fmt.Errorf("prompt is required")
	}
	return input, nil
}

func (service Service) fetchFlowMembers(ctx context.Context, requesterEmail string) ([]flowMemberForTool, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+"/flow/api/summary", nil)
	if errorValue != nil {
		return nil, errorValue
	}
	setFlowRequesterEmailHeader(httpRequest, requesterEmail)
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	body, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return nil, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("flow summary failed: %s", strings.TrimSpace(string(body)))
	}
	var summary flowSummaryForTool
	if errorValue := json.Unmarshal(body, &summary); errorValue != nil {
		return nil, errorValue
	}
	return summary.Members, nil
}

func flowTaskAddErrorResponse(toolName string, failure flowTaskAddFailure) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(failure)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Status:          "error",
		Content:         failure.Message,
		IsError:         true,
		Message:         failure.Message,
		ErrorCode:       failure.ErrorCode,
		FailureStage:    failure.FailureStage,
		Retryable:       failure.Retryable,
		SafeRetry:       failure.SafeRetry,
		Result:          result,
	}
}

func (service Service) postFlowTask(ctx context.Context, payload map[string]any, requesterEmail string) (json.RawMessage, error) {
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+"/flow/api/tasks/quick", bytes.NewReader(document))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	setFlowRequesterEmailHeader(httpRequest, requesterEmail)
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	body, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return nil, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("flow task add failed: %s", strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

func setFlowRequesterEmailHeader(request *http.Request, requesterEmail string) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(requesterEmail))
	if normalizedEmail == "" {
		return
	}
	request.Header.Set(flowRequesterEmailHeader, normalizedEmail)
}
