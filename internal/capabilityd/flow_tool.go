package capabilityd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
)

type flowTaskAddInput struct {
	Title                  string   `json:"title"`
	Goal                   string   `json:"goal"`
	Size                   string   `json:"size"`
	Status                 string   `json:"status"`
	StartDate              string   `json:"startDate"`
	EndDate                string   `json:"endDate"`
	TargetPersonHint       string   `json:"targetPersonHint"`
	ParticipantPersonHints []string `json:"participantPersonHints"`
}

type flowTaskCreatePayload struct {
	OwnerID        string   `json:"ownerID"`
	ParticipantIDs []string `json:"participantIDs"`
	Content        string   `json:"content"`
	Goal           string   `json:"goal,omitempty"`
	Size           string   `json:"size,omitempty"`
	Status         string   `json:"status,omitempty"`
	StartDate      string   `json:"startDate,omitempty"`
	EndDate        string   `json:"endDate,omitempty"`
}

type flowTaskListInput struct {
	Query            string            `json:"query"`
	TargetPersonHint string            `json:"targetPersonHint"`
	Scope            flowTaskListScope `json:"scope"`
	WeekFrom         int               `json:"weekFrom"`
	WeekTo           int               `json:"weekTo"`
	Status           string            `json:"status"`
	Limit            int               `json:"limit"`
}

type flowTaskListScope string

const (
	flowTaskListScopeSelf flowTaskListScope = "self"
	flowTaskListScopeAll  flowTaskListScope = "all"
)

type flowTaskUpdateInput struct {
	TaskID         string  `json:"taskID"`
	Title          *string `json:"title"`
	Goal           *string `json:"goal"`
	Status         *string `json:"status"`
	Size           *string `json:"size"`
	Category       *string `json:"category"`
	Type           *string `json:"type"`
	StartDate      *string `json:"startDate"`
	EndDate        *string `json:"endDate"`
	Flag           *int    `json:"flag"`
	RequestReason  *string `json:"requestReason"`
	DecisionReason *string `json:"decisionReason"`
}

type flowTaskDeleteInput struct {
	TaskID string `json:"taskID"`
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
	ID                       string                      `json:"id"`
	OwnerID                  string                      `json:"ownerID"`
	OwnerName                string                      `json:"ownerName"`
	ParticipantIDs           []string                    `json:"participantIDs"`
	ParticipantNames         []string                    `json:"participantNames"`
	ParticipantPresentations []personPresentationForTool `json:"participantPresentations,omitempty"`
	Business                 string                      `json:"business"`
	Type                     string                      `json:"type"`
	Content                  string                      `json:"content"`
	Goal                     string                      `json:"goal"`
	Size                     string                      `json:"size"`
	Status                   string                      `json:"status"`
	StartDate                string                      `json:"startDate"`
	EndDate                  string                      `json:"endDate"`
	WeekCode                 string                      `json:"weekCode"`
	Flag                     int                         `json:"flag"`
	RequestReason            string                      `json:"requestReason"`
	DecisionReason           string                      `json:"decisionReason"`
	MattermostPostID         string                      `json:"mattermostPostID"`
}

const flowRequesterEmailHeader = "X-InternKim-Requester-Email"

var flowTaskDeleteNotFoundError = errors.New("flow task delete target not found")

