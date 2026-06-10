package admind

import (
	"bytes"
	"encoding/json"
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
	weekCode := strings.TrimSpace(request.URL.Query().Get("week"))
	if weekCode == "" {
		weekCode = weekCodeForDate(now)
	}
	weekStart := weekStartForCode(weekCode, now)
	currentWeekCode := weekCodeForDate(now)
	currentWeekStart := weekStartForCode(currentWeekCode, now)
	members := service.flowMembers(request)
	definitions, errorValue := service.readFlowDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	weeklyTasks, errorValue := service.readFlowTasks(request.Context(), weekCode, members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	report, errorValue := service.buildFlowReport(request.Context(), weekStart, members, weeklyTasks, definitions)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	metrics := buildFlowMetrics(weeklyTasks, definitions)
	metrics.MemberScores = map[string]int{}
	metrics.MemberScoreDetails = map[string]flowMemberScoreItem{}
	metrics.TotalScore = 0
	response := flowSummaryResponse{
		Week:        buildFlowWeek(weekCode, weekStart, now),
		CurrentWeek: buildFlowWeek(currentWeekCode, currentWeekStart, now),
		WeeklyTasks: weeklyTasks,
		Metrics:     metrics,
		Report:      report,
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
	if errorValue := service.writeFlowTask(request.Context(), task); errorValue != nil {
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
	if found && !flowTaskPayloadHasBusiness(document) {
		task.Business = existingTask.Business
	}
	task.MattermostPostID = existingTask.MattermostPostID
	if errorValue := service.writeFlowTask(request.Context(), task); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task = service.applyFlowMattermostProjection(request.Context(), task)
	service.writeJSON(responseWriter, task)
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
