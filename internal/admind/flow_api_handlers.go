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
	"time"
)

func (service *Service) handleFlow(responseWriter http.ResponseWriter, request *http.Request) {
	request.Header.Del(flowResolvedActorHeader)
	path := strings.TrimPrefix(request.URL.Path, "/flow/api")
	switch {
	case request.Method == http.MethodGet && path == "/summary":
		if !service.authorizeFlowRequest(request, flowActionRead, flowResourceSummary) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.writeFlowSummary(responseWriter, request)
	case request.Method == http.MethodGet && path == "/state":
		if !service.authorizeFlowRequest(request, flowActionRead, flowResourceSummary) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.writeFlowState(responseWriter, request)
	case request.Method == http.MethodGet && path == "/status":
		service.writeFlowStatus(responseWriter)
	case request.Method == http.MethodPut && path == "/definitions":
		if !service.authorizeFlowRequest(request, flowActionManage, flowResourceDefinition) {
			http.Error(responseWriter, "admin access required", http.StatusForbidden)
			return
		}
		service.updateFlowDefinitions(responseWriter, request)
	case request.Method == http.MethodPost && path == "/tasks/quick":
		if !service.authorizeFlowRequest(request, flowActionCreate, flowResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.createQuickFlowTask(responseWriter, request)
	case request.Method == http.MethodPost && path == "/tasks/move":
		if !service.authorizeFlowRequest(request, flowActionUpdate, flowResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.moveFlowTaskOnBoard(responseWriter, request)
	case request.Method == http.MethodPost && path == "/tasks":
		if !service.authorizeFlowRequest(request, flowActionCreate, flowResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.createFlowTask(responseWriter, request)
	case request.Method == http.MethodPut && strings.HasPrefix(path, "/tasks/"):
		if !service.authorizeFlowRequest(request, flowActionUpdate, flowResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.updateFlowTask(responseWriter, request, strings.TrimPrefix(path, "/tasks/"))
	case request.Method == http.MethodDelete && strings.HasPrefix(path, "/tasks/"):
		if !service.authorizeFlowRequest(request, flowActionDelete, flowResourceTask) {
			http.Error(responseWriter, "flow access required", http.StatusForbidden)
			return
		}
		service.deleteFlowTask(responseWriter, request, strings.TrimPrefix(path, "/tasks/"))
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) writeFlowStatus(responseWriter http.ResponseWriter) {
	_, errorValue := os.Stat(service.Configuration.FlowDatabasePath)
	response := flowStatusResponse{
		DatabasePath: service.Configuration.FlowDatabasePath,
		Exists:       errorValue == nil,
		Ready:        true,
		Message:      "Flow task storage is backed by SQLite.",
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) writeFlowSummary(responseWriter http.ResponseWriter, request *http.Request) {
	now := time.Now()
	weekCode, errorValue := parseFlowSummaryWeekCode(request.URL.Query().Get("week"), now)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	weekStart := weekStartForCode(weekCode, now)
	currentWeekCode := weekCodeForDate(now)
	currentWeekStart := weekStartForCode(currentWeekCode, now)
	members := service.flowMembers(request)
	memberFingerprint, errorValue := flowSummaryMemberFingerprint(members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	dependencyKeys := flowSummaryDependencyKeysForWeek(weekCode, weekStart)
	readModel, errorValue := service.readCachedFlowSummaryReadModel(request.Context(), weekCode, dependencyKeys, memberFingerprint, func(ctx context.Context) (flowSummaryReadModel, error) {
		return service.buildFlowSummaryReadModel(ctx, weekCode, weekStart, members)
	})
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	response := flowSummaryResponse{
		Week:        buildFlowWeek(weekCode, weekStart, now),
		CurrentWeek: buildFlowWeek(currentWeekCode, currentWeekStart, now),
		WeeklyTasks: readModel.WeeklyTasks,
		Metrics:     readModel.Metrics,
		Report:      readModel.Report,
		Source:      "sqlite",
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) writeFlowState(responseWriter http.ResponseWriter, request *http.Request) {
	now := time.Now()
	currentWeekCode := weekCodeForDate(now)
	currentWeekStart := weekStartForCode(currentWeekCode, now)
	members := service.flowMembers(request)
	definitions, errorValue := service.readFlowDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	allTasks, errorValue := service.readAllFlowTasks(request.Context(), members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	scoreStart, scoreEnd := flowScoreDateRange(currentWeekStart)
	scoreTasks, errorValue := service.readFlowTasksBetweenDates(request.Context(), scoreStart.Format("2006-01-02"), scoreEnd.Format("2006-01-02"), members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	callerEmail := service.flowActorEmail(request)
	memberScoreDetails := buildFlowMemberScoreDetails(scoreTasks, members, definitions, currentWeekStart)
	memberScores := currentFlowMemberScores(memberScoreDetails)
	distanceMembers := applyFlowMemberScores(calculateFlowMemberDistances(members, allTasks, definitions), memberScores)
	metrics := buildFlowMetrics(nil, definitions)
	metrics.MemberScores = memberScores
	metrics.MemberScoreDetails = memberScoreDetails
	metrics.TotalScore = totalFlowScore(memberScores)
	response := flowStateResponse{
		CurrentWeek:      buildFlowWeek(currentWeekCode, currentWeekStart, now),
		Members:          distanceMembers,
		Tasks:            allTasks,
		Metrics:          metrics,
		Definitions:      definitions,
		StatusOptions:    flowStatusOptions(),
		CurrentUserEmail: callerEmail,
		CurrentUserName:  resolveCurrentUserName(distanceMembers, callerEmail),
		IsAdmin:          service.isFlowAdminEmail(request.Context(), callerEmail),
		Source:           "sqlite",
	}
	service.writeJSON(responseWriter, response)
}

func (service *Service) createFlowTask(responseWriter http.ResponseWriter, request *http.Request) {
	members := service.flowMembers(request)
	definitions, errorValue := service.readFlowDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task, errorValue := service.flowTaskFromRequest(request, members, definitions, "")
	if errorValue != nil {
		writeFlowRequestError(responseWriter, errorValue)
		return
	}
	task.Business = firstNonEmpty(task.Business, defaultFlowTaskBusiness(definitions))
	task, errorValue = service.writeFlowTaskAtStatusEnd(request.Context(), task)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task = service.applyFlowMattermostProjection(request.Context(), task)
	service.writeJSON(responseWriter, task)
}

func defaultFlowTaskBusiness(definitions flowDefinitions) string {
	if len(definitions.Categories) == 0 {
		return ""
	}
	return strings.TrimSpace(definitions.Categories[0])
}

func flowTaskPayloadHasBusiness(document []byte) bool {
	var payload map[string]json.RawMessage
	if json.Unmarshal(document, &payload) != nil {
		return false
	}
	_, hasCategory := payload["category"]
	_, hasBusiness := payload["business"]
	return hasCategory || hasBusiness
}

func (service *Service) updateFlowTask(responseWriter http.ResponseWriter, request *http.Request, taskID string) {
	members := service.flowMembers(request)
	definitions, errorValue := service.readFlowDefinitions(request.Context())
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
	task, errorValue := service.flowTaskFromRequest(request, members, definitions, taskID)
	if errorValue != nil {
		writeFlowRequestError(responseWriter, errorValue)
		return
	}
	existingTask, found, errorValue := service.readFlowTaskByID(request.Context(), task.ID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "task not found", http.StatusNotFound)
		return
	}
	if !service.canUpdateFlowTask(request, existingTask) {
		http.Error(responseWriter, "task owner, participant, or admin access required", http.StatusForbidden)
		return
	}
	if flowTaskAssignmentChanged(existingTask, task) && !service.canManageFlowTaskAssignment(request, existingTask) {
		http.Error(responseWriter, "task owner or admin access required to change assignment", http.StatusForbidden)
		return
	}
	if !flowTaskPayloadHasBusiness(document) {
		task.Business = existingTask.Business
	}
	task.MattermostPostID = existingTask.MattermostPostID
	task.CreatedAt = existingTask.CreatedAt
	if task.Status != existingTask.Status {
		task, errorValue = service.writeFlowTaskAtStatusEnd(request.Context(), task)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		if !task.StatusRankProvided {
			task.StatusRank = existingTask.StatusRank
		}
		if errorValue := service.writeFlowTask(request.Context(), task); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
	}
	task = service.applyFlowMattermostProjection(request.Context(), task)
	service.writeJSON(responseWriter, task)
}

func (service *Service) moveFlowTaskOnBoard(responseWriter http.ResponseWriter, request *http.Request) {
	var payload flowTaskBoardMoveRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		writeFlowRequestError(responseWriter, errorValue)
		return
	}
	payload = cleanFlowTaskBoardMoveRequest(payload)
	if errorValue := validateFlowTaskBoardMoveRequest(payload); errorValue != nil {
		writeFlowTaskBoardMoveError(responseWriter, errorValue)
		return
	}
	task, errorValue := service.writeFlowTaskBoardMove(request.Context(), payload, func(task flowTask) bool {
		return service.canUpdateFlowTask(request, task)
	})
	if errorValue != nil {
		writeFlowTaskBoardMoveError(responseWriter, errorValue)
		return
	}
	task = service.applyFlowMattermostProjection(request.Context(), task)
	service.writeJSON(responseWriter, task)
}

func writeFlowTaskBoardMoveError(responseWriter http.ResponseWriter, errorValue error) {
	switch {
	case errors.Is(errorValue, errFlowTaskBoardMoveInvalidRequest):
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
	case errors.Is(errorValue, errFlowTaskBoardMoveTaskNotFound):
		http.Error(responseWriter, "task not found", http.StatusNotFound)
	case errors.Is(errorValue, errFlowTaskBoardMoveForbidden):
		http.Error(responseWriter, "task owner, participant, or admin access required", http.StatusForbidden)
	default:
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
	}
}

func (service *Service) deleteFlowTask(responseWriter http.ResponseWriter, request *http.Request, taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		http.Error(responseWriter, "task id is required", http.StatusBadRequest)
		return
	}
	task, found, errorValue := service.readFlowTaskByID(request.Context(), taskID)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if !found {
		http.Error(responseWriter, "task not found", http.StatusNotFound)
		return
	}
	if !service.canDeleteFlowTask(request, task) {
		http.Error(responseWriter, "task owner or admin access required", http.StatusForbidden)
		return
	}
	if errorValue := service.deleteFlowTaskByID(request.Context(), taskID); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]any{
		"status": "deleted",
		"task":   task,
	})
}

func (service *Service) updateFlowDefinitions(responseWriter http.ResponseWriter, request *http.Request) {
	var payload flowDefinitionsWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		writeFlowRequestError(responseWriter, errorValue)
		return
	}
	definitions := flowDefinitions{
		Categories: cleanFlowDefinitionValues(payload.Categories),
		Types:      cleanFlowDefinitionValues(payload.Types),
		Sizes:      cleanFlowSizeDefinitions(payload.Sizes),
	}
	if len(definitions.Types) == 0 {
		writeFlowRequestError(responseWriter, flowValidationError("at least one type is required"))
		return
	}
	if len(definitions.Sizes) == 0 {
		writeFlowRequestError(responseWriter, flowValidationError("at least one size definition is required"))
		return
	}
	if errorValue := service.writeFlowDefinitions(request.Context(), definitions); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, definitions)
}

func writeFlowRequestError(responseWriter http.ResponseWriter, errorValue error) {
	http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
}
