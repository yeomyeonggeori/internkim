package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type flowTaskListInput struct {
	Query            string `json:"query"`
	TargetPersonHint string `json:"targetPersonHint"`
	WeekCode         string `json:"weekCode"`
	Status           string `json:"status"`
	Limit            int    `json:"limit"`
}

type flowTaskCompleteInput struct {
	TaskID           string `json:"taskID"`
	Query            string `json:"query"`
	TargetPersonHint string `json:"targetPersonHint"`
	WeekCode         string `json:"weekCode"`
	CompletionNote   string `json:"completionNote"`
}

type flowSummaryForTool struct {
	Week    flowWeekForTool     `json:"week"`
	Members []flowMemberForTool `json:"members"`
	Tasks   []flowTaskForTool   `json:"tasks"`
}

type flowWeekForTool struct {
	Code string `json:"code"`
}

type flowMemberForTool struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	MattermostUsername string `json:"mattermostUsername"`
}

type flowTaskForTool struct {
	ID               string   `json:"id"`
	OwnerID          string   `json:"ownerID"`
	OwnerName        string   `json:"ownerName"`
	ParticipantIDs   []string `json:"participantIDs"`
	Business         string   `json:"business"`
	Type             string   `json:"type"`
	Content          string   `json:"content"`
	Goal             string   `json:"goal"`
	Size             string   `json:"size"`
	Status           string   `json:"status"`
	StartDate        string   `json:"startDate"`
	EndDate          string   `json:"endDate"`
	WeekCode         string   `json:"weekCode"`
	Flag             int      `json:"flag"`
	RequestReason    string   `json:"requestReason"`
	DecisionReason   string   `json:"decisionReason"`
	MattermostPostID string   `json:"mattermostPostID"`
}

const flowRequesterEmailHeader = "X-InternKim-Requester-Email"

func isFlowTaskTool(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "flow.task.add", "flow.task.list", "flow.task.complete":
		return true
	default:
		return false
	}
}

func (service Service) invokeFlowTaskTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	switch strings.TrimSpace(request.ToolName) {
	case "flow.task.add":
		return service.invokeFlowTaskAdd(ctx, request)
	case "flow.task.list":
		return service.invokeFlowTaskList(ctx, request)
	case "flow.task.complete":
		return service.invokeFlowTaskComplete(ctx, request)
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("flow task tool is not configured: %s", request.ToolName)
	}
}

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

func (service Service) invokeFlowTaskList(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFlowTaskListInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchFlowSummary(ctx, request.Context.RequesterEmail, input.WeekCode)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	ownerID, failure := resolveFlowTaskFilterOwner(input.Query, input.TargetPersonHint, request.Context.RequesterEmail, summary.Members)
	if failure != nil {
		return flowTaskErrorResponse(request.ToolName, *failure), nil
	}
	tasks := filterFlowTasks(summary.Tasks, flowTaskFilter{Query: input.Query, OwnerID: ownerID, Status: input.Status, Limit: input.Limit})
	result, _ := json.Marshal(map[string]any{
		"weekCode": summary.Week.Code,
		"tasks":    tasks,
		"count":    len(tasks),
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        request.ToolName,
		Status:          "ok",
		Result:          result,
	}, nil
}

func (service Service) invokeFlowTaskComplete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFlowTaskCompleteInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchFlowSummary(ctx, request.Context.RequesterEmail, input.WeekCode)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	task, candidates, status := resolveFlowTaskCompletionTarget(input, summary.Tasks, summary.Members, request.Context.RequesterEmail)
	switch status {
	case "not_found":
		return flowTaskCompletionChoiceResponse(request.ToolName, "flow_task_not_found", "완료할 업무를 찾지 못했습니다.", candidates), nil
	case "ambiguous":
		return flowTaskCompletionChoiceResponse(request.ToolName, "flow_task_ambiguous", "완료할 업무가 여러 개입니다. 어떤 업무인지 사용자에게 선택을 요청하세요.", candidates), nil
	case "already_completed":
		result, _ := json.Marshal(map[string]any{"status": "already_completed", "task": task})
		return flowTaskSuccessResponse(request.ToolName, "already_completed", result), nil
	}
	task.Status = "완료"
	task.DecisionReason = firstNonEmptyString(strings.TrimSpace(input.CompletionNote), task.DecisionReason)
	result, errorValue := service.putFlowTask(ctx, task, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return flowTaskSuccessResponse(request.ToolName, "completed", result), nil
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

func decodeFlowTaskListInput(document json.RawMessage) (flowTaskListInput, error) {
	var input flowTaskListInput
	if len(bytes.TrimSpace(document)) > 0 {
		if errorValue := json.Unmarshal(document, &input); errorValue != nil {
			return flowTaskListInput{}, errorValue
		}
	}
	input.Query = strings.TrimSpace(input.Query)
	input.TargetPersonHint = strings.TrimSpace(input.TargetPersonHint)
	input.WeekCode = strings.TrimSpace(input.WeekCode)
	input.Status = strings.TrimSpace(input.Status)
	if input.Limit < 0 {
		input.Limit = 0
	}
	return input, nil
}

func decodeFlowTaskCompleteInput(document json.RawMessage) (flowTaskCompleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return flowTaskCompleteInput{}, fmt.Errorf("flow.task.complete input is required")
	}
	var input flowTaskCompleteInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return flowTaskCompleteInput{}, errorValue
	}
	input.TaskID = strings.TrimSpace(input.TaskID)
	input.Query = strings.TrimSpace(input.Query)
	input.TargetPersonHint = strings.TrimSpace(input.TargetPersonHint)
	input.WeekCode = strings.TrimSpace(input.WeekCode)
	input.CompletionNote = strings.TrimSpace(input.CompletionNote)
	if input.TaskID == "" && input.Query == "" {
		return flowTaskCompleteInput{}, fmt.Errorf("taskID or query is required")
	}
	return input, nil
}