func (service Service) invokeFlowTaskTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	switch strings.TrimSpace(request.ToolName) {
	case "task.add":
		return service.invokeFlowTaskAdd(ctx, request)
	case "task.list":
		return service.invokeFlowTaskList(ctx, request)
	case "task.update":
		return service.invokeFlowTaskUpdate(ctx, request)
	case "task.delete":
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
	participantIDs, participantFailure := resolveFlowParticipantIDs(input.ParticipantPersonHints, ownerResolution.OwnerID, members)
	if participantFailure != nil {
		return flowTaskAddErrorResponse(request.ToolName, *participantFailure), nil
	}
	payload := flowTaskCreatePayload{
		OwnerID:        ownerResolution.OwnerID,
		ParticipantIDs: participantIDs,
		Content:        input.Title,
		Goal:           input.Goal,
		Size:           input.Size,
		Status:         input.Status,
		StartDate:      input.StartDate,
		EndDate:        input.EndDate,
	}
	result, errorValue := service.postFlowTask(ctx, payload, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if flowTaskDuplicateID(result) != "" {
		return flowTaskUpdateErrorResponse(request.ToolName, flowTaskDuplicateFailure()), nil
	}
	result = enrichFlowTaskResultDocument(result, members)
	taskID := flowTaskResultID(result)
	if taskID == "" {
		return flowTaskUpdateErrorResponse(request.ToolName, flowTaskInvalidResultFailure(request.ToolName)), nil
	}
	return capabilitySuccessResponse(request.ToolName, flowTaskResponseStatus(result), result)
}

func (service Service) invokeFlowTaskUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFlowTaskUpdateInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchFlowAllTasks(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if len(summary.Members) == 0 {
		if members, membersError := service.fetchFlowMembers(ctx, request.Context.RequesterEmail); membersError == nil {
			summary.Members = members
		}
	}
	task, failure := resolveFlowTaskUpdateTarget(input.TaskID, summary.Tasks)
	if failure != nil {
		return flowTaskUpdateErrorResponse(request.ToolName, *failure), nil
	}
	updatedTask := applyFlowTaskUpdateInput(task, input)
	result, errorValue := service.putFlowTask(ctx, updatedTask, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result = enrichFlowTaskResultDocument(result, summary.Members)
	taskID := flowTaskResultID(result)
	if taskID != input.TaskID || !flowTaskUpdateResultMatchesInput(result, input) {
		return flowTaskUpdateErrorResponse(request.ToolName, flowTaskInvalidResultFailure(request.ToolName)), nil
	}
	return capabilitySuccessResponse(request.ToolName, flowTaskResponseStatus(result), result)
}

func (service Service) invokeFlowTaskList(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFlowTaskListInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchFlowAllTasks(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if len(summary.Members) == 0 {
		if members, membersError := service.fetchFlowMembers(ctx, request.Context.RequesterEmail); membersError == nil {
			summary.Members = members
		}
	}
	ownerID, failure := resolveFlowTaskListOwner(input, request.Context.RequesterEmail, summary.Members)
	if failure != nil {
		return flowTaskErrorResponse(request.ToolName, *failure), nil
	}
	statusFilter := normalizeFlowStatusFilter(input.Status)
	weekCodes, errorValue := flowTaskListWeekCodes(input.WeekFrom, input.WeekTo, summary.Week.Code)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	filteredTasks := filterFlowTasks(summary.Tasks, flowTaskFilter{Query: input.Query, MemberID: ownerID, Status: statusFilter, WeekCodes: weekCodes, CurrentWeekCode: summary.Week.Code, Limit: input.Limit})
	tasks := enrichFlowTasksForTool(filteredTasks, summary.Members)
	result, _ := json.Marshal(map[string]any{
		"scope":        flowTaskListPeopleScope(ownerID),
		"weekFrom":     input.WeekFrom,
		"weekTo":       input.WeekTo,
		"statusFilter": statusFilter,
		"ownerID":      ownerID,
		"tasks":        tasks,
		"count":        len(tasks),
	})
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        request.ToolName,
		Outcome:         capabilities.ToolOutcomeSucceeded,
		Status:          "ok",
		Result:          result,
	}, nil
}

func (service Service) invokeFlowTaskDelete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeFlowTaskDeleteInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result, errorValue := service.deleteFlowTask(ctx, input.TaskID, request.Context.RequesterEmail)
	if errors.Is(errorValue, flowTaskDeleteNotFoundError) {
		return flowTaskUpdateErrorResponse(request.ToolName, flowTaskNotFoundFailure(request.ToolName)), nil
	}
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if !flowTaskDeleteEvidenceMatchesTaskID(result, input.TaskID) {
		return flowTaskUpdateErrorResponse(request.ToolName, flowTaskNotFoundFailure(request.ToolName)), nil
	}
	result, _ = json.Marshal(map[string]any{"taskID": input.TaskID, "deleted": true})
	return capabilitySuccessResponse(request.ToolName, "deleted", result)
}

func flowTaskResultID(result json.RawMessage) string {
	var document struct {
		TaskID string `json:"taskID"`
	}
	json.Unmarshal(result, &document)
	return strings.TrimSpace(document.TaskID)
}

func flowTaskUpdateResultMatchesInput(result json.RawMessage, input flowTaskUpdateInput) bool {
	var task flowTaskForTool
	if json.Unmarshal(result, &task) != nil {
		return false
	}
	return stringPatchMatches(input.Title, task.Content) &&
		stringPatchMatches(input.Goal, task.Goal) &&
		stringPatchMatches(input.Status, task.Status) &&
		stringPatchMatches(input.Size, task.Size) &&
		stringPatchMatches(input.Category, task.Business) &&
		stringPatchMatches(input.Type, task.Type) &&
		stringPatchMatches(input.StartDate, task.StartDate) &&
		stringPatchMatches(input.EndDate, task.EndDate) &&
		integerPatchMatches(input.Flag, task.Flag) &&
		stringPatchMatches(input.RequestReason, task.RequestReason) &&
		stringPatchMatches(input.DecisionReason, task.DecisionReason)
}

func stringPatchMatches(expected *string, actual string) bool {
	return expected == nil || *expected == actual
}

func integerPatchMatches(expected *int, actual int) bool {
	return expected == nil || *expected == actual
}

func flowTaskDuplicateID(result json.RawMessage) string {
	var document struct {
		Status        string          `json:"status"`
		DuplicateTask flowTaskForTool `json:"duplicateTask"`
	}
	if json.Unmarshal(result, &document) != nil || document.Status != "skipped_duplicate" {
		return ""
	}
	return strings.TrimSpace(document.DuplicateTask.ID)
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
	ErrorCode    string `json:"errorCode"`
	FailureStage string `json:"failureStage"`
	Message      string `json:"message"`
	Retryable    bool   `json:"retryable"`
	SafeRetry    bool   `json:"safeRetry"`
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
		return flowTaskAddInput{}, fmt.Errorf("task.add input is required")
	}
	var input flowTaskAddInput
	if errorValue := decodeStrictFlowTaskInput(document, &input); errorValue != nil {
		return flowTaskAddInput{}, errorValue
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Goal = strings.TrimSpace(input.Goal)
	input.Size = strings.ToUpper(strings.TrimSpace(input.Size))
	input.Status = strings.TrimSpace(input.Status)
	input.StartDate = strings.TrimSpace(input.StartDate)
	input.EndDate = strings.TrimSpace(input.EndDate)
	input.TargetPersonHint = strings.TrimSpace(input.TargetPersonHint)
	input.ParticipantPersonHints = uniqueTrimmedStringValues(input.ParticipantPersonHints)
	if input.Title == "" {
		return flowTaskAddInput{}, fmt.Errorf("title is required")
	}
	if input.Size != "" && !containsString(flowTaskAddSizes(), input.Size) {
		return flowTaskAddInput{}, fmt.Errorf("size is not allowed")
	}
	if input.Status != "" && !containsString(flowTaskAddStatuses(), input.Status) {
		return flowTaskAddInput{}, fmt.Errorf("status is not allowed")
	}
	return input, nil
}

func flowTaskAddSizes() []string {
	return []string{"XS", "S", "M", "L", "XL", "XXL"}
}

func flowTaskAddStatuses() []string {
	return []string{"예정", "진행", "완료", "일시정지", "기각", "중단"}
}

func flowTaskUpdateStatuses() []string {
	return append(flowTaskAddStatuses(), "요청")
}

func decodeFlowTaskListInput(document json.RawMessage) (flowTaskListInput, error) {
	var input flowTaskListInput
	if len(bytes.TrimSpace(document)) > 0 {
		if errorValue := decodeStrictFlowTaskInput(document, &input); errorValue != nil {
			return flowTaskListInput{}, errorValue
		}
	}
	input.Query = strings.TrimSpace(input.Query)
	input.TargetPersonHint = strings.TrimSpace(input.TargetPersonHint)
	input.Status = strings.TrimSpace(input.Status)
	if input.Scope == "" {
		input.Scope = flowTaskListScopeSelf
	}
	if input.Scope != flowTaskListScopeSelf && input.Scope != flowTaskListScopeAll {
		return flowTaskListInput{}, fmt.Errorf("scope must be self or all")
	}
	if input.Limit < 0 {
		input.Limit = 0
	}
	return input, nil
}

func decodeFlowTaskUpdateInput(document json.RawMessage) (flowTaskUpdateInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return flowTaskUpdateInput{}, fmt.Errorf("task.update input is required")
	}
	var input flowTaskUpdateInput
	if errorValue := decodeStrictFlowTaskInput(document, &input); errorValue != nil {
		return flowTaskUpdateInput{}, errorValue
	}
	input.TaskID = strings.TrimSpace(input.TaskID)
	trimStringPointer(&input.Title)
	trimStringPointer(&input.Goal)
	trimStringPointer(&input.Status)
	trimStringPointer(&input.Size)
	trimStringPointer(&input.Category)
	trimStringPointer(&input.Type)
	trimStringPointer(&input.StartDate)
	trimStringPointer(&input.EndDate)
	trimStringPointer(&input.RequestReason)
	trimStringPointer(&input.DecisionReason)
	if input.Size != nil {
		normalizedSize := strings.ToUpper(*input.Size)
		input.Size = &normalizedSize
	}
	if input.TaskID == "" {
		return flowTaskUpdateInput{}, fmt.Errorf("taskID is required")
	}
	if !hasFlowTaskUpdatePatch(input) {
		return flowTaskUpdateInput{}, fmt.Errorf("task.update requires at least one mutable field")
	}
	if input.Status != nil && !containsString(flowTaskUpdateStatuses(), *input.Status) {
		return flowTaskUpdateInput{}, fmt.Errorf("status is not allowed")
	}
	if input.Size != nil && !containsString(flowTaskAddSizes(), *input.Size) {
		return flowTaskUpdateInput{}, fmt.Errorf("size is not allowed")
	}
	return input, nil
}

