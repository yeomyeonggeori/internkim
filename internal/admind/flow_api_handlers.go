package admind

import (
	"encoding/json"
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
	members := service.flowMembers(request)
	definitions, errorValue := service.readFlowDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	tasks, errorValue := service.readFlowTasks(request.Context(), weekCode, members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	report, errorValue := service.buildFlowReport(request.Context(), weekStart, members, tasks, definitions)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	scoreStart, scoreEnd := flowScoreDateRange(weekStart)
	scoreTasks, errorValue := service.readFlowTasksBetweenDates(request.Context(), scoreStart.Format("2006-01-02"), scoreEnd.Format("2006-01-02"), members)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	callerEmail := service.flowActorEmail(request)
	memberScores := buildFlowMemberScores(scoreTasks, members, definitions, weekStart)
	distanceMembers := applyFlowMemberScores(calculateFlowMemberDistances(members, tasks, definitions), memberScores)
	metrics := buildFlowMetrics(tasks, definitions)
	metrics.MemberScores = memberScores
	metrics.TotalScore = totalFlowScore(memberScores)
	response := flowSummaryResponse{
		Week:             buildFlowWeek(weekCode, weekStart, now),
		Members:          distanceMembers,
		Tasks:            tasks,
		Metrics:          metrics,
		Definitions:      definitions,
		Report:           report,
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
	if errorValue := service.writeFlowTask(request.Context(), task); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task = service.syncFlowMattermostNotification(request.Context(), task)
	service.writeJSON(responseWriter, task)
}

func (service *Service) updateFlowTask(responseWriter http.ResponseWriter, request *http.Request, taskID string) {
	members := service.flowMembers(request)
	definitions, errorValue := service.readFlowDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task, errorValue := service.flowTaskFromRequest(request, members, definitions, taskID)
	if errorValue != nil {
		writeFlowRequestError(responseWriter, errorValue)
		return
	}
	task.MattermostPostID = service.existingFlowMattermostPostID(request.Context(), task.ID)
	if errorValue := service.writeFlowTask(request.Context(), task); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task = service.syncFlowMattermostNotification(request.Context(), task)
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
