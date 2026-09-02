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
	"slices"
	"strconv"
	"strings"
	"time"

	"gitlab.com/eastriver/internkim/internal/capabilities"
	"gitlab.com/eastriver/internkim/pkg/capabilityprotocol"
)

type taskAddInput struct {
	Title                  string   `json:"title"`
	Size                   string   `json:"size"`
	Status                 string   `json:"status"`
	Business               string   `json:"business"`
	Type                   string   `json:"type"`
	StartsAt               string   `json:"startsAt"`
	EndsAt                 string   `json:"endsAt"`
	ParticipantPersonHints []string `json:"participantPersonHints"`
}

type taskCreatePayload struct {
	OwnerID        string   `json:"ownerID"`
	ParticipantIDs []string `json:"participantIDs"`
	Content        string   `json:"content"`
	Size           string   `json:"size,omitempty"`
	Status         string   `json:"status,omitempty"`
	Business       string   `json:"business,omitempty"`
	Type           string   `json:"type,omitempty"`
	StartDate      string   `json:"startDate,omitempty"`
	EndDate        string   `json:"endDate,omitempty"`
}

type taskListInput struct {
	Query                 string        `json:"query"`
	ParticipantPersonHint string        `json:"participantPersonHint"`
	Scope                 taskListScope `json:"scope"`
	WeekFrom              int           `json:"weekFrom"`
	WeekTo                int           `json:"weekTo"`
	Status                string        `json:"status"`
	Limit                 int           `json:"limit"`
}

type taskListScope string

const (
	taskListScopeSelf taskListScope = "self"
	taskListScopeAll  taskListScope = "all"
)

type taskUpdateInput struct {
	TaskHint               string    `json:"taskHint"`
	Title                  *string   `json:"title"`
	Status                 *string   `json:"status"`
	Size                   *string   `json:"size"`
	Business               *string   `json:"business"`
	Type                   *string   `json:"type"`
	StartsAt               *string   `json:"startsAt"`
	EndsAt                 *string   `json:"endsAt"`
	ParticipantPersonHints *[]string `json:"participantPersonHints"`
}

type taskDeleteInput struct {
	TaskHint string `json:"taskHint"`
}

type taskSummaryForTool struct {
	Week        taskWeekForTool        `json:"week"`
	Members     []taskMemberForTool    `json:"members"`
	Tasks       []taskForTool          `json:"tasks"`
	WeeklyTasks []taskForTool          `json:"weeklyTasks"`
	Definitions taskDefinitionsForTool `json:"definitions"`
}

type taskWeekForTool struct {
	Code string `json:"code"`
}

type taskMemberForTool struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	MattermostUsername string `json:"mattermostUsername"`
}

type taskForTool struct {
	ID                       string                      `json:"id"`
	OwnerID                  string                      `json:"ownerID"`
	OwnerName                string                      `json:"ownerName"`
	ParticipantIDs           []string                    `json:"participantIDs"`
	ParticipantNames         []string                    `json:"participantNames"`
	ParticipantPresentations []personPresentationForTool `json:"participantPresentations,omitempty"`
	Business                 string                      `json:"business"`
	Type                     string                      `json:"type"`
	Content                  string                      `json:"content"`
	Size                     string                      `json:"size"`
	Status                   string                      `json:"status"`
	StartDate                string                      `json:"startDate"`
	EndDate                  string                      `json:"endDate"`
	WeekCode                 string                      `json:"weekCode"`
	CreatedAt                string                      `json:"createdAt,omitempty"`
}

var taskDeleteNotFoundError = errors.New("flow task delete target not found")

func (service Service) invokeTaskTool(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	switch strings.TrimSpace(request.ToolName) {
	case "task_add":
		return service.invokeTaskAdd(ctx, request)
	case "task_list":
		return service.invokeTaskList(ctx, request)
	case "task_update":
		return service.invokeTaskUpdate(ctx, request)
	case "task_delete":
		return service.invokeTaskDelete(ctx, request)
	case "person_list":
		return service.invokePersonList(ctx, request)
	default:
		return capabilities.ToolInvokeResponse{}, fmt.Errorf("flow task tool is not configured: %s", request.ToolName)
	}
}