func decodeFlowTaskDeleteInput(document json.RawMessage) (flowTaskDeleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return flowTaskDeleteInput{}, fmt.Errorf("task.delete input is required")
	}
	var input flowTaskDeleteInput
	if errorValue := decodeStrictFlowTaskInput(document, &input); errorValue != nil {
		return flowTaskDeleteInput{}, errorValue
	}
	input.TaskID = strings.TrimSpace(input.TaskID)
	if input.TaskID == "" {
		return flowTaskDeleteInput{}, fmt.Errorf("taskID is required")
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

func decodeStrictFlowTaskInput(document json.RawMessage, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(value); errorValue != nil {
		return errorValue
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		return fmt.Errorf("task input contains trailing data")
	}
	return nil
}

func (service Service) fetchFlowMembers(ctx context.Context, requesterEmail string) ([]flowMemberForTool, error) {
	summary, errorValue := service.fetchFlowSummary(ctx, requesterEmail, "")
	if errorValue != nil {
		return nil, errorValue
	}
	return summary.Members, nil
}

func (service Service) fetchFlowSummary(ctx context.Context, requesterEmail string, weekCode string) (flowSummaryForTool, error) {
	if strings.TrimSpace(weekCode) == "" {
		return service.fetchFlowAllTasks(ctx, requesterEmail)
	}
	body, errorValue := service.getFlow(ctx, "/flow/api/summary?week="+url.QueryEscape(strings.TrimSpace(weekCode)), requesterEmail)
	if errorValue != nil {
		return flowSummaryForTool{}, errorValue
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

func (service Service) fetchFlowAllTasks(ctx context.Context, requesterEmail string) (flowSummaryForTool, error) {
	body, errorValue := service.getFlow(ctx, "/flow/api/state", requesterEmail)
	if errorValue != nil {
		return flowSummaryForTool{}, errorValue
	}
	var state struct {
		CurrentWeek flowWeekForTool     `json:"currentWeek"`
		Members     []flowMemberForTool `json:"members"`
		Tasks       []flowTaskForTool   `json:"tasks"`
	}
	if errorValue := json.Unmarshal(body, &state); errorValue != nil {
		return flowSummaryForTool{}, errorValue
	}
	return flowSummaryForTool{Week: state.CurrentWeek, Members: state.Members, Tasks: state.Tasks}, nil
}

func (service Service) getFlow(ctx context.Context, path string, requesterEmail string) ([]byte, error) {
	requestURL := strings.TrimRight(service.Configuration.AdmindBaseURL, "/") + path
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
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
		return nil, fmt.Errorf("flow read failed: %s", strings.TrimSpace(string(body)))
	}
	return body, nil
}

func flowTaskAddErrorResponse(toolName string, failure flowTaskAddFailure) capabilities.ToolInvokeResponse {
	result, _ := json.Marshal(failure)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
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
		Outcome:         capabilities.ToolOutcomeFailed,
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

func (service Service) postFlowTask(ctx context.Context, payload flowTaskCreatePayload, requesterEmail string) (json.RawMessage, error) {
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(service.Configuration.AdmindBaseURL, "/")+"/flow/api/tasks", bytes.NewReader(document))
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
	if httpResponse.StatusCode == http.StatusNotFound {
		return nil, flowTaskDeleteNotFoundError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("flow task delete failed: %s", strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

func flowTaskDeleteEvidenceMatchesTaskID(document json.RawMessage, taskID string) bool {
	var evidence struct {
		Status string `json:"status"`
		Task   struct {
			ID string `json:"id"`
		} `json:"task"`
	}
	if errorValue := json.Unmarshal(document, &evidence); errorValue != nil {
		return false
	}
	return evidence.Status == "deleted" && evidence.Task.ID == taskID
}

type flowTaskFilter struct {
	Query           string
	MemberID        string
	Status          string
	WeekCodes       map[string]bool
	CurrentWeekCode string
	Limit           int
}

func resolveFlowTaskListOwner(input flowTaskListInput, requesterEmail string, members []flowMemberForTool) (string, *flowTaskAddFailure) {
	if input.Scope == flowTaskListScopeAll && strings.TrimSpace(input.TargetPersonHint) == "" {
		return "", nil
	}
	resolution := resolveFlowOwner(flowTaskAddInput{TargetPersonHint: input.TargetPersonHint}, requesterEmail, members)
	if resolution.Failure != nil {
		return "", resolution.Failure
	}
	return resolution.OwnerID, nil
}

func flowTaskListPeopleScope(ownerID string) string {
	if ownerID != "" {
		return "person"
	}
	return "everyone"
}

func flowTaskListWeekCodes(weekFrom int, weekTo int, currentWeekCode string) (map[string]bool, error) {
	if weekFrom > weekTo {
		weekFrom, weekTo = weekTo, weekFrom
	}
	if weekTo-weekFrom > 520 {
		return nil, nil
	}
	currentWeekDate, errorValue := flowWeekDate(currentWeekCode)
	if errorValue != nil {
		return nil, errorValue
	}
	weekCodes := map[string]bool{}
	for offset := weekFrom; offset <= weekTo; offset++ {
		weekCodes[weekCodeForFlowDate(currentWeekDate.AddDate(0, 0, offset*7))] = true
	}
	return weekCodes, nil
}

func flowWeekDate(weekCode string) (time.Time, error) {
	if len(weekCode) != 5 || weekCode[2] != 'W' {
		return time.Time{}, fmt.Errorf("flow current week is invalid")
	}
	year, yearError := strconv.Atoi(weekCode[:2])
	week, weekError := strconv.Atoi(weekCode[3:])
	if yearError != nil || weekError != nil || week < 1 || week > 53 {
		return time.Time{}, fmt.Errorf("flow current week is invalid")
	}
	januaryFourth := time.Date(2000+year, time.January, 4, 0, 0, 0, 0, time.UTC)
	weekdayOffset := (int(januaryFourth.Weekday()) + 6) % 7
	weekDate := januaryFourth.AddDate(0, 0, -weekdayOffset+(week-1)*7)
	if weekCodeForFlowDate(weekDate) != weekCode {
		return time.Time{}, fmt.Errorf("flow current week is invalid")
	}
	return weekDate, nil
}

func weekCodeForFlowDate(date time.Time) string {
	year, week := date.ISOWeek()
	return fmt.Sprintf("%02dW%02d", year%100, week)
}

func normalizeFlowStatusFilter(status string) string {
	trimmedStatus := strings.TrimSpace(status)
	switch strings.ToLower(trimmedStatus) {
	case "":
		return ""
	case "예정", "예약", "planned", "scheduled", "upcoming", "todo", "to do":
		return "예정"
	case "요청", "requested", "request":
		return "요청"
	case "진행", "in_progress", "in progress", "doing", "progress", "wip":
		return "진행"
	case "완료", "done", "complete", "completed", "finished":
		return "완료"
	case "일시정지", "paused", "hold", "on hold":
		return "일시정지"
	case "기각", "rejected", "reject":
		return "기각"
	case "중단", "stopped", "stop", "cancelled", "canceled":
		return "중단"
	default:
		return trimmedStatus
	}
}

func filterFlowTasks(tasks []flowTaskForTool, filter flowTaskFilter) []flowTaskForTool {
	filteredTasks := []flowTaskForTool{}
	for _, task := range tasks {
		if !flowTaskMatchesMember(task, filter.MemberID) {
			continue
		}
		if !flowTaskMatchesWeekCodes(task, filter.WeekCodes, filter.CurrentWeekCode) {
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

func flowTaskMatchesMember(task flowTaskForTool, memberID string) bool {
	if memberID == "" {
		return true
	}
	if task.OwnerID == memberID {
		return true
	}
	return containsString(task.ParticipantIDs, memberID)
}

func flowTaskMatchesWeekCodes(task flowTaskForTool, weekCodes map[string]bool, currentWeekCode string) bool {
	if len(weekCodes) == 0 {
		return true
	}
	for _, weekCode := range flowTaskFilterWeekCodes(task, currentWeekCode) {
		if weekCodes[weekCode] {
			return true
		}
	}
	return false
}

func flowTaskFilterWeekCodes(task flowTaskForTool, currentWeekCode string) []string {
	switch strings.TrimSpace(task.Status) {
	case "예정":
		return plannedFlowTaskWeekCodes(task, currentWeekCode)
	case "일시정지":
		return firstFlowWeekCode(currentWeekCode, task.WeekCode)
	case "완료", "기각", "중단":
		return firstFlowWeekCode(flowTaskDateWeekCode(task.EndDate), flowTaskDateWeekCode(task.StartDate), task.WeekCode)
	default:
		return firstFlowWeekCode(task.WeekCode)
	}
}

func plannedFlowTaskWeekCodes(task flowTaskForTool, currentWeekCode string) []string {
	startWeekCode := flowTaskDateWeekCode(task.StartDate)
	if startWeekCode == "" {
		return firstFlowWeekCode(currentWeekCode, task.WeekCode)
	}
	if flowWeekCodeIsCurrentOrEarlier(startWeekCode, currentWeekCode) {
		return firstFlowWeekCode(currentWeekCode, task.WeekCode)
	}
	return firstFlowWeekCode(startWeekCode, task.WeekCode)
}

func flowWeekCodeIsCurrentOrEarlier(weekCode string, currentWeekCode string) bool {
	year, week, ok := parseFlowWeekCode(weekCode)
	if !ok {
		return false
	}
	currentYear, currentWeek, ok := parseFlowWeekCode(currentWeekCode)
	if !ok {
		return false
	}
	return year < currentYear || year == currentYear && week <= currentWeek
}

func parseFlowWeekCode(weekCode string) (int, int, bool) {
	var year int
	var week int
	_, errorValue := fmt.Sscanf(strings.TrimSpace(strings.ToUpper(weekCode)), "%dW%d", &year, &week)
	if errorValue != nil {
		return 0, 0, false
	}
	return year, week, true
}

func firstFlowWeekCode(values ...string) []string {
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return []string{trimmedValue}
		}
	}
	return nil
}

func flowTaskDateWeekCode(dateText string) string {
	date, errorValue := time.Parse("2006-01-02", strings.TrimSpace(dateText))
	if errorValue != nil {
		return ""
	}
	return weekCodeForFlowDate(date)
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
		Outcome:         capabilities.ToolOutcomeFailed,
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

func resolveFlowTaskUpdateTarget(taskID string, tasks []flowTaskForTool) (flowTaskForTool, *flowTaskUpdateFailure) {
	for _, task := range tasks {
		if task.ID == taskID {
			return task, nil
		}
	}
	failure := flowTaskNotFoundFailure("task.update")
	return flowTaskForTool{}, &failure
}

func flowTaskNotFoundFailure(toolName string) flowTaskUpdateFailure {
	return flowTaskUpdateFailure{
		ErrorCode:    "flow_task_not_found",
		FailureStage: "target_resolution",
		Message:      toolName + " target was not found; use task.list to discover the exact taskID",
		Retryable:    true,
		SafeRetry:    true,
	}
}

func flowTaskDuplicateFailure() flowTaskUpdateFailure {
	return flowTaskUpdateFailure{
		ErrorCode:    "flow_task_duplicate",
		FailureStage: "duplicate_guard",
		Message:      "task.add skipped an existing duplicate; use task.list to inspect it before deciding whether to add another task",
	}
}

func flowTaskInvalidResultFailure(toolName string) flowTaskUpdateFailure {
	return flowTaskUpdateFailure{
		ErrorCode:    "flow_task_result_invalid",
		FailureStage: "result_contract",
		Message:      toolName + " returned a result that does not identify the requested task",
	}
}

func applyFlowTaskUpdateInput(task flowTaskForTool, input flowTaskUpdateInput) flowTaskForTool {
	if input.Title != nil {
		task.Content = *input.Title
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
	return input.Title != nil ||
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