func (service Service) fetchFlowMembers(ctx context.Context, requesterEmail string) ([]flowMemberForTool, error) {
	summary, errorValue := service.fetchFlowSummary(ctx, requesterEmail, "")
	if errorValue != nil {
		return nil, errorValue
	}
	return summary.Members, nil
}

func (service Service) fetchFlowSummary(ctx context.Context, requesterEmail string, weekCode string) (flowSummaryForTool, error) {
	endpoint := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + "/flow/api/summary"
	if strings.TrimSpace(weekCode) != "" {
		endpoint += "?week=" + url.QueryEscape(strings.TrimSpace(weekCode))
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if errorValue != nil {
		return flowSummaryForTool{}, errorValue
	}
	setFlowRequesterEmailHeader(httpRequest, requesterEmail)
	httpResponse, errorValue := service.httpClient().Do(httpRequest)
	if errorValue != nil {
		return flowSummaryForTool{}, errorValue
	}
	defer httpResponse.Body.Close()
	body, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return flowSummaryForTool{}, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return flowSummaryForTool{}, fmt.Errorf("flow summary failed: %s", strings.TrimSpace(string(body)))
	}
	var summary flowSummaryForTool
	if errorValue := json.Unmarshal(body, &summary); errorValue != nil {
		return flowSummaryForTool{}, errorValue
	}
	return summary, nil
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

func (service Service) putFlowTask(ctx context.Context, task flowTaskForTool, requesterEmail string) (json.RawMessage, error) {
	payload := map[string]any{
		"ownerID":        task.OwnerID,
		"participantIDs": task.ParticipantIDs,
		"category":       task.Business,
		"type":           task.Type,
		"content":        task.Content,
		"goal":           task.Goal,
		"size":           task.Size,
		"status":         task.Status,
		"startDate":      task.StartDate,
		"endDate":        task.EndDate,
		"weekCode":       task.WeekCode,
		"flag":           task.Flag,
		"requestReason":  task.RequestReason,
		"decisionReason": task.DecisionReason,
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPut, strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+"/flow/api/tasks/"+url.PathEscape(task.ID), bytes.NewReader(document))
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
		return nil, fmt.Errorf("flow task update failed: %s", strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

type flowTaskFilter struct {
	Query   string
	OwnerID string
	Status  string
	Limit   int
}

func resolveFlowTaskFilterOwner(query string, targetPersonHint string, requesterEmail string, members []flowMemberForTool) (string, *flowTaskAddFailure) {
	if strings.TrimSpace(targetPersonHint) == "" {
		return "", nil
	}
	resolution := resolveFlowOwner(flowTaskAddInput{Prompt: query, TargetPersonHint: targetPersonHint}, requesterEmail, members)
	if resolution.Failure != nil {
		return "", resolution.Failure
	}
	return resolution.OwnerID, nil
}

func resolveFlowTaskCompletionTarget(input flowTaskCompleteInput, tasks []flowTaskForTool, members []flowMemberForTool, requesterEmail string) (flowTaskForTool, []flowTaskForTool, string) {
	if input.TaskID != "" {
		for _, task := range tasks {
			if task.ID == input.TaskID {
				if task.Status == "완료" {
					return task, []flowTaskForTool{task}, "already_completed"
				}
				return task, []flowTaskForTool{task}, "matched"
			}
		}
		return flowTaskForTool{}, nil, "not_found"
	}
	ownerID, failure := resolveFlowTaskFilterOwner(input.Query, input.TargetPersonHint, requesterEmail, members)
	if failure != nil {
		return flowTaskForTool{}, nil, "ambiguous"
	}
	candidates := filterFlowTasks(tasks, flowTaskFilter{Query: input.Query, OwnerID: ownerID})
	activeCandidates := []flowTaskForTool{}
	for _, candidate := range candidates {
		if candidate.Status != "완료" {
			activeCandidates = append(activeCandidates, candidate)
		}
	}
	if len(activeCandidates) == 1 {
		return activeCandidates[0], activeCandidates, "matched"
	}
	if len(activeCandidates) > 1 {
		return flowTaskForTool{}, activeCandidates, "ambiguous"
	}
	if len(candidates) == 1 && candidates[0].Status == "완료" {
		return candidates[0], candidates, "already_completed"
	}
	if len(candidates) > 1 {
		return flowTaskForTool{}, candidates, "ambiguous"
	}
	return flowTaskForTool{}, nil, "not_found"
}

func filterFlowTasks(tasks []flowTaskForTool, filter flowTaskFilter) []flowTaskForTool {
	filteredTasks := []flowTaskForTool{}
	for _, task := range tasks {
		if filter.OwnerID != "" && task.OwnerID != filter.OwnerID {
			continue
		}
		if filter.Status != "" && task.Status != filter.Status {
			continue
		}
		if filter.Query != "" && !flowTaskMatchesQuery(task, filter.Query) {
			continue
		}
		filteredTasks = append(filteredTasks, task)
		if filter.Limit > 0 && len(filteredTasks) >= filter.Limit {
			break
		}
	}
	return filteredTasks
}

func flowTaskMatchesQuery(task flowTaskForTool, query string) bool {
	normalizedQuery := normalizeFlowTaskSearchText(query)
	if normalizedQuery == "" {
		return true
	}
	values := []string{task.Content, task.Goal, task.Business, task.Type, task.OwnerName, task.Status}
	for _, value := range values {
		if strings.Contains(normalizeFlowTaskSearchText(value), normalizedQuery) {
			return true
		}
	}
	return false
}

func normalizeFlowTaskSearchText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), "")
}

func flowTaskCompletionChoiceResponse(toolName string, errorCode string, message string, candidates []flowTaskForTool) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(map[string]any{"status": errorCode, "message": message, "candidates": candidates})
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Status:          errorCode,
		Result:          result,
		Content:         message,
		IsError:         true,
		ErrorCode:       errorCode,
		FailureStage:    "flow_task_resolution",
		Retryable:       false,
		SafeRetry:       false,
	}
}

func flowTaskErrorResponse(toolName string, failure flowTaskAddFailure) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(failure)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Status:          "error",
		Result:          result,
		Content:         failure.Message,
		IsError:         true,
		ErrorCode:       failure.ErrorCode,
		FailureStage:    failure.FailureStage,
		Retryable:       failure.Retryable,
		SafeRetry:       failure.SafeRetry,
	}
}

func flowTaskSuccessResponse(toolName string, status string, result json.RawMessage) capabilities.ToolInvokeResponse {
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Status:          status,
		Result:          result,
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return trimmedValue
		}
	}
	return ""
}

func setFlowRequesterEmailHeader(request *http.Request, requesterEmail string) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(requesterEmail))
	if normalizedEmail == "" {
		return
	}
	request.Header.Set(flowRequesterEmailHeader, normalizedEmail)
}
