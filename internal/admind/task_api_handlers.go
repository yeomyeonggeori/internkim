package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
)

func (service *Service) handleTask(responseWriter http.ResponseWriter, request *http.Request) {
	request.Header.Del(taskResolvedActorHeader)
	request = request.WithContext(withTaskActor(request.Context(), service.taskActorEmail(request)))
	// The board is read from the company, so a write that only reached the queue
	// would be invisible until the queue was next drained. The queue still holds
	// it when the link is down, which is what it is for.

	path := taskAPIPath(request.URL.Path)
	switch {
	case request.Method == http.MethodGet && path == "/summary":
		if !service.authorizeTaskRequest(request, taskActionRead, taskResourceSummary) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.writeTaskSummary(responseWriter, request)
	case request.Method == http.MethodGet && path == "/state":
		if !service.authorizeTaskRequest(request, taskActionRead, taskResourceSummary) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.writeTaskState(responseWriter, request)
	case request.Method == http.MethodGet && path == "/status":
		service.writeTaskStatus(responseWriter)
	case request.Method == http.MethodPut && path == "/definitions":
		if !service.authorizeTaskRequest(request, taskActionManage, taskResourceDefinition) {
			http.Error(responseWriter, "admin access required", http.StatusForbidden)
			return
		}
		service.updateTaskDefinitions(responseWriter, request)
	case request.Method == http.MethodPost && path == "/tasks/quick":
		if !service.authorizeTaskRequest(request, taskActionCreate, taskResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.createQuickTask(responseWriter, request)
	case request.Method == http.MethodPost && path == "/tasks/move":
		if !service.authorizeTaskRequest(request, taskActionUpdate, taskResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.moveTaskOnBoard(responseWriter, request)
	case request.Method == http.MethodPost && path == "/tasks":
		if !service.authorizeTaskRequest(request, taskActionCreate, taskResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.createTask(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/tasks/"):
		if !service.authorizeTaskRequest(request, taskActionUpdate, taskResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.updateTask(responseWriter, request, strings.TrimPrefix(path, "/tasks/"))
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/tasks/"):
		if !service.authorizeTaskRequest(request, taskActionDelete, taskResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.deleteTask(responseWriter, request, strings.TrimPrefix(path, "/tasks/"))
	default:
		http.NotFound(responseWriter, request)
	}
}

func taskAPIPath(requestPath string) string {
	for _, prefix := range []string{"/task/api", "/flow/api"} {
		if strings.HasPrefix(requestPath, prefix) {
			return strings.TrimPrefix(requestPath, prefix)
		}
	}
	return requestPath
}

func (service *Service) writeTaskStatus(responseWriter http.ResponseWriter) {
	_, errorValue := os.Stat(service.stateDatabasePath())
	response := taskStatusResponse{
		DatabasePath: service.stateDatabasePath(),
		Exists:       errorValue == nil,
		Ready:        true,
		Message:      "Flow task storage is backed by SQLite.",
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) writeTaskSummary(responseWriter http.ResponseWriter, request *http.Request) {
	now := taskDateNow()
	weekCode, errorValue := parseTaskSummaryWeekCode(request.URL.Query().Get("week"), now)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	weekStart := weekStartForCode(weekCode, now)
	currentWeekCode := weekCodeForDate(now)
	currentWeekStart := weekStartForCode(currentWeekCode, now)
	members := service.taskMembers(request)
	memberFingerprint, errorValue := taskSummaryMemberFingerprint(members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	dependencyKeys := taskSummaryDependencyKeysForWeek(weekCode, weekStart)
	readModel, errorValue := service.readCachedTaskSummaryReadModel(request.Context(), weekCode, dependencyKeys, memberFingerprint, func(ctx context.Context) (taskSummaryReadModel, error) {
		return service.buildTaskSummaryReadModel(ctx, weekCode, weekStart, members)
	})
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	response := taskSummaryResponse{
		Week:        buildTaskWeek(weekCode, weekStart, now),
		CurrentWeek: buildTaskWeek(currentWeekCode, currentWeekStart, now),
		WeeklyTasks: readModel.WeeklyTasks,
		Metrics:     readModel.Metrics,
		Report:      readModel.Report,
		Source:      "sqlite",
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) writeTaskState(responseWriter http.ResponseWriter, request *http.Request) {
	now := taskDateNow()
	currentWeekCode := weekCodeForDate(now)
	currentWeekStart := weekStartForCode(currentWeekCode, now)
	members := service.taskMembers(request)
	definitions, errorValue := service.readTaskDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	allTasks, errorValue := service.readAllTasks(request.Context(), members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	scoreStart, scoreEnd := taskScoreDateRange(currentWeekStart)
	scoreTasks, errorValue := service.readTasksBetweenDates(request.Context(), scoreStart.Format("2006-01-02"), scoreEnd.Format("2006-01-02"), members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	callerEmail := service.taskActorEmail(request)
	memberScoreDetails := buildTaskMemberScoreDetails(scoreTasks, members, definitions, currentWeekStart)
	memberScores := currentTaskMemberScores(memberScoreDetails)
	distanceMembers := applyTaskMemberScores(calculateTaskMemberDistances(members, allTasks, definitions), memberScores)
	metrics := buildTaskMetrics(nil, definitions)
	metrics.MemberScores = memberScores
	metrics.MemberScoreDetails = memberScoreDetails
	metrics.TotalScore = totalTaskScore(memberScores)
	response := taskStateResponse{
		CurrentWeek:      buildTaskWeek(currentWeekCode, currentWeekStart, now),
		Members:          distanceMembers,
		Tasks:            allTasks,
		Metrics:          metrics,
		Definitions:      definitions,
		StatusOptions:    taskStatusOptions(),
		CurrentUserEmail: callerEmail,
		CurrentUserName:  resolveCurrentUserName(distanceMembers, callerEmail),
		IsAdmin:          service.isTaskAdminEmail(request.Context(), callerEmail),
		Source:           "sqlite",
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) createTask(responseWriter http.ResponseWriter, request *http.Request) {
	members := service.taskMembers(request)
	definitions, errorValue := service.readTaskDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task, payload, errorValue := service.taskAndPayloadFromRequest(request, members, definitions, "")
	if errorValue != nil {
		writeTaskRequestError(responseWriter, errorValue)
		return
	}
	task.Business = firstNonEmpty(task.Business, defaultTaskBusiness(definitions))
	if payload.IsCalendarEvent {
		task.CalendarEventID = service.createPairedCalendarEventForTask(request, task, payload)
	}
	// The company holds the board, so the task is written there and the answer is
	// what the company saved. A device that has no company writes its own store.
	if saved, answered, saveError := service.saveCentralTask(request, task, service.taskPeopleByID(request.Context())); answered {
		if saveError != nil {
			http.Error(responseWriter, saveError.Error(), http.StatusBadGateway)
			return
		}
		service.writeJSON(responseWriter, saved)
		return
	}
	task, errorValue = service.writeTaskAtStatusEnd(request.Context(), task)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task = service.applyTaskMattermostProjection(request.Context(), task)
	service.writeJSON(responseWriter, service.taskAnsweredWithCompanyIdentity(request.Context(), task))
}

func defaultTaskBusiness(definitions taskDefinitions) string {
	if len(definitions.Categories) == 0 {
		return ""
	}
	return strings.TrimSpace(definitions.Categories[0])
}

func taskPayloadHasBusiness(document []byte) bool {
	var payload map[string]json.RawMessage
	if json.Unmarshal(document, &payload) != nil {
		return false
	}
	_, hasCategory := payload["category"]
	_, hasBusiness := payload["business"]
	return hasCategory || hasBusiness
}

func (service *Service) updateTask(responseWriter http.ResponseWriter, request *http.Request, taskID string) {
	members := service.taskMembers(request)
	definitions, errorValue := service.readTaskDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	document, errorValue := io.ReadAll(request.Body)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	request.Body = io.NopCloser(bytes.NewReader(document))
	task, errorValue := service.taskFromRequest(request, members, definitions, taskID)
	if errorValue != nil {
		writeTaskRequestError(responseWriter, errorValue)
		return
	}
	existingTask, found, errorValue := service.readTaskAnswering(request.Context(), task.ID, members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "task not found", http.StatusNotFound)
		return
	}
	// The caller names the task by the company's identifier, and the row that
	// answered to it is filed under the device's own.
	task.ID = existingTask.ID
	if !service.canUpdateTask(request, existingTask) {
		http.Error(responseWriter, "task owner, participant, or admin access required", http.StatusForbidden)
		return
	}
	if taskAssignmentChanged(existingTask, task) && !service.canManageTaskAssignment(request, existingTask) {
		http.Error(responseWriter, "task owner or admin access required to change assignment", http.StatusForbidden)
		return
	}
	if !taskPayloadHasBusiness(document) {
		task.Business = existingTask.Business
	}
	task.MattermostPostID = existingTask.MattermostPostID
	task.CreatedAt = existingTask.CreatedAt
	if saved, answered, saveError := service.saveCentralTask(request, task, service.taskPeopleByID(request.Context())); answered {
		if saveError != nil {
			http.Error(responseWriter, saveError.Error(), http.StatusBadGateway)
			return
		}
		service.writeJSON(responseWriter, saved)
		return
	}
	if task.Status != existingTask.Status {
		task, errorValue = service.writeTaskAtStatusEnd(request.Context(), task)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if !task.StatusRankProvided {
			task.StatusRank = existingTask.StatusRank
		}
		if errorValue := service.writeTask(request.Context(), task); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
	}
	task = service.applyTaskMattermostProjection(request.Context(), task)
	service.writeJSON(responseWriter, service.taskAnsweredWithCompanyIdentity(request.Context(), task))
}

func (service *Service) moveTaskOnBoard(responseWriter http.ResponseWriter, request *http.Request) {
	var payload taskBoardMoveRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		writeTaskRequestError(responseWriter, errorValue)
		return
	}
	payload = cleanTaskBoardMoveRequest(payload)
	if errorValue := validateTaskBoardMoveRequest(payload); errorValue != nil {
		writeTaskBoardMoveError(responseWriter, errorValue)
		return
	}
	task, errorValue := service.writeTaskBoardMove(request.Context(), payload, func(task Task) bool {
		return service.canUpdateTask(request, task)
	})
	if errorValue != nil {
		writeTaskBoardMoveError(responseWriter, errorValue)
		return
	}
	task = service.applyTaskMattermostProjection(request.Context(), task)
	service.writeJSON(responseWriter, service.taskAnsweredWithCompanyIdentity(request.Context(), task))
}

func writeTaskBoardMoveError(responseWriter http.ResponseWriter, errorValue error) {
	switch {
	case errors.Is(errorValue, errTaskBoardMoveInvalidRequest):
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
	case errors.Is(errorValue, errTaskBoardMoveTaskNotFound):
		http.Error(responseWriter, "task not found", http.StatusNotFound)
	case errors.Is(errorValue, errTaskBoardMoveForbidden):
		http.Error(responseWriter, "task owner, participant, or admin access required", http.StatusForbidden)
	default:
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
	}
}

func (service *Service) deleteTask(responseWriter http.ResponseWriter, request *http.Request, taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		http.Error(responseWriter, "task id is required", http.StatusBadRequest)
		return
	}
	task, found, errorValue := service.readTaskAnswering(request.Context(), taskID, service.taskMembers(request))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "task not found", http.StatusNotFound)
		return
	}
	if !service.canDeleteTask(request, task) {
		http.Error(responseWriter, "task owner or admin access required", http.StatusForbidden)
		return
	}
	if answered, removeError := service.removeCentralTask(request, taskID); answered {
		if removeError != nil {
			http.Error(responseWriter, removeError.Error(), http.StatusBadGateway)
			return
		}
		responseWriter.WriteHeader(http.StatusNoContent)
		return
	}
	if errorValue := service.deleteTaskByID(request.Context(), task.ID); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.deletePairedCalendarEventForTask(request.Context(), task)
	service.writeJSON(responseWriter, map[string]any{
		"status": "deleted",
		"task":   task,
	})
}

func (service *Service) updateTaskDefinitions(responseWriter http.ResponseWriter, request *http.Request) {
	var payload taskDefinitionsWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		writeTaskRequestError(responseWriter, errorValue)
		return
	}
	definitions := taskDefinitions{
		Categories:     cleanTaskDefinitionValues(payload.Categories),
		CategoryColors: cleanTaskDefinitionColors(payload.CategoryColors),
		Types:          cleanTaskDefinitionValues(payload.Types),
		TypeColors:     cleanTaskDefinitionColors(payload.TypeColors),
		Sizes:          cleanTaskSizeDefinitions(payload.Sizes),
	}
	if len(definitions.Types) == 0 {
		writeTaskRequestError(responseWriter, taskValidationError("at least one type is required"))
		return
	}
	if len(definitions.Sizes) == 0 {
		writeTaskRequestError(responseWriter, taskValidationError("at least one size definition is required"))
		return
	}
	if errorValue := service.writeTaskDefinitions(request.Context(), definitions); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, definitions)
}

func writeTaskRequestError(responseWriter http.ResponseWriter, errorValue error) {
	http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
}
