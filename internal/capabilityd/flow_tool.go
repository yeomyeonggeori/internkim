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

type flowTaskUpdateInput struct {
	TaskID           string  `json:"taskID"`
	Query            string  `json:"query"`
	TargetPersonHint string  `json:"targetPersonHint"`
	WeekCode         string  `json:"weekCode"`
	Content          *string `json:"content"`
	Goal             *string `json:"goal"`
	Status           *string `json:"status"`
	Size             *string `json:"size"`
	Category         *string `json:"category"`
	Type             *string `json:"type"`
	StartDate        *string `json:"startDate"`
	EndDate          *string `json:"endDate"`
	Flag             *int    `json:"flag"`
	RequestReason    *string `json:"requestReason"`
	DecisionReason   *string `json:"decisionReason"`
}

type flowTaskDeleteInput struct {
	TaskID           string `json:"taskID"`
	Query            string `json:"query"`
	TargetPersonHint string `json:"targetPersonHint"`
	WeekCode         string `json:"weekCode"`
}

type flowSummaryForTool struct {
	Week        flowWeekForTool     `json:"week"`
	Members     []flowMemberForTool `json:"members"`
	Tasks       []flowTaskForTool   `json:"tasks"`
	WeeklyTasks []flowTaskForTool   `json:"weeklyTasks"`
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
	ParticipantNames []string `json:"participantNames"`
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
	case "flow.task.add", "flow.task.list", "flow.task.update", "flow.task.delete":
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
	case "flow.task.update":
		return service.invokeFlowTaskUpdate(ctx, request)
	case "flow.task.delete":
		return service.invokeFlowTaskDelete(ctx, request)
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
		Status:          flowTaskResponseStatus(result),
		Result:          result,
	}, nil
}

func (service Service) invokeFlowTaskUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFlowTaskUpdateInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchFlowSummary(ctx, request.Context.RequesterEmail, input.WeekCode)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	task, failure := resolveFlowTaskUpdateTarget(input, summary)
	if failure != nil {
		return flowTaskUpdateErrorResponse(request.ToolName, *failure), nil
	}
	updatedTask := applyFlowTaskUpdateInput(task, input)
	result, errorValue := service.putFlowTask(ctx, updatedTask, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        request.ToolName,
		Status:          flowTaskResponseStatus(result),
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

func (service Service) invokeFlowTaskDelete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFlowTaskDeleteInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchFlowSummary(ctx, request.Context.RequesterEmail, input.WeekCode)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	task, failure := resolveFlowTaskDeleteTarget(input, summary)
	if failure != nil {
		return flowTaskUpdateErrorResponse(request.ToolName, *failure), nil
	}
	result, errorValue := service.deleteFlowTask(ctx, task.ID, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        request.ToolName,
		Status:          flowTaskResponseStatus(result),
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

type flowTaskUpdateFailure struct {
	ErrorCode    string                    `json:"errorCode"`
	FailureStage string                    `json:"failureStage"`
	Message      string                    `json:"message"`
	Candidates   []flowTaskUpdateCandidate `json:"candidates,omitempty"`
	Retryable    bool                      `json:"retryable"`
	SafeRetry    bool                      `json:"safeRetry"`
}

type flowTaskUpdateCandidate struct {
	ID          string `json:"id"`
	OwnerName   string `json:"ownerName"`
	Content     string `json:"content"`
	Status      string `json:"status"`
	WeekCode    string `json:"weekCode"`
	StartDate   string `json:"startDate,omitempty"`
	EndDate     string `json:"endDate,omitempty"`
	MatchReason string `json:"matchReason,omitempty"`
}

func flowTaskResponseStatus(result json.RawMessage) string {
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

func decodeFlowTaskUpdateInput(document json.RawMessage) (flowTaskUpdateInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return flowTaskUpdateInput{}, fmt.Errorf("flow.task.update input is required")
	}
	var input flowTaskUpdateInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return flowTaskUpdateInput{}, errorValue
	}
	input.TaskID = strings.TrimSpace(input.TaskID)
	input.Query = strings.TrimSpace(input.Query)
	input.TargetPersonHint = strings.TrimSpace(input.TargetPersonHint)
	input.WeekCode = strings.TrimSpace(input.WeekCode)
	trimStringPointer(&input.Content)
	trimStringPointer(&input.Goal)
	trimStringPointer(&input.Status)
	trimStringPointer(&input.Size)
	trimStringPointer(&input.Category)
	trimStringPointer(&input.Type)
	trimStringPointer(&input.StartDate)
	trimStringPointer(&input.EndDate)
	trimStringPointer(&input.RequestReason)
	trimStringPointer(&input.DecisionReason)
	if input.TaskID == "" && input.Query == "" {
		return flowTaskUpdateInput{}, fmt.Errorf("taskID or query is required")
	}
	return input, nil
}

func decodeFlowTaskDeleteInput(document json.RawMessage) (flowTaskDeleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return flowTaskDeleteInput{}, fmt.Errorf("flow.task.delete input is required")
	}
	var input flowTaskDeleteInput
	if errorValue := json.Unmarshal(document, &input); errorValue != nil {
		return flowTaskDeleteInput{}, errorValue
	}
	input.TaskID = strings.TrimSpace(input.TaskID)
	input.Query = strings.TrimSpace(input.Query)
	input.TargetPersonHint = strings.TrimSpace(input.TargetPersonHint)
	input.WeekCode = strings.TrimSpace(input.WeekCode)
	if input.TaskID == "" && input.Query == "" {
		return flowTaskDeleteInput{}, fmt.Errorf("taskID or query is required")
	}
	return input, nil
}