func (service Service) invokeTaskAdd(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeTaskAddInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchTaskAllTasks(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	members := summary.Members
	input, labelFailure := resolveTaskAddLabels(input, summary.Definitions)
	if labelFailure != nil {
		return taskFailureResponse(request.ToolName, *labelFailure), nil
	}
	ownerResolution := service.resolveTaskOwner(ctx, input.ParticipantPersonHints, request.Context.RequesterEmail, members)
	if ownerResolution.Failure != nil {
		return taskFailureResponse(request.ToolName, *ownerResolution.Failure), nil
	}
	if duplicateTask, isDuplicate := findRecentDuplicateTask(summary.Tasks, ownerResolution.OwnerID, input.Title, time.Now()); isDuplicate {
		logTaskAddDeduplicated(duplicateTask.ID, ownerResolution.OwnerID)
		if mergedTask, hasNewValues := mergeTaskAddInputIntoDuplicate(duplicateTask, input); hasNewValues {
			result, errorValue := service.putTask(ctx, mergedTask, request.Context.RequesterEmail)
			if errorValue != nil {
				return capabilities.ToolInvokeResponse{}, errorValue
			}
			result = enrichTaskResultDocument(result, members)
			return capabilitySuccessResponse(request.ToolName, taskResponseStatus(result), result)
		}
		result, errorValue := json.Marshal(taskResultDocument(duplicateTask, members))
		if errorValue != nil {
			return capabilities.ToolInvokeResponse{}, errorValue
		}
		return capabilitySuccessResponse(request.ToolName, taskResponseStatus(result), result)
	}
	participantIDs, participantFailure := service.resolveTaskParticipantIDs(ctx, input.ParticipantPersonHints, ownerResolution.OwnerID, members)
	if participantFailure != nil {
		return taskFailureResponse(request.ToolName, *participantFailure), nil
	}
	payload := taskCreatePayload{
		OwnerID:        ownerResolution.OwnerID,
		ParticipantIDs: participantIDs,
		Content:        input.Title,
		Size:           input.Size,
		Status:         input.Status,
		Business:       input.Business,
		Type:           input.Type,
		StartDate:      input.StartsAt,
		EndDate:        input.EndsAt,
	}
	result, errorValue := service.postTask(ctx, payload, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if taskDuplicateID(result) != "" {
		return taskFailureResponse(request.ToolName, taskDuplicateFailure()), nil
	}
	result = enrichTaskResultDocument(result, members)
	taskID := taskResultID(result)
	if taskID == "" {
		return taskFailureResponse(request.ToolName, taskInvalidResultFailure(request.ToolName)), nil
	}
	return capabilitySuccessResponse(request.ToolName, taskResponseStatus(result), result)
}

func (service Service) invokeTaskUpdate(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeTaskUpdateInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchTaskAllTasks(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if len(summary.Members) == 0 {
		if members, membersError := service.fetchTaskMembers(ctx, request.Context.RequesterEmail); membersError == nil {
			summary.Members = members
		}
	}
	task, failure := resolveTaskHint(input.TaskHint, service.requesterTaskOwnerID(ctx, request.Context.RequesterEmail, summary.Members), summary.Tasks)
	if failure != nil {
		return taskFailureResponse(request.ToolName, *failure), nil
	}
	input, labelFailure := resolveTaskLabels(input, summary.Definitions)
	if labelFailure != nil {
		return taskFailureResponse(request.ToolName, *labelFailure), nil
	}
	participantIDs, participantFailure := service.resolveTaskUpdateParticipants(ctx, input, task, summary.Members)
	if participantFailure != nil {
		return taskFailureResponse(request.ToolName, *participantFailure), nil
	}
	updatedTask := applyTaskUpdateInput(task, input, participantIDs)
	result, errorValue := service.putTask(ctx, updatedTask, request.Context.RequesterEmail)
	if errorValue != nil {
		if failure, isRefused := taskWriteRefusal(errorValue, task, participantIDs, summary.Members); isRefused {
			return taskFailureResponse(request.ToolName, failure), nil
		}
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	result = enrichTaskResultDocument(result, summary.Members)
	taskID := taskResultID(result)
	if taskID != task.ID || !taskUpdateResultMatchesInput(result, input, participantIDs) {
		return taskFailureResponse(request.ToolName, taskInvalidResultFailure(request.ToolName)), nil
	}
	return capabilitySuccessResponse(request.ToolName, taskResponseStatus(result), result)
}

func (service Service) invokeTaskList(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeTaskListInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchTaskAllTasks(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if len(summary.Members) == 0 {
		if members, membersError := service.fetchTaskMembers(ctx, request.Context.RequesterEmail); membersError == nil {
			summary.Members = members
		}
	}
	ownerID, failure := service.resolveTaskListOwner(ctx, input, request.Context.RequesterEmail, summary.Members)
	if failure != nil {
		return taskErrorResponse(request.ToolName, *failure), nil
	}
	statusFilter := strings.TrimSpace(input.Status)
	weekCodes, errorValue := taskListWeekCodes(input.WeekFrom, input.WeekTo, summary.Week.Code)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	filteredTasks := filterTasks(summary.Tasks, taskFilter{Query: input.Query, MemberID: ownerID, Status: statusFilter, WeekCodes: weekCodes, CurrentWeekCode: summary.Week.Code, Limit: input.Limit})
	tasks := enrichTasksForTool(filteredTasks, summary.Members)
	result, _ := json.Marshal(map[string]any{
		"scope":            taskListPeopleScope(ownerID),
		"weekFrom":         input.WeekFrom,
		"weekTo":           input.WeekTo,
		"statusFilter":     statusFilter,
		"ownerID":          ownerID,
		"tasks":            tasks,
		"count":            len(tasks),
		"registeredLabels": registeredTaskLabels(summary.Definitions),
	})
	return capabilitySuccessResponse(request.ToolName, "ok", result)
}

func (service Service) invokeTaskDelete(ctx context.Context, request capabilities.ToolInvokeRequest) (capabilities.ToolInvokeResponse, error) {
	input, errorValue := decodeTaskDeleteInput(request.Input)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	summary, errorValue := service.fetchTaskAllTasks(ctx, request.Context.RequesterEmail)
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	task, failure := resolveTaskHint(input.TaskHint, service.requesterTaskOwnerID(ctx, request.Context.RequesterEmail, summary.Members), summary.Tasks)
	if failure != nil {
		return taskFailureResponse(request.ToolName, *failure), nil
	}
	result, errorValue := service.deleteTask(ctx, task.ID, request.Context.RequesterEmail)
	if errors.Is(errorValue, taskDeleteNotFoundError) {
		return taskFailureResponse(request.ToolName, taskNotFoundFailure(request.ToolName)), nil
	}
	if errorValue != nil {
		return capabilities.ToolInvokeResponse{}, errorValue
	}
	if !taskDeleteEvidenceMatchesTaskID(result, task.ID) {
		return taskFailureResponse(request.ToolName, taskNotFoundFailure(request.ToolName)), nil
	}
	result, _ = json.Marshal(map[string]any{"taskID": task.ID, "deleted": true})
	return capabilitySuccessResponse(request.ToolName, "deleted", result)
}

func taskResultID(result json.RawMessage) string {
	var document struct {
		TaskID string `json:"taskID"`
	}
	json.Unmarshal(result, &document)
	return strings.TrimSpace(document.TaskID)
}

func taskUpdateResultMatchesInput(result json.RawMessage, input taskUpdateInput, participantIDs *[]string) bool {
	var task taskForTool
	if json.Unmarshal(result, &task) != nil {
		return false
	}
	return identifierSetPatchMatches(participantIDs, task.ParticipantIDs) &&
		stringPatchMatches(input.Title, task.Content) &&
		stringPatchMatches(input.Status, task.Status) &&
		stringPatchMatches(input.Size, task.Size) &&
		stringPatchMatches(input.Business, task.Business) &&
		stringPatchMatches(input.Type, task.Type) &&
		stringPatchMatches(input.StartsAt, task.StartDate) &&
		stringPatchMatches(input.EndsAt, task.EndDate)
}

func stringPatchMatches(expected *string, actual string) bool {
	return expected == nil || *expected == actual
}

func identifierSetPatchMatches(expected *[]string, actual []string) bool {
	if expected == nil {
		return true
	}
	actualIdentifiers := map[string]bool{}
	for _, identifier := range actual {
		actualIdentifiers[strings.TrimSpace(identifier)] = true
	}
	if len(actualIdentifiers) != len(*expected) {
		return false
	}
	for _, identifier := range *expected {
		if !actualIdentifiers[identifier] {
			return false
		}
	}
	return true
}

func taskDuplicateID(result json.RawMessage) string {
	var document struct {
		Status        string      `json:"status"`
		DuplicateTask taskForTool `json:"duplicateTask"`
	}
	if json.Unmarshal(result, &document) != nil || document.Status != "skipped_duplicate" {
		return ""
	}
	return strings.TrimSpace(document.DuplicateTask.ID)
}

type taskAddFailure struct {
	ErrorCode     string                      `json:"errorCode"`
	FailureStage  string                      `json:"failureStage"`
	Message       string                      `json:"message"`
	Candidates    []taskAddCandidate          `json:"candidates,omitempty"`
	RecoveryHints []capabilities.RecoveryHint `json:"recoveryHints,omitempty"`
	Retryable     bool                        `json:"retryable"`
	SafeRetry     bool                        `json:"safeRetry"`
}

type taskAddCandidate struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	MattermostUsername string `json:"mattermostUsername,omitempty"`
	Mention            string `json:"mention,omitempty"`
}

type taskUpdateFailure struct {
	ErrorCode    string              `json:"errorCode"`
	FailureStage string              `json:"failureStage"`
	Message      string              `json:"message"`
	Candidates   []taskHintCandidate `json:"candidates,omitempty"`
	Retryable    bool                `json:"retryable"`
	SafeRetry    bool                `json:"safeRetry"`
}

type taskWriteRefusalFailure struct {
	ErrorCode    string `json:"errorCode"`
	FailureStage string `json:"failureStage"`
	Message      string `json:"message"`
}

func (failure taskWriteRefusalFailure) failureEnvelope() taskFailureEnvelope {
	return taskFailureEnvelope{failure.ErrorCode, failure.FailureStage, failure.Message, false, false}
}

type taskLabelFailure struct {
	ErrorCode     string                      `json:"errorCode"`
	FailureStage  string                      `json:"failureStage"`
	Message       string                      `json:"message"`
	Field         string                      `json:"field"`
	Candidates    []string                    `json:"candidates,omitempty"`
	RecoveryHints []capabilities.RecoveryHint `json:"recoveryHints,omitempty"`
	Retryable     bool                        `json:"retryable"`
	SafeRetry     bool                        `json:"safeRetry"`
}

type taskFailureEnvelope struct {
	ErrorCode    string
	FailureStage string
	Message      string
	Retryable    bool
	SafeRetry    bool
}

type taskFailure interface {
	failureEnvelope() taskFailureEnvelope
}

func (failure taskAddFailure) failureEnvelope() taskFailureEnvelope {
	return taskFailureEnvelope{failure.ErrorCode, failure.FailureStage, failure.Message, failure.Retryable, failure.SafeRetry}
}

func (failure taskUpdateFailure) failureEnvelope() taskFailureEnvelope {
	return taskFailureEnvelope{failure.ErrorCode, failure.FailureStage, failure.Message, failure.Retryable, failure.SafeRetry}
}

func (failure taskLabelFailure) failureEnvelope() taskFailureEnvelope {
	return taskFailureEnvelope{failure.ErrorCode, failure.FailureStage, failure.Message, failure.Retryable, failure.SafeRetry}
}

type taskHintCandidate struct {
	TaskID string `json:"taskID"`
	Title  string `json:"title"`
}

func taskResponseStatus(result json.RawMessage) string {
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

func decodeTaskAddInput(document json.RawMessage) (taskAddInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return taskAddInput{}, fmt.Errorf("task_add input is required")
	}
	var input taskAddInput
	if errorValue := decodeStrictTaskInput(document, &input); errorValue != nil {
		return taskAddInput{}, errorValue
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Size = strings.ToUpper(strings.TrimSpace(input.Size))
	input.Status = strings.TrimSpace(input.Status)
	input.Business = strings.TrimSpace(input.Business)
	input.Type = strings.TrimSpace(input.Type)
	input.StartsAt = strings.TrimSpace(input.StartsAt)
	input.EndsAt = strings.TrimSpace(input.EndsAt)
	input.ParticipantPersonHints = uniqueTrimmedStringValues(input.ParticipantPersonHints)
	if input.Title == "" {
		return taskAddInput{}, fmt.Errorf("title is required")
	}
	if input.Size != "" && !containsString(taskAddSizes(), input.Size) {
		return taskAddInput{}, fmt.Errorf("size is not allowed")
	}
	input.Status = strings.TrimSpace(input.Status)
	if input.Status != "" && !containsString(taskAddStatuses(), input.Status) {
		return taskAddInput{}, fmt.Errorf("status is not allowed")
	}
	return input, nil
}

func taskAddSizes() []string {
	return []string{"XS", "S", "M", "L", "XL", "XXL"}
}

var (
	catalogTaskAddStatuses    = mustReadToolStatusEnum("task_add")
	catalogTaskUpdateStatuses = mustReadToolStatusEnum("task_update")
)

func taskAddStatuses() []string {
	return slices.Clone(catalogTaskAddStatuses)
}

func taskUpdateStatuses() []string {
	return slices.Clone(catalogTaskUpdateStatuses)
}

func mustReadToolStatusEnum(toolName string) []string {
	var schema struct {
		Properties struct {
			Status struct {
				Enum []string `json:"enum"`
			} `json:"status"`
		} `json:"properties"`
	}
	descriptor := capabilityprotocol.MustGeneratedToolDescriptors(toolName)[0]
	if errorValue := json.Unmarshal(descriptor.InputSchema, &schema); errorValue != nil {
		panic(fmt.Errorf("the %s input schema cannot be read: %w", toolName, errorValue))
	}
	if len(schema.Properties.Status.Enum) == 0 {
		panic(fmt.Errorf("the %s input schema names no statuses", toolName))
	}
	return schema.Properties.Status.Enum
}

func decodeTaskListInput(document json.RawMessage) (taskListInput, error) {
	var input taskListInput
	if len(bytes.TrimSpace(document)) > 0 {
		if errorValue := decodeStrictTaskInput(document, &input); errorValue != nil {
			return taskListInput{}, errorValue
		}
	}
	input.Query = strings.TrimSpace(input.Query)
	input.ParticipantPersonHint = strings.TrimSpace(input.ParticipantPersonHint)
	input.Status = strings.TrimSpace(input.Status)
	if input.Scope == "" {
		input.Scope = taskListScopeSelf
	}
	if input.Scope != taskListScopeSelf && input.Scope != taskListScopeAll {
		return taskListInput{}, fmt.Errorf("scope must be self or all")
	}
	if input.Limit < 0 {
		input.Limit = 0
	}
	return input, nil
}

func decodeTaskUpdateInput(document json.RawMessage) (taskUpdateInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return taskUpdateInput{}, fmt.Errorf("task_update input is required")
	}
	var input taskUpdateInput
	if errorValue := decodeStrictTaskInput(document, &input); errorValue != nil {
		return taskUpdateInput{}, errorValue
	}
	input.TaskHint = strings.TrimSpace(input.TaskHint)
	trimStringPointer(&input.Title)
	trimStringPointer(&input.Status)
	trimStringPointer(&input.Size)
	trimStringPointer(&input.Business)
	trimStringPointer(&input.Type)
	trimStringPointer(&input.StartsAt)
	trimStringPointer(&input.EndsAt)
	if input.ParticipantPersonHints != nil {
		participantPersonHints := uniqueTrimmedStringValues(*input.ParticipantPersonHints)
		input.ParticipantPersonHints = &participantPersonHints
	}
	if input.Size != nil {
		normalizedSize := strings.ToUpper(*input.Size)
		input.Size = &normalizedSize
	}
	if input.TaskHint == "" {
		return taskUpdateInput{}, fmt.Errorf("taskHint is required")
	}
	if !hasTaskUpdatePatch(input) {
		return taskUpdateInput{}, fmt.Errorf("task_update requires at least one mutable field")
	}
	if input.Status != nil {
		normalizedStatus := strings.TrimSpace(*input.Status)
		input.Status = &normalizedStatus
		if !containsString(taskUpdateStatuses(), normalizedStatus) {
			return taskUpdateInput{}, fmt.Errorf("status is not allowed")
		}
	}
	if input.Size != nil && !containsString(taskAddSizes(), *input.Size) {
		return taskUpdateInput{}, fmt.Errorf("size is not allowed")
	}
	return input, nil
}

func decodeTaskDeleteInput(document json.RawMessage) (taskDeleteInput, error) {
	if len(bytes.TrimSpace(document)) == 0 {
		return taskDeleteInput{}, fmt.Errorf("task_delete input is required")
	}
	var input taskDeleteInput
	if errorValue := decodeStrictTaskInput(document, &input); errorValue != nil {
		return taskDeleteInput{}, errorValue
	}
	input.TaskHint = strings.TrimSpace(input.TaskHint)
	if input.TaskHint == "" {
		return taskDeleteInput{}, fmt.Errorf("taskHint is required")
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

func decodeStrictTaskInput(document json.RawMessage, value any) error {
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

func (service Service) fetchTaskMembers(ctx context.Context, requesterEmail string) ([]taskMemberForTool, error) {
	summary, errorValue := service.fetchTaskSummary(ctx, requesterEmail, "")
	if errorValue != nil {
		return nil, errorValue
	}
	return summary.Members, nil
}

func (service Service) fetchTaskSummary(ctx context.Context, requesterEmail string, weekCode string) (taskSummaryForTool, error) {
	if strings.TrimSpace(weekCode) == "" {
		return service.fetchTaskAllTasks(ctx, requesterEmail)
	}
	body, errorValue := service.getTask(ctx, "/task/api/summary?week="+url.QueryEscape(strings.TrimSpace(weekCode)), requesterEmail)
	if errorValue != nil {
		return taskSummaryForTool{}, errorValue
	}
	var summary taskSummaryForTool
	if errorValue := json.Unmarshal(body, &summary); errorValue != nil {
		return taskSummaryForTool{}, errorValue
	}
	if len(summary.Tasks) == 0 && len(summary.WeeklyTasks) > 0 {
		summary.Tasks = summary.WeeklyTasks
	}
	return summary, nil
}

func (service Service) fetchTaskAllTasks(ctx context.Context, requesterEmail string) (taskSummaryForTool, error) {
	body, errorValue := service.getTask(ctx, "/task/api/state", requesterEmail)
	if errorValue != nil {
		return taskSummaryForTool{}, errorValue
	}
	var state struct {
		CurrentWeek taskWeekForTool        `json:"currentWeek"`
		Members     []taskMemberForTool    `json:"members"`
		Tasks       []taskForTool          `json:"tasks"`
		Definitions taskDefinitionsForTool `json:"definitions"`
	}
	if errorValue := json.Unmarshal(body, &state); errorValue != nil {
		return taskSummaryForTool{}, errorValue
	}
	return taskSummaryForTool{Week: state.CurrentWeek, Members: state.Members, Tasks: state.Tasks, Definitions: state.Definitions}, nil
}

func (service Service) getTask(ctx context.Context, path string, requesterEmail string) ([]byte, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, admindRequesterURL(path), nil)
	if errorValue != nil {
		return nil, errorValue
	}
	httpResponse, errorValue := service.askAdmindAsTheRequester(httpRequest, requesterEmail)
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

func taskFailureResponseErrorCode(errorCode string) string {
	switch errorCode {
	case "task_owner_ambiguous", "task_participant_ambiguous", "task_label_ambiguous":
		return "interaction_required"
	default:
		return errorCode
	}
}

func taskFailureResponse(toolName string, failure taskFailure) capabilities.ToolInvokeResponse {
	envelope := failure.failureEnvelope()
	result, _ := json.Marshal(failure)
	return capabilities.ToolInvokeResponse{
		Provider:        "internkim",
		SelectedBackend: "device",
		ToolName:        toolName,
		Outcome:         capabilities.ToolOutcomeFailed,
		Status:          "error",
		Content:         envelope.Message,
		IsError:         true,
		Message:         envelope.Message,
		ErrorCode:       taskFailureResponseErrorCode(envelope.ErrorCode),
		FailureStage:    envelope.FailureStage,
		Retryable:       envelope.Retryable,
		SafeRetry:       envelope.SafeRetry,
		Result:          result,
	}
}

func (service Service) postTask(ctx context.Context, payload taskCreatePayload, requesterEmail string) (json.RawMessage, error) {
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, admindRequesterURL("/task/api/tasks"), bytes.NewReader(document))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := service.askAdmindAsTheRequester(httpRequest, requesterEmail)
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

func (service Service) putTask(ctx context.Context, task taskForTool, requesterEmail string) (json.RawMessage, error) {
	payload := map[string]any{
		"ownerID":        task.OwnerID,
		"participantIDs": task.ParticipantIDs,
		"category":       task.Business,
		"type":           task.Type,
		"content":        task.Content,
		"size":           task.Size,
		"status":         task.Status,
		"startDate":      task.StartDate,
		"endDate":        task.EndDate,
		"weekCode":       task.WeekCode,
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodPut, admindRequesterURL("/task/api/tasks/"+url.PathEscape(task.ID)), bytes.NewReader(document))
	if errorValue != nil {
		return nil, errorValue
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpResponse, errorValue := service.askAdmindAsTheRequester(httpRequest, requesterEmail)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	body, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return nil, readError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, taskAPIStatusError{StatusCode: httpResponse.StatusCode, Body: strings.TrimSpace(string(body))}
	}
	return json.RawMessage(body), nil
}

type taskAPIStatusError struct {
	StatusCode int
	Body       string
}

func (statusError taskAPIStatusError) Error() string {
	return fmt.Sprintf("flow task update failed: %d %s", statusError.StatusCode, statusError.Body)
}

func (service Service) deleteTask(ctx context.Context, taskID string, requesterEmail string) (json.RawMessage, error) {
	httpRequest, errorValue := http.NewRequestWithContext(ctx, http.MethodDelete, admindRequesterURL("/task/api/tasks/"+url.PathEscape(taskID)), nil)
	if errorValue != nil {
		return nil, errorValue
	}
	httpResponse, errorValue := service.askAdmindAsTheRequester(httpRequest, requesterEmail)
	if errorValue != nil {
		return nil, errorValue
	}
	defer httpResponse.Body.Close()
	body, readError := io.ReadAll(httpResponse.Body)
	if readError != nil {
		return nil, readError
	}
	if httpResponse.StatusCode == http.StatusNotFound {
		return nil, taskDeleteNotFoundError
	}
	if httpResponse.StatusCode >= http.StatusBadRequest {
		return nil, fmt.Errorf("flow task delete failed: %s", strings.TrimSpace(string(body)))
	}
	return json.RawMessage(body), nil
}

func taskDeleteEvidenceMatchesTaskID(document json.RawMessage, taskID string) bool {
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

type taskFilter struct {
	Query           string
	MemberID        string
	Status          string
	WeekCodes       map[string]bool
	CurrentWeekCode string
	Limit           int
}

func (service Service) resolveTaskListOwner(ctx context.Context, input taskListInput, requesterEmail string, members []taskMemberForTool) (string, *taskAddFailure) {
	if input.Scope == taskListScopeAll && strings.TrimSpace(input.ParticipantPersonHint) == "" {
		return "", nil
	}
	resolution := service.resolveTaskOwner(ctx, []string{input.ParticipantPersonHint}, requesterEmail, members)
	if resolution.Failure != nil {
		return "", resolution.Failure
	}
	return resolution.OwnerID, nil
}

func taskListPeopleScope(ownerID string) string {
	if ownerID != "" {
		return "person"
	}
	return "everyone"
}

func taskListWeekCodes(weekFrom int, weekTo int, currentWeekCode string) (map[string]bool, error) {
	if weekFrom > weekTo {
		weekFrom, weekTo = weekTo, weekFrom
	}
	if weekTo-weekFrom > 520 {
		return nil, nil
	}
	currentWeekDate, errorValue := taskWeekDate(currentWeekCode)
	if errorValue != nil {
		return nil, errorValue
	}
	weekCodes := map[string]bool{}
	for offset := weekFrom; offset <= weekTo; offset++ {
		weekCodes[weekCodeForTaskDate(currentWeekDate.AddDate(0, 0, offset*7))] = true
	}
	return weekCodes, nil
}

func taskWeekDate(weekCode string) (time.Time, error) {
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
	if weekCodeForTaskDate(weekDate) != weekCode {
		return time.Time{}, fmt.Errorf("flow current week is invalid")
	}
	return weekDate, nil
}

func weekCodeForTaskDate(date time.Time) string {
	year, week := date.ISOWeek()
	return fmt.Sprintf("%02dW%02d", year%100, week)
}

func filterTasks(tasks []taskForTool, filter taskFilter) []taskForTool {
	filteredTasks := []taskForTool{}
	for _, task := range tasks {
		if !taskMatchesMember(task, filter.MemberID) {
			continue
		}
		if !taskMatchesWeekCodes(task, filter.WeekCodes, filter.CurrentWeekCode) {
			continue
		}
		if filter.Status != "" && task.Status != filter.Status {
			continue
		}
		if filter.Query != "" && !taskMatchesQuery(task, filter.Query) {
			continue
		}
		filteredTasks = append(filteredTasks, task)
		if filter.Limit > 0 && len(filteredTasks) >= filter.Limit {
			break
		}
	}
	return filteredTasks
}

func taskMatchesMember(task taskForTool, memberID string) bool {
	if memberID == "" {
		return true
	}
	if task.OwnerID == memberID {
		return true
	}
	return containsString(task.ParticipantIDs, memberID)
}

func taskMatchesWeekCodes(task taskForTool, weekCodes map[string]bool, currentWeekCode string) bool {
	if len(weekCodes) == 0 {
		return true
	}
	for _, weekCode := range taskFilterWeekCodes(task, currentWeekCode) {
		if weekCodes[weekCode] {
			return true
		}
	}
	return false
}

func taskFilterWeekCodes(task taskForTool, currentWeekCode string) []string {
	switch strings.TrimSpace(task.Status) {
	case "planned":
		return plannedTaskWeekCodes(task, currentWeekCode)
	case "paused":
		return firstTaskWeekCode(currentWeekCode, task.WeekCode)
	case "completed", "rejected", "stopped":
		return firstTaskWeekCode(taskDateWeekCode(task.EndDate), taskDateWeekCode(task.StartDate), task.WeekCode)
	default:
		return firstTaskWeekCode(task.WeekCode)
	}
}

func plannedTaskWeekCodes(task taskForTool, currentWeekCode string) []string {
	startWeekCode := taskDateWeekCode(task.StartDate)
	if startWeekCode == "" {
		return firstTaskWeekCode(currentWeekCode, task.WeekCode)
	}
	if taskWeekCodeIsCurrentOrEarlier(startWeekCode, currentWeekCode) {
		return firstTaskWeekCode(currentWeekCode, task.WeekCode)
	}
	return firstTaskWeekCode(startWeekCode, task.WeekCode)
}

func taskWeekCodeIsCurrentOrEarlier(weekCode string, currentWeekCode string) bool {
	year, week, ok := parseTaskWeekCode(weekCode)
	if !ok {
		return false
	}
	currentYear, currentWeek, ok := parseTaskWeekCode(currentWeekCode)
	if !ok {
		return false
	}
	return year < currentYear || year == currentYear && week <= currentWeek
}

func parseTaskWeekCode(weekCode string) (int, int, bool) {
	var year int
	var week int
	_, errorValue := fmt.Sscanf(strings.TrimSpace(strings.ToUpper(weekCode)), "%dW%d", &year, &week)
	if errorValue != nil {
		return 0, 0, false
	}
	return year, week, true
}

func firstTaskWeekCode(values ...string) []string {
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue != "" {
			return []string{trimmedValue}
		}
	}
	return nil
}

func taskDateWeekCode(dateText string) string {
	date, errorValue := time.Parse("2006-01-02", strings.TrimSpace(dateText))
	if errorValue != nil {
		return ""
	}
	return weekCodeForTaskDate(date)
}

func taskMatchesQuery(task taskForTool, query string) bool {
	normalizedQuery := normalizeTaskSearchText(query)
	if normalizedQuery == "" {
		return true
	}
	values := []string{task.Content, task.Business, task.Type, task.OwnerName, task.Status}
	for _, value := range values {
		if strings.Contains(normalizeTaskSearchText(value), normalizedQuery) {
			return true
		}
	}
	return false
}

func normalizeTaskSearchText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), "")
}

func taskErrorResponse(toolName string, failure taskAddFailure) capabilities.ToolInvokeResponse {
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
		ErrorCode:       taskFailureResponseErrorCode(failure.ErrorCode),
		FailureStage:    failure.FailureStage,
		Retryable:       failure.Retryable,
		SafeRetry:       failure.SafeRetry,
	}
}

func (task taskForTool) hintIdentifiers() []string { return []string{task.ID} }

func (task taskForTool) hintTitle() string { return task.Content }

func (task taskForTool) hintNearness(hint string) float64 {
	return titleNearness(hint, task.Content)
}

func resolveTaskHint(taskHint string, requesterOwnerID string, tasks []taskForTool) (taskForTool, *taskUpdateFailure) {
	resolution := resolveHint(taskHint, tasks, taskOwnership(requesterOwnerID))
	if resolution.Outcome == hintResolved {
		return resolution.Match, nil
	}
	failure := taskHintUnresolvedFailure(resolution)
	return taskForTool{}, &failure
}

func taskOwnership(requesterOwnerID string) func(taskForTool) bool {
	trimmedRequesterOwnerID := strings.TrimSpace(requesterOwnerID)
	if trimmedRequesterOwnerID == "" {
		return nil
	}
	return func(task taskForTool) bool { return task.OwnerID == trimmedRequesterOwnerID }
}

func taskHintUnresolvedFailure(resolution hintResolution[taskForTool]) taskUpdateFailure {
	return taskUpdateFailure{
		ErrorCode:    "task_hint_unresolved",
		FailureStage: "target_resolution",
		Message:      unresolvedHintMessage("task", "taskHint", "task_list", resolution.Outcome),
		Candidates:   taskHintCandidates(resolution.Candidates),
		Retryable:    true,
		SafeRetry:    true,
	}
}

func taskHintCandidates(tasks []taskForTool) []taskHintCandidate {
	candidates := make([]taskHintCandidate, 0, len(tasks))
	for _, task := range tasks {
		candidates = append(candidates, taskHintCandidate{TaskID: task.ID, Title: task.Content})
	}
	return candidates
}

func taskNotFoundFailure(toolName string) taskUpdateFailure {
	return taskUpdateFailure{
		ErrorCode:    "task_not_found",
		FailureStage: "target_resolution",
		Message:      toolName + " target was not found; it may have been deleted since task_list was called",
		Retryable:    true,
		SafeRetry:    true,
	}
}

func taskDuplicateFailure() taskUpdateFailure {
	return taskUpdateFailure{
		ErrorCode:    "task_duplicate",
		FailureStage: "duplicate_guard",
		Message:      "task_add skipped an existing duplicate; use task_list to inspect it before deciding whether to add another task",
	}
}

func taskInvalidResultFailure(toolName string) taskUpdateFailure {
	return taskUpdateFailure{
		ErrorCode:    "task_result_invalid",
		FailureStage: "result_contract",
		Message:      toolName + " returned a result that does not identify the requested task",
	}
}

func applyTaskUpdateInput(task taskForTool, input taskUpdateInput, participantIDs *[]string) taskForTool {
	if participantIDs != nil {
		task.ParticipantIDs = *participantIDs
	}
	if input.Title != nil {
		task.Content = *input.Title
	}
	if input.Status != nil {
		task.Status = *input.Status
	}
	if input.Size != nil {
		task.Size = *input.Size
	}
	if input.Business != nil {
		task.Business = *input.Business
	}
	if input.Type != nil {
		task.Type = *input.Type
	}
	if input.StartsAt != nil {
		task.StartDate = *input.StartsAt
	}
	if input.EndsAt != nil {
		task.EndDate = *input.EndsAt
	}
	return task
}

func hasTaskUpdatePatch(input taskUpdateInput) bool {
	return input.Title != nil ||
		input.Status != nil ||
		input.Size != nil ||
		input.Business != nil ||
		input.Type != nil ||
		input.StartsAt != nil ||
		input.EndsAt != nil ||
		input.ParticipantPersonHints != nil
}