func trimStringPointer(value **string) {
	if value == nil || *value == nil {
		return
	}
	trimmedValue := strings.TrimSpace(**value)
	*value = &trimmedValue
}

func (service Service) fetchFlowMembers(ctx context.Context, requesterEmail string) ([]flowMemberForTool, error) {
	summary, errorValue := service.fetchFlowSummary(ctx, requesterEmail, "")
	if errorValue != nil {
		return nil, errorValue
	}
	return summary.Members, nil
}

func (service Service) fetchFlowSummary(ctx context.Context, requesterEmail string, weekCode string) (flowSummaryForTool, error) {
	requestURL := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + "/flow/api/summary"
	if strings.TrimSpace(weekCode) != "" {
		requestURL += "?week=" + url.QueryEscape(strings.TrimSpace(weekCode))
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
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
	if len(summary.Tasks) == 0 && len(summary.WeeklyTasks) > 0 {
		summary.Tasks = summary.WeeklyTasks
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

func flowTaskUpdateErrorResponse(toolName string, failure flowTaskUpdateFailure) capabilities.ToolInvokeResponse {
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

func (service Service) deleteFlowTask(ctx context.Context, taskID string, requesterEmail string) (json.RawMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodDelete, strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+"/flow/api/tasks/"+url.PathEscape(taskID), nil)
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
		return nil, fmt.Errorf("flow task delete failed: %s", strings.TrimSpace(string(body)))
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

func setFlowRequesterEmailHeader(request *http.Request, requesterEmail string) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(requesterEmail))
	if normalizedEmail == "" {
		return
	}
	request.Header.Set(flowRequesterEmailHeader, normalizedEmail)
}

func resolveFlowTaskUpdateTarget(input flowTaskUpdateInput, summary flowSummaryForTool) (flowTaskForTool, *flowTaskUpdateFailure) {
	candidates := flowTaskUpdateCandidates(input, summary)
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	failure := flowTaskUpdateFailure{
		ErrorCode:    "flow_task_not_found",
		FailureStage: "target_resolution",
		Message:      "flow.task.update target was not found; ask the user to identify the task",
		Retryable:    true,
		SafeRetry:    true,
	}
	if len(candidates) > 1 {
		failure.ErrorCode = "flow_task_ambiguous"
		failure.Message = "flow.task.update target is ambiguous; ask the user to choose one candidate"
		failure.Candidates = flowTaskUpdateCandidateSummaries(candidates, "matched")
	}
	return flowTaskForTool{}, &failure
}

func resolveFlowTaskDeleteTarget(input flowTaskDeleteInput, summary flowSummaryForTool) (flowTaskForTool, *flowTaskUpdateFailure) {
	return resolveFlowTaskUpdateTarget(flowTaskUpdateInput{
		TaskID:           input.TaskID,
		Query:            input.Query,
		TargetPersonHint: input.TargetPersonHint,
		WeekCode:         input.WeekCode,
	}, summary)
}

func flowTaskUpdateCandidates(input flowTaskUpdateInput, summary flowSummaryForTool) []flowTaskForTool {
	tasks := filterFlowTasksByPerson(summary.Tasks, input.TargetPersonHint, summary.Members)
	if input.TaskID != "" {
		return matchingFlowTasksByID(tasks, input.TaskID)
	}
	return matchingFlowTasksByQuery(tasks, input.Query)
}

func filterFlowTasksByPerson(tasks []flowTaskForTool, targetPersonHint string, members []flowMemberForTool) []flowTaskForTool {
	if strings.TrimSpace(targetPersonHint) == "" {
		return tasks
	}
	matches := matchingFlowMembers(targetPersonHint, members)
	if len(matches) == 0 {
		return []flowTaskForTool{}
	}
	memberIDs := map[string]bool{}
	for _, member := range matches {
		memberIDs[member.ID] = true
	}
	filteredTasks := []flowTaskForTool{}
	for _, task := range tasks {
		if memberIDs[task.OwnerID] || intersectsFlowMemberIDs(memberIDs, task.ParticipantIDs) {
			filteredTasks = append(filteredTasks, task)
		}
	}
	return filteredTasks
}

func matchingFlowTasksByID(tasks []flowTaskForTool, taskID string) []flowTaskForTool {
	matches := []flowTaskForTool{}
	for _, task := range tasks {
		if strings.EqualFold(strings.TrimSpace(task.ID), strings.TrimSpace(taskID)) {
			matches = append(matches, task)
		}
	}
	return matches
}

func matchingFlowTasksByQuery(tasks []flowTaskForTool, query string) []flowTaskForTool {
	normalizedQuery := normalizeFlowSearchText(query)
	matches := []flowTaskForTool{}
	for _, task := range tasks {
		if strings.Contains(normalizeFlowSearchText(flowTaskSearchDocument(task)), normalizedQuery) {
			matches = append(matches, task)
		}
	}
	return matches
}

func flowTaskSearchDocument(task flowTaskForTool) string {
	return strings.Join([]string{
		task.ID,
		task.OwnerName,
		strings.Join(task.ParticipantNames, " "),
		task.Business,
		task.Type,
		task.Content,
		task.Goal,
		task.Size,
		task.Status,
		task.StartDate,
		task.EndDate,
		task.WeekCode,
		task.RequestReason,
		task.DecisionReason,
	}, " ")
}

func normalizeFlowSearchText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func intersectsFlowMemberIDs(memberIDs map[string]bool, taskMemberIDs []string) bool {
	for _, taskMemberID := range taskMemberIDs {
		if memberIDs[taskMemberID] {
			return true
		}
	}
	return false
}

func flowTaskUpdateCandidateSummaries(tasks []flowTaskForTool, reason string) []flowTaskUpdateCandidate {
	candidates := make([]flowTaskUpdateCandidate, 0, len(tasks))
	for _, task := range tasks {
		candidates = append(candidates, flowTaskUpdateCandidate{
			ID:          task.ID,
			OwnerName:   task.OwnerName,
			Content:     task.Content,
			Status:      task.Status,
			WeekCode:    task.WeekCode,
			StartDate:   task.StartDate,
			EndDate:     task.EndDate,
			MatchReason: reason,
		})
	}
	return candidates
}

func applyFlowTaskUpdateInput(task flowTaskForTool, input flowTaskUpdateInput) flowTaskForTool {
	if !hasFlowTaskUpdatePatch(input) {
		task.Status = "완료"
		return task
	}
	if input.Content != nil {
		task.Content = *input.Content
	}
	if input.Goal != nil {
		task.Goal = *input.Goal
	}
	if input.Status != nil {
		task.Status = *input.Status
	}
	if input.Size != nil {
		task.Size = *input.Size
	}
	if input.Category != nil {
		task.Business = *input.Category
	}
	if input.Type != nil {
		task.Type = *input.Type
	}
	if input.StartDate != nil {
		task.StartDate = *input.StartDate
	}
	if input.EndDate != nil {
		task.EndDate = *input.EndDate
	}
	if input.Flag != nil {
		task.Flag = *input.Flag
	}
	if input.RequestReason != nil {
		task.RequestReason = *input.RequestReason
	}
	if input.DecisionReason != nil {
		task.DecisionReason = *input.DecisionReason
	}
	return task
}

func hasFlowTaskUpdatePatch(input flowTaskUpdateInput) bool {
	return input.Content != nil ||
		input.Goal != nil ||
		input.Status != nil ||
		input.Size != nil ||
		input.Category != nil ||
		input.Type != nil ||
		input.StartDate != nil ||
		input.EndDate != nil ||
		input.Flag != nil ||
		input.RequestReason != nil ||
		input.DecisionReason != nil
}
