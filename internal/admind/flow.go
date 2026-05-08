package admind

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"

	_ "modernc.org/sqlite"
)

type flowSummaryResponse struct {
	Week             flowWeek        `json:"week"`
	Members          []flowMember    `json:"members"`
	Tasks            []flowTask      `json:"tasks"`
	Metrics          flowMetrics     `json:"metrics"`
	Definitions      flowDefinitions `json:"definitions"`
	StatusOptions    []string        `json:"statusOptions"`
	CurrentUserEmail string          `json:"currentUserEmail"`
	CurrentUserName  string          `json:"currentUserName"`
	IsAdmin          bool            `json:"isAdmin"`
	Source           string          `json:"source"`
}

type flowStatusResponse struct {
	DatabasePath string `json:"databasePath"`
	Exists       bool   `json:"exists"`
	Ready        bool   `json:"ready"`
	Message      string `json:"message"`
}

type flowWeek struct {
	Code      string `json:"code"`
	StartISO  string `json:"startISO"`
	EndISO    string `json:"endISO"`
	Previous  string `json:"previous"`
	Next      string `json:"next"`
	IsCurrent bool   `json:"isCurrent"`
}

type flowMember struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Email             string `json:"email"`
	Role              string `json:"role"`
	MattermostStatus  string `json:"mattermostStatus"`
	Score             int    `json:"score"`
	ActiveTaskCount   int    `json:"activeTaskCount"`
	CompleteTaskCount int    `json:"completeTaskCount"`
}

type flowTask struct {
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
	StartDate        string   `json:"startDate,omitempty"`
	EndDate          string   `json:"endDate,omitempty"`
	WeekCode         string   `json:"weekCode"`
	Flag             int      `json:"flag"`
	RequestReason    string   `json:"requestReason,omitempty"`
	DecisionReason   string   `json:"decisionReason,omitempty"`
	MattermostPostID string   `json:"mattermostPostID,omitempty"`
}

type flowMetrics struct {
	TotalTasks     int            `json:"totalTasks"`
	CompletedTasks int            `json:"completedTasks"`
	RequestedTasks int            `json:"requestedTasks"`
	PausedTasks    int            `json:"pausedTasks"`
	StoppedTasks   int            `json:"stoppedTasks"`
	TotalScore     int            `json:"totalScore"`
	StatusCounts   map[string]int `json:"statusCounts"`
	BusinessCounts map[string]int `json:"businessCounts"`
	TypeCounts     map[string]int `json:"typeCounts"`
	MemberScores   map[string]int `json:"memberScores"`
}

type flowDefinitions struct {
	Categories []string             `json:"categories"`
	Types      []string             `json:"types"`
	Sizes      []flowSizeDefinition `json:"sizes"`
}

type flowSizeDefinition struct {
	Name               string `json:"name"`
	DistanceKM         int    `json:"distanceKm"`
	MaxHours           int    `json:"maxHours"`
	DevelopmentExample string `json:"developmentExample"`
	OtherExample       string `json:"otherExample"`
	Note               string `json:"note"`
	Score              int    `json:"score"`
	Label              string `json:"label"`
}

type flowTaskWriteRequest struct {
	OwnerID        string   `json:"ownerID"`
	ParticipantIDs []string `json:"participantIDs"`
	Business       string   `json:"business"`
	Category       string   `json:"category"`
	Type           string   `json:"type"`
	Content        string   `json:"content"`
	Goal           string   `json:"goal"`
	Size           string   `json:"size"`
	Status         string   `json:"status"`
	StartDate      string   `json:"startDate"`
	EndDate        string   `json:"endDate"`
	WeekCode       string   `json:"weekCode"`
	Flag           int      `json:"flag"`
	RequestReason  string   `json:"requestReason"`
	DecisionReason string   `json:"decisionReason"`
}

type flowQuickTaskRequest struct {
	Prompt         string   `json:"prompt"`
	OwnerID        string   `json:"ownerID"`
	ParticipantIDs []string `json:"participantIDs"`
	WeekCode       string   `json:"weekCode"`
	RequesterEmail string   `json:"requesterEmail"`
	Source         string   `json:"source"`
	AllowDuplicate bool     `json:"allowDuplicate"`
}

type flowDefinitionsWriteRequest struct {
	Categories []string             `json:"categories"`
	Types      []string             `json:"types"`
	Sizes      []flowSizeDefinition `json:"sizes"`
}

type inferredFlowTask struct {
	Category       string   `json:"category"`
	Type           string   `json:"type"`
	Content        string   `json:"content"`
	Goal           string   `json:"goal"`
	Size           string   `json:"size"`
	Status         string   `json:"status"`
	StartDate      string   `json:"startDate"`
	EndDate        string   `json:"endDate"`
	ParticipantIDs []string `json:"participantIDs"`
	RequestReason  string   `json:"requestReason"`
}

type flowDuplicateDecision struct {
	IsDuplicate     bool   `json:"isDuplicate"`
	DuplicateTaskID string `json:"duplicateTaskID"`
	Reason          string `json:"reason"`
}

type capabilityLLMResponse struct {
	Content string `json:"content"`
}

type flowValidationError string

func (errorValue flowValidationError) Error() string {
	return string(errorValue)
}

const (
	flowActionRead   = "read"
	flowActionCreate = "create"
	flowActionUpdate = "update"
	flowActionManage = "manage"

	flowResourceSummary    = "api:flow.summary"
	flowResourceTask       = "api:flow.task"
	flowResourceDefinition = "api:flow.definition"

	flowRequesterEmailHeader = "X-InternKim-Requester-Email"
)

func (service *Service) serveFlowPage(responseWriter http.ResponseWriter, request *http.Request) {
	if request.URL.Path == "/flow" {
		http.Redirect(responseWriter, request, "/flow/", http.StatusFound)
		return
	}
	if service.serveFlowStaticFile(responseWriter, request) {
		return
	}
	service.serveFlowIndex(responseWriter, request)
}

func (service *Service) serveFlowStaticFile(responseWriter http.ResponseWriter, request *http.Request) bool {
	relativePath := strings.TrimPrefix(request.URL.Path, "/flow/")
	if relativePath == "" {
		return false
	}
	filePath := filepath.Join(service.Configuration.AdminUIPath, "flow", relativePath)
	fileInfo, errorValue := os.Stat(filePath)
	if errorValue != nil || fileInfo.IsDir() {
		return false
	}
	http.ServeFile(responseWriter, request, filePath)
	return true
}

func (service *Service) serveFlowIndex(responseWriter http.ResponseWriter, request *http.Request) {
	flowIndexPath := filepath.Join(service.Configuration.AdminUIPath, "flow", "index.html")
	if fileInfo, errorValue := os.Stat(flowIndexPath); errorValue == nil && !fileInfo.IsDir() {
		http.ServeFile(responseWriter, request, flowIndexPath)
		return
	}
	http.ServeFile(responseWriter, request, filepath.Join(service.Configuration.AdminUIPath, "index.html"))
}

func (service *Service) handleFlow(responseWriter http.ResponseWriter, request *http.Request) {
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

func (service *Service) authorizeFlowRequest(request *http.Request, action string, resource string) bool {
	actorEmail := service.flowActorEmail(request)
	if actorEmail == "" {
		return false
	}
	if action == flowActionManage && resource == flowResourceDefinition {
		return service.isFlowAdminActor(request, actorEmail)
	}
	if resource == flowResourceSummary && action == flowActionRead {
		return service.isFlowStaffActor(request.Context(), actorEmail)
	}
	if resource == flowResourceTask && (action == flowActionCreate || action == flowActionUpdate) {
		return service.isFlowStaffActor(request.Context(), actorEmail)
	}
	return false
}

func (service *Service) flowActorEmail(request *http.Request) string {
	if callerEmail := authenticatedCallerEmail(request); callerEmail != "" {
		return callerEmail
	}
	if !isLocalRequest(request) {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(request.Header.Get(flowRequesterEmailHeader)))
}

func (service *Service) isFlowStaffActor(ctx context.Context, actorEmail string) bool {
	if service.isFlowAdminEmail(ctx, actorEmail) {
		return true
	}
	if strings.TrimSpace(actorEmail) == "" {
		return false
	}
	if !service.hasDeviceAuth() {
		return true
	}
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	records, errorValue := service.lookupUserRecords(ctx, fleetID, fleetSecret)
	if errorValue != nil {
		return false
	}
	for _, record := range records {
		if strings.EqualFold(record.Email, actorEmail) && isActiveFlowUser(record) {
			return true
		}
	}
	return false
}

func (service *Service) isFlowAdminActor(request *http.Request, actorEmail string) bool {
	return service.isFlowAdminEmail(request.Context(), actorEmail)
}

func (service *Service) isFlowAdminEmail(ctx context.Context, actorEmail string) bool {
	if strings.TrimSpace(actorEmail) == "" {
		return false
	}
	if service.hasDeviceAuth() {
		return service.isCurrentAdminEmail(ctx, actorEmail) || service.isClaimedAdminEmail(actorEmail)
	}
	adminEmail := service.seedAdminEmail()
	return adminEmail != "" && strings.EqualFold(actorEmail, adminEmail)
}

func isActiveFlowUser(record adminUserMutation) bool {
	status := strings.ToLower(strings.TrimSpace(record.Status))
	return status == "" || status == "active"
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
	callerEmail := service.flowActorEmail(request)
	scoredMembers := scoreFlowMembers(members, tasks, definitions)
	response := flowSummaryResponse{
		Week:             buildFlowWeek(weekCode, weekStart, now),
		Members:          scoredMembers,
		Tasks:            tasks,
		Metrics:          buildFlowMetrics(tasks, definitions),
		Definitions:      definitions,
		StatusOptions:    flowStatusOptions(),
		CurrentUserEmail: callerEmail,
		CurrentUserName:  resolveCurrentUserName(scoredMembers, callerEmail),
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

func (service *Service) createQuickFlowTask(responseWriter http.ResponseWriter, request *http.Request) {
	members := service.flowMembers(request)
	definitions, errorValue := service.readFlowDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	var payload flowQuickTaskRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		writeFlowRequestError(responseWriter, errorValue)
		return
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if prompt == "" {
		writeFlowRequestError(responseWriter, flowValidationError("prompt is required"))
		return
	}
	requesterEmail := service.flowRequesterEmail(request, payload)
	owner, errorValue := flowOwnerFromQuickRequest(payload, members, requesterEmail)
	if errorValue != nil {
		writeFlowRequestError(responseWriter, errorValue)
		return
	}
	inferredTask, errorValue := service.inferFlowTask(request.Context(), prompt, payload.WeekCode, owner, members, definitions)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	writeRequest := flowTaskWriteRequest{
		OwnerID:        owner.ID,
		ParticipantIDs: firstNonEmptySlice(inferredTask.ParticipantIDs, payload.ParticipantIDs, []string{owner.ID}),
		Category:       inferredTask.Category,
		Type:           inferredTask.Type,
		Content:        inferredTask.Content,
		Goal:           inferredTask.Goal,
		Size:           inferredTask.Size,
		Status:         inferredTask.Status,
		StartDate:      inferredTask.StartDate,
		EndDate:        inferredTask.EndDate,
		WeekCode:       payload.WeekCode,
		RequestReason:  inferredTask.RequestReason,
	}
	if shouldForceQuickTaskRequest(owner, requesterEmail) {
		writeRequest.Status = "요청"
		writeRequest.RequestReason = firstNonEmpty(writeRequest.RequestReason, prompt)
		if requesterID := memberIDForEmail(members, requesterEmail); requesterID != "" && !containsString(writeRequest.ParticipantIDs, requesterID) {
			writeRequest.ParticipantIDs = append(writeRequest.ParticipantIDs, requesterID)
		}
	}
	body, _ := json.Marshal(writeRequest)
	clonedRequest := request.Clone(request.Context())
	clonedRequest.Body = io.NopCloser(bytes.NewReader(body))
	task, errorValue := service.flowTaskFromRequest(clonedRequest, members, definitions, "")
	if errorValue != nil {
		writeFlowRequestError(responseWriter, errorValue)
		return
	}
	if !payload.AllowDuplicate {
		duplicateTask, reason, found, errorValue := service.findQuickFlowTaskDuplicate(request.Context(), task, members)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		if found {
			service.writeQuickFlowTaskDuplicate(responseWriter, duplicateTask, reason)
			return
		}
	}
	if errorValue := service.writeFlowTask(request.Context(), task); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task = service.syncFlowMattermostNotification(request.Context(), task)
	service.writeJSON(responseWriter, task)
}

func (service *Service) writeQuickFlowTaskDuplicate(responseWriter http.ResponseWriter, task flowTask, reason string) {
	service.writeJSON(responseWriter, map[string]any{
		"status":        "skipped_duplicate",
		"message":       "이미 추가된 Flow 업무라 건너뛰었습니다. 그래도 추가하려면 다시 추가하라고 확인해 주세요.",
		"duplicateTask": task,
		"reason":        strings.TrimSpace(reason),
	})
}

func writeFlowRequestError(responseWriter http.ResponseWriter, errorValue error) {
	http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
}

func (service *Service) flowTaskFromRequest(request *http.Request, members []flowMember, definitions flowDefinitions, taskID string) (flowTask, error) {
	var payload flowTaskWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		return flowTask{}, errorValue
	}
	memberByID := map[string]flowMember{}
	for _, member := range members {
		memberByID[member.ID] = member
	}
	owner, found := memberByID[strings.TrimSpace(payload.OwnerID)]
	if !found {
		return flowTask{}, flowValidationError("ownerID is not a known member")
	}
	participantIDs := uniqueNonEmpty(payload.ParticipantIDs)
	if len(participantIDs) == 0 {
		participantIDs = []string{owner.ID}
	}
	if !containsString(participantIDs, owner.ID) {
		participantIDs = append([]string{owner.ID}, participantIDs...)
	}
	participants := make([]flowMember, 0, len(participantIDs))
	for _, memberID := range participantIDs {
		member, found := memberByID[memberID]
		if !found {
			return flowTask{}, flowValidationError("participantID is not a known member")
		}
		participants = append(participants, member)
	}
	content := strings.TrimSpace(payload.Content)
	if content == "" {
		return flowTask{}, flowValidationError("content is required")
	}
	weekCode := strings.TrimSpace(payload.WeekCode)
	if weekCode == "" {
		weekCode = weekCodeForDate(time.Now())
	}
	status := firstNonEmpty(strings.TrimSpace(payload.Status), "예정")
	callerEmail := strings.ToLower(strings.TrimSpace(service.flowActorEmail(request)))
	if callerEmail != "" && !service.isFlowAdminEmail(request.Context(), callerEmail) && !strings.EqualFold(owner.Email, callerEmail) {
		status = "요청"
		payload.RequestReason = firstNonEmpty(strings.TrimSpace(payload.RequestReason), "타인 업무 추가 요청")
		if requesterID := memberIDForEmail(members, callerEmail); requesterID != "" && !containsString(participantIDs, requesterID) {
			participantIDs = append(participantIDs, requesterID)
			participants = append(participants, memberByID[requesterID])
		}
	}
	if !containsString(flowStatusOptions(), status) {
		return flowTask{}, flowValidationError("status is not allowed")
	}
	category := strings.TrimSpace(firstNonEmpty(payload.Category, payload.Business))
	if category != "" && len(definitions.Categories) > 0 && !containsString(definitions.Categories, category) {
		return flowTask{}, flowValidationError("category is not allowed")
	}
	taskType := firstNonEmpty(strings.TrimSpace(payload.Type), "기타")
	if !containsString(definitions.Types, taskType) {
		return flowTask{}, flowValidationError("type is not allowed")
	}
	size := firstNonEmpty(strings.ToUpper(strings.TrimSpace(payload.Size)), "M")
	if !containsFlowSize(definitions.Sizes, size) {
		return flowTask{}, flowValidationError("size is not allowed")
	}
	id := strings.TrimSpace(taskID)
	if id == "" {
		id = stableFlowID(weekCode + owner.ID + content + time.Now().UTC().Format(time.RFC3339Nano))
	}
	return flowTask{
		ID:               id,
		OwnerID:          owner.ID,
		OwnerName:        owner.Name,
		ParticipantIDs:   memberIDs(participants),
		ParticipantNames: memberNames(participants),
		Business:         category,
		Type:             taskType,
		Content:          content,
		Goal:             strings.TrimSpace(payload.Goal),
		Size:             size,
		Status:           status,
		StartDate:        strings.TrimSpace(payload.StartDate),
		EndDate:          strings.TrimSpace(payload.EndDate),
		WeekCode:         weekCode,
		Flag:             payload.Flag,
		RequestReason:    strings.TrimSpace(payload.RequestReason),
		DecisionReason:   strings.TrimSpace(payload.DecisionReason),
	}, nil
}

func (service *Service) openFlowDatabase(ctx context.Context) (*sql.DB, error) {
	if errorValue := os.MkdirAll(filepath.Dir(service.Configuration.FlowDatabasePath), 0o700); errorValue != nil {
		return nil, errorValue
	}
	database, errorValue := sql.Open("sqlite", service.Configuration.FlowDatabasePath)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := ensureFlowSchema(ctx, database); errorValue != nil {
		_ = database.Close()
		return nil, errorValue
	}
	return database, nil
}

func ensureFlowSchema(ctx context.Context, database *sql.DB) error {
	_, errorValue := database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_tasks (
	id TEXT PRIMARY KEY,
	week_code TEXT NOT NULL,
	owner_id TEXT NOT NULL,
	owner_name TEXT NOT NULL,
	participant_ids TEXT NOT NULL,
	participant_names TEXT NOT NULL,
	business TEXT NOT NULL,
	type TEXT NOT NULL,
	content TEXT NOT NULL,
	goal TEXT NOT NULL,
	size TEXT NOT NULL,
	status TEXT NOT NULL,
	start_date TEXT NOT NULL,
	end_date TEXT NOT NULL,
	flag INTEGER NOT NULL,
	request_reason TEXT NOT NULL,
	decision_reason TEXT NOT NULL,
	updated_at TEXT NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_definitions (
	kind TEXT NOT NULL,
	value TEXT NOT NULL,
	position INTEGER NOT NULL,
	PRIMARY KEY(kind, value)
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_definition_meta (
	kind TEXT PRIMARY KEY,
	initialized INTEGER NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS flow_size_definitions (
	name TEXT PRIMARY KEY,
	distance_km INTEGER NOT NULL,
	max_hours INTEGER NOT NULL,
	development_example TEXT NOT NULL,
	other_example TEXT NOT NULL,
	note TEXT NOT NULL,
	position INTEGER NOT NULL
)`)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := ensureFlowColumn(ctx, database, "flow_tasks", "mattermost_post_id", "TEXT NOT NULL DEFAULT ''"); errorValue != nil {
		return errorValue
	}
	return seedFlowDefinitions(ctx, database)
}

func ensureFlowColumn(ctx context.Context, database *sql.DB, tableName string, columnName string, definition string) error {
	rows, errorValue := database.QueryContext(ctx, "PRAGMA table_info("+tableName+")")
	if errorValue != nil {
		return errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var columnIndex int
		var existingColumnName string
		var columnType string
		var isNotNull int
		var defaultValue sql.NullString
		var primaryKey int
		if errorValue := rows.Scan(&columnIndex, &existingColumnName, &columnType, &isNotNull, &defaultValue, &primaryKey); errorValue != nil {
			return errorValue
		}
		if strings.EqualFold(existingColumnName, columnName) {
			return rows.Err()
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, "ALTER TABLE "+tableName+" ADD COLUMN "+columnName+" "+definition)
	return errorValue
}

func seedFlowDefinitions(ctx context.Context, database *sql.DB) error {
	if errorValue := seedFlowDefinitionKind(ctx, database, "category", []string{}); errorValue != nil {
		return errorValue
	}
	if errorValue := seedFlowDefinitionKind(ctx, database, "type", defaultFlowTypes()); errorValue != nil {
		return errorValue
	}
	return seedFlowSizeDefinitions(ctx, database)
}

func seedFlowDefinitionKind(ctx context.Context, database *sql.DB, kind string, values []string) error {
	var initialized int
	errorValue := database.QueryRowContext(ctx, "SELECT initialized FROM flow_definition_meta WHERE kind = ?", kind).Scan(&initialized)
	if errorValue == nil && initialized == 1 {
		return nil
	}
	if errorValue != nil && errorValue != sql.ErrNoRows {
		return errorValue
	}
	for index, value := range values {
		if _, errorValue := database.ExecContext(ctx, "INSERT OR IGNORE INTO flow_definitions(kind, value, position) VALUES(?, ?, ?)", kind, value, index); errorValue != nil {
			return errorValue
		}
	}
	_, errorValue = database.ExecContext(ctx, "INSERT INTO flow_definition_meta(kind, initialized) VALUES(?, 1) ON CONFLICT(kind) DO UPDATE SET initialized = 1", kind)
	return errorValue
}

func seedFlowSizeDefinitions(ctx context.Context, database *sql.DB) error {
	var initialized int
	errorValue := database.QueryRowContext(ctx, "SELECT initialized FROM flow_definition_meta WHERE kind = ?", "size").Scan(&initialized)
	if errorValue == nil && initialized == 1 {
		return nil
	}
	if errorValue != nil && errorValue != sql.ErrNoRows {
		return errorValue
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := replaceFlowSizeDefinitions(ctx, transaction, defaultFlowSizeDefinitions()); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) readFlowTasks(ctx context.Context, weekCode string, members []flowMember) ([]flowTask, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id
FROM flow_tasks
WHERE week_code = ?
ORDER BY status = '요청' DESC, owner_name, updated_at DESC`, weekCode)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	tasks := []flowTask{}
	for rows.Next() {
		task, errorValue := scanFlowTask(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		tasks = append(tasks, task)
	}
	return alignFlowTasksWithMembers(tasks, members), rows.Err()
}

func (service *Service) readFlowTaskByID(ctx context.Context, taskID string) (flowTask, bool, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return flowTask{}, false, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id
FROM flow_tasks
WHERE id = ?`, taskID)
	if errorValue != nil {
		return flowTask{}, false, errorValue
	}
	defer rows.Close()
	if !rows.Next() {
		return flowTask{}, false, rows.Err()
	}
	task, errorValue := scanFlowTask(rows)
	if errorValue != nil {
		return flowTask{}, false, errorValue
	}
	return task, true, rows.Err()
}

func (service *Service) existingFlowMattermostPostID(ctx context.Context, taskID string) string {
	task, found, errorValue := service.readFlowTaskByID(ctx, taskID)
	if errorValue != nil || !found {
		return ""
	}
	return task.MattermostPostID
}

func (service *Service) writeFlowTask(ctx context.Context, task flowTask) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	participantIDs, errorValue := json.Marshal(task.ParticipantIDs)
	if errorValue != nil {
		return errorValue
	}
	participantNames, errorValue := json.Marshal(task.ParticipantNames)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO flow_tasks (
	id, week_code, owner_id, owner_name, participant_ids, participant_names, business, type, content, goal, size, status, start_date, end_date, flag, request_reason, decision_reason, mattermost_post_id, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	week_code = excluded.week_code,
	owner_id = excluded.owner_id,
	owner_name = excluded.owner_name,
	participant_ids = excluded.participant_ids,
	participant_names = excluded.participant_names,
	business = excluded.business,
	type = excluded.type,
	content = excluded.content,
	goal = excluded.goal,
	size = excluded.size,
	status = excluded.status,
	start_date = excluded.start_date,
	end_date = excluded.end_date,
	flag = excluded.flag,
	request_reason = excluded.request_reason,
	decision_reason = excluded.decision_reason,
	mattermost_post_id = excluded.mattermost_post_id,
	updated_at = excluded.updated_at`,
		task.ID,
		task.WeekCode,
		task.OwnerID,
		task.OwnerName,
		string(participantIDs),
		string(participantNames),
		task.Business,
		task.Type,
		task.Content,
		task.Goal,
		task.Size,
		task.Status,
		task.StartDate,
		task.EndDate,
		task.Flag,
		task.RequestReason,
		task.DecisionReason,
		task.MattermostPostID,
		time.Now().UTC().Format(time.RFC3339),
	)
	return errorValue
}

func (service *Service) updateFlowTaskMattermostPostID(ctx context.Context, taskID string, postID string) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, "UPDATE flow_tasks SET mattermost_post_id = ?, updated_at = ? WHERE id = ?", strings.TrimSpace(postID), time.Now().UTC().Format(time.RFC3339), taskID)
	return errorValue
}

func (service *Service) syncFlowMattermostNotification(ctx context.Context, task flowTask) flowTask {
	nextTask, errorValue := service.trySyncFlowMattermostNotification(ctx, task)
	if errorValue != nil {
		log.Printf("Flow Mattermost notification sync failed: %v", errorValue)
		return task
	}
	return nextTask
}

func (service *Service) trySyncFlowMattermostNotification(ctx context.Context, task flowTask) (flowTask, error) {
	if strings.TrimSpace(service.Configuration.MattermostAdminPasswordPath) == "" {
		return task, nil
	}
	token, errorValue := service.mattermostAdminToken(ctx)
	if errorValue != nil {
		return task, errorValue
	}
	teamRecord, errorValue := service.ensureMattermostTeam(ctx, token)
	if errorValue != nil {
		return task, errorValue
	}
	channelID, errorValue := service.ensureMattermostFlowChannel(ctx, token, teamRecord.ID)
	if errorValue != nil {
		return task, errorValue
	}
	if !shouldNotifyFlowTask(task) {
		return service.deleteFlowMattermostNotification(ctx, token, task)
	}
	return service.upsertFlowMattermostNotification(ctx, token, channelID, task)
}

func shouldNotifyFlowTask(task flowTask) bool {
	switch strings.TrimSpace(task.Status) {
	case "요청", "기각", "중단", "완료":
		return true
	default:
		return false
	}
}

func (service *Service) upsertFlowMattermostNotification(ctx context.Context, token string, channelID string, task flowTask) (flowTask, error) {
	if strings.TrimSpace(task.MattermostPostID) == "" {
		return service.createFlowMattermostNotification(ctx, token, channelID, task)
	}
	body := map[string]any{
		"message": flowMattermostNotificationMessage(task),
		"props":   flowMattermostNotificationProps(task),
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPut, "/api/v4/posts/"+url.PathEscape(task.MattermostPostID)+"/patch", token, body, nil); errorValue != nil {
		return task, errorValue
	}
	return task, nil
}

func (service *Service) createFlowMattermostNotification(ctx context.Context, token string, channelID string, task flowTask) (flowTask, error) {
	body := map[string]any{
		"channel_id": channelID,
		"message":    flowMattermostNotificationMessage(task),
		"props":      flowMattermostNotificationProps(task),
	}
	var response struct {
		ID string `json:"id"`
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodPost, "/api/v4/posts", token, body, &response); errorValue != nil {
		return task, errorValue
	}
	task.MattermostPostID = strings.TrimSpace(response.ID)
	if task.MattermostPostID == "" {
		return task, nil
	}
	return task, service.updateFlowTaskMattermostPostID(ctx, task.ID, task.MattermostPostID)
}

func (service *Service) deleteFlowMattermostNotification(ctx context.Context, token string, task flowTask) (flowTask, error) {
	if strings.TrimSpace(task.MattermostPostID) == "" {
		return task, nil
	}
	if errorValue := service.mattermostRequest(ctx, http.MethodDelete, "/api/v4/posts/"+url.PathEscape(task.MattermostPostID), token, nil, nil); errorValue != nil {
		return task, errorValue
	}
	task.MattermostPostID = ""
	return task, service.updateFlowTaskMattermostPostID(ctx, task.ID, "")
}

func flowMattermostNotificationMessage(task flowTask) string {
	lines := []string{fmt.Sprintf("**%s · %s · %s**", task.Status, task.OwnerName, task.Content)}
	if task.Type != "" || task.Size != "" {
		lines = append(lines, "유형/크기: "+strings.TrimSpace(task.Type+" "+task.Size))
	}
	if len(task.ParticipantNames) > 0 {
		lines = append(lines, "참여자: "+strings.Join(task.ParticipantNames, ", "))
	}
	if reason := firstNonEmpty(task.RequestReason, task.DecisionReason); strings.TrimSpace(reason) != "" {
		lines = append(lines, "사유: "+strings.TrimSpace(reason))
	}
	lines = append(lines, mattermostFlowURL(task))
	return strings.Join(lines, "\n")
}

func flowMattermostNotificationProps(task flowTask) map[string]any {
	return map[string]any{
		"internkim_flow_task":      true,
		"internkim_flow_task_id":   task.ID,
		"internkim_flow_week_code": task.WeekCode,
	}
}

func mattermostFlowURL(task flowTask) string {
	weekCode := strings.TrimSpace(task.WeekCode)
	if weekCode == "" {
		return "[Flow 열기](/flow/)"
	}
	return "[Flow 열기](/flow/?week=" + url.QueryEscape(weekCode) + ")"
}

func (service *Service) readFlowDefinitions(ctx context.Context) (flowDefinitions, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	defer database.Close()
	definitions := flowDefinitions{}
	categories, errorValue := readFlowDefinitionValues(ctx, database, "category")
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	types, errorValue := readFlowDefinitionValues(ctx, database, "type")
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	sizes, errorValue := readFlowSizeDefinitions(ctx, database)
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	definitions.Categories = categories
	definitions.Types = types
	definitions.Sizes = sizes
	if len(definitions.Types) == 0 {
		definitions.Types = defaultFlowTypes()
	}
	if len(definitions.Sizes) == 0 {
		definitions.Sizes = defaultFlowSizeDefinitions()
	} else {
		definitions.Sizes = restoreFlowSizeDefaults(definitions.Sizes)
	}
	return definitions, nil
}

func restoreFlowSizeDefaults(sizes []flowSizeDefinition) []flowSizeDefinition {
	defaultByName := map[string]flowSizeDefinition{}
	for _, definition := range defaultFlowSizeDefinitions() {
		defaultByName[strings.ToUpper(strings.TrimSpace(definition.Name))] = definition
	}
	result := make([]flowSizeDefinition, 0, len(sizes))
	for _, size := range sizes {
		fallback, hasFallback := defaultByName[strings.ToUpper(strings.TrimSpace(size.Name))]
		if hasFallback {
			if size.DistanceKM <= 0 {
				size.DistanceKM = fallback.DistanceKM
			}
			if size.MaxHours <= 0 {
				size.MaxHours = fallback.MaxHours
			}
			if strings.TrimSpace(size.DevelopmentExample) == "" {
				size.DevelopmentExample = fallback.DevelopmentExample
			}
			if strings.TrimSpace(size.OtherExample) == "" {
				size.OtherExample = fallback.OtherExample
			}
			if strings.TrimSpace(size.Note) == "" {
				size.Note = fallback.Note
			}
		}
		size.Score = size.DistanceKM
		size.Label = flowSizeLabel(size)
		result = append(result, size)
	}
	return result
}

func readFlowDefinitionValues(ctx context.Context, database *sql.DB, kind string) ([]string, error) {
	rows, errorValue := database.QueryContext(ctx, "SELECT value FROM flow_definitions WHERE kind = ? ORDER BY position, value", kind)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if errorValue := rows.Scan(&value); errorValue != nil {
			return nil, errorValue
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func readFlowSizeDefinitions(ctx context.Context, database *sql.DB) ([]flowSizeDefinition, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT name, distance_km, max_hours, development_example, other_example, note
FROM flow_size_definitions
ORDER BY position, name`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	sizes := []flowSizeDefinition{}
	for rows.Next() {
		var size flowSizeDefinition
		if errorValue := rows.Scan(&size.Name, &size.DistanceKM, &size.MaxHours, &size.DevelopmentExample, &size.OtherExample, &size.Note); errorValue != nil {
			return nil, errorValue
		}
		size.Score = size.DistanceKM
		size.Label = flowSizeLabel(size)
		sizes = append(sizes, size)
	}
	return sizes, rows.Err()
}

func (service *Service) writeFlowDefinitions(ctx context.Context, definitions flowDefinitions) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := replaceFlowDefinitionKind(ctx, transaction, "category", definitions.Categories); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceFlowDefinitionKind(ctx, transaction, "type", definitions.Types); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceFlowSizeDefinitions(ctx, transaction, definitions.Sizes); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func flowOwnerFromQuickRequest(payload flowQuickTaskRequest, members []flowMember, callerEmail string) (flowMember, error) {
	memberByID := map[string]flowMember{}
	for _, member := range members {
		memberByID[member.ID] = member
	}
	if owner, found := memberByID[strings.TrimSpace(payload.OwnerID)]; found {
		return owner, nil
	}
	for _, member := range members {
		if strings.EqualFold(member.Email, callerEmail) {
			return member, nil
		}
	}
	if len(members) > 0 {
		return members[0], nil
	}
	return flowMember{}, flowValidationError("ownerID is not a known member")
}

func (service *Service) flowRequesterEmail(request *http.Request, payload flowQuickTaskRequest) string {
	if actorEmail := service.flowActorEmail(request); actorEmail != "" {
		return actorEmail
	}
	if isLocalRequest(request) && strings.TrimSpace(payload.RequesterEmail) != "" {
		return strings.ToLower(strings.TrimSpace(payload.RequesterEmail))
	}
	return strings.ToLower(strings.TrimSpace(authenticatedCallerEmail(request)))
}

func shouldForceQuickTaskRequest(owner flowMember, requesterEmail string) bool {
	if strings.TrimSpace(requesterEmail) == "" {
		return false
	}
	if strings.EqualFold(owner.Email, requesterEmail) {
		return false
	}
	return true
}

func memberIDForEmail(members []flowMember, email string) string {
	for _, member := range members {
		if strings.EqualFold(member.Email, strings.TrimSpace(email)) {
			return member.ID
		}
	}
	return ""
}

func (service *Service) inferFlowTask(ctx context.Context, prompt string, weekCode string, owner flowMember, members []flowMember, definitions flowDefinitions) (inferredFlowTask, error) {
	requestDocument, errorValue := json.Marshal(flowLLMRequest(prompt, weekCode, owner, members, definitions))
	if errorValue != nil {
		return inferredFlowTask{}, errorValue
	}
	responseDocument, errorValue := service.callCapabilityStructuredLLM(ctx, requestDocument)
	if errorValue != nil {
		return inferredFlowTask{}, errorValue
	}
	var response capabilityLLMResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return inferredFlowTask{}, errorValue
	}
	var task inferredFlowTask
	if errorValue := json.Unmarshal([]byte(response.Content), &task); errorValue != nil {
		return inferredFlowTask{}, fmt.Errorf("flow task inference returned invalid JSON: %w", errorValue)
	}
	task.Content = firstNonEmpty(strings.TrimSpace(task.Content), prompt)
	task.Type = firstNonEmpty(strings.TrimSpace(task.Type), "기타")
	task.Size = firstNonEmpty(strings.ToUpper(strings.TrimSpace(task.Size)), "XS")
	task.Status = firstNonEmpty(strings.TrimSpace(task.Status), "예정")
	if !containsString(definitions.Types, task.Type) {
		task.Type = "기타"
	}
	if !containsFlowSize(definitions.Sizes, task.Size) {
		task.Size = "XS"
	}
	if task.Category != "" && !containsString(definitions.Categories, task.Category) {
		task.Category = ""
	}
	if !containsString(flowStatusOptions(), task.Status) {
		task.Status = "예정"
	}
	task.StartDate = strings.TrimSpace(task.StartDate)
	task.EndDate = strings.TrimSpace(task.EndDate)
	task.ParticipantIDs = cleanParticipantIDs(task.ParticipantIDs, members, owner.ID)
	return task, nil
}

func (service *Service) findQuickFlowTaskDuplicate(ctx context.Context, task flowTask, members []flowMember) (flowTask, string, bool, error) {
	existingTasks, errorValue := service.readFlowTasks(ctx, task.WeekCode, members)
	if errorValue != nil {
		return flowTask{}, "", false, errorValue
	}
	sameDateTasks := flowTasksWithMatchingDates(existingTasks, task)
	if len(sameDateTasks) == 0 {
		return flowTask{}, "", false, nil
	}
	decision, errorValue := service.decideFlowTaskDuplicate(ctx, task, sameDateTasks)
	if errorValue != nil {
		return flowTask{}, "", false, errorValue
	}
	if !decision.IsDuplicate {
		return flowTask{}, "", false, nil
	}
	duplicateTask, found := flowTaskByID(sameDateTasks, decision.DuplicateTaskID)
	if !found {
		duplicateTask = sameDateTasks[0]
	}
	return duplicateTask, decision.Reason, true, nil
}

func (service *Service) decideFlowTaskDuplicate(ctx context.Context, task flowTask, existingTasks []flowTask) (flowDuplicateDecision, error) {
	requestDocument, errorValue := json.Marshal(flowDuplicateLLMRequest(task, existingTasks))
	if errorValue != nil {
		return flowDuplicateDecision{}, errorValue
	}
	responseDocument, errorValue := service.callCapabilityStructuredLLM(ctx, requestDocument)
	if errorValue != nil {
		return flowDuplicateDecision{}, errorValue
	}
	var response capabilityLLMResponse
	if errorValue := json.Unmarshal(responseDocument, &response); errorValue != nil {
		return flowDuplicateDecision{}, errorValue
	}
	var decision flowDuplicateDecision
	if errorValue := json.Unmarshal([]byte(response.Content), &decision); errorValue != nil {
		return flowDuplicateDecision{}, fmt.Errorf("flow duplicate guard returned invalid JSON: %w", errorValue)
	}
	decision.DuplicateTaskID = strings.TrimSpace(decision.DuplicateTaskID)
	decision.Reason = strings.TrimSpace(decision.Reason)
	return decision, nil
}

func (service *Service) callCapabilityStructuredLLM(ctx context.Context, requestDocument []byte) ([]byte, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network string, address string) (net.Conn, error) {
			_ = network
			_ = address
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", blueclawruntime.CapabilitySocketPath)
		},
	}
	client := http.Client{Transport: transport, Timeout: 45 * time.Second}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost, "http://internkim/v1/llm/structured", bytes.NewReader(requestDocument))
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Content-Type", "application/json")
	response, errorValue := client.Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	body, readError := io.ReadAll(response.Body)
	if readError != nil {
		return nil, readError
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("flow task inference failed: %s", strings.TrimSpace(string(body)))
	}
	return body, nil
}

func flowTasksWithMatchingDates(tasks []flowTask, task flowTask) []flowTask {
	result := []flowTask{}
	for _, existingTask := range tasks {
		if strings.TrimSpace(existingTask.ID) == strings.TrimSpace(task.ID) {
			continue
		}
		if strings.TrimSpace(existingTask.StartDate) != strings.TrimSpace(task.StartDate) {
			continue
		}
		if strings.TrimSpace(existingTask.EndDate) != strings.TrimSpace(task.EndDate) {
			continue
		}
		result = append(result, existingTask)
	}
	return result
}

func flowTaskByID(tasks []flowTask, taskID string) (flowTask, bool) {
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) == strings.TrimSpace(taskID) {
			return task, true
		}
	}
	return flowTask{}, false
}

func flowLLMRequest(prompt string, weekCode string, owner flowMember, members []flowMember, definitions flowDefinitions) map[string]any {
	return map[string]any{
		"executionMode": "remote",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You convert short task notes into a weekly work tracker task. Return only values allowed by the schema. Keep Korean task content concise. Pick the closest type and size. Use status 예정 unless the note clearly says 진행, 완료, 요청, 기각, 일시정지, or 중단. Use YYYY-MM-DD dates only when the note clearly names a date; otherwise use empty strings.",
			},
			{
				"role":    "user",
				"content": flowInferencePrompt(prompt, weekCode, owner, members, definitions),
			},
		},
		"structuredOutputSchema": map[string]any{
			"name":               "flow_task_inference",
			"isStrictlyEnforced": true,
			"document": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"category", "type", "content", "goal", "size", "status", "startDate", "endDate", "participantIDs", "requestReason"},
				"properties": map[string]any{
					"category":       map[string]any{"type": "string", "enum": append([]string{""}, definitions.Categories...)},
					"type":           map[string]any{"type": "string", "enum": definitions.Types},
					"content":        map[string]any{"type": "string"},
					"goal":           map[string]any{"type": "string"},
					"size":           map[string]any{"type": "string", "enum": flowSizeNames(definitions.Sizes)},
					"status":         map[string]any{"type": "string", "enum": flowStatusOptions()},
					"startDate":      map[string]any{"type": "string"},
					"endDate":        map[string]any{"type": "string"},
					"participantIDs": map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": memberIDOptions(members)}},
					"requestReason":  map[string]any{"type": "string"},
				},
			},
		},
	}
}

func flowDuplicateLLMRequest(task flowTask, existingTasks []flowTask) map[string]any {
	return map[string]any{
		"executionMode": "remote",
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a duplicate guard for a weekly work tracker. Decide whether the candidate is substantially the same real-world task as one of the existing tasks. The existing tasks already have the same start and end dates as the candidate. Treat paraphrases and translations as duplicates. Do not treat related follow-ups, separate meetings, or different deliverables as duplicates. If there is no duplicate, set duplicateTaskID to an empty string.",
			},
			{
				"role":    "user",
				"content": flowDuplicatePrompt(task, existingTasks),
			},
		},
		"structuredOutputSchema": map[string]any{
			"name":               "flow_task_duplicate_guard",
			"isStrictlyEnforced": true,
			"document": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"isDuplicate", "duplicateTaskID", "reason"},
				"properties": map[string]any{
					"isDuplicate":     map[string]any{"type": "boolean"},
					"duplicateTaskID": map[string]any{"type": "string", "enum": append([]string{""}, flowTaskIDs(existingTasks)...)},
					"reason":          map[string]any{"type": "string"},
				},
			},
		},
	}
}

func flowInferencePrompt(prompt string, weekCode string, owner flowMember, members []flowMember, definitions flowDefinitions) string {
	memberLines := make([]string, 0, len(members))
	for _, member := range members {
		memberLines = append(memberLines, member.ID+"="+member.Name+"<"+member.Email+">")
	}
	now := time.Now()
	resolvedWeekCode := firstNonEmpty(strings.TrimSpace(weekCode), weekCodeForDate(now))
	weekStart := weekStartForCode(resolvedWeekCode, now)
	return strings.Join([]string{
		"Task note: " + prompt,
		"Today: " + now.Format("2006-01-02"),
		"Week code: " + resolvedWeekCode,
		"Week dates: " + weekStart.Format("2006-01-02") + " to " + weekStart.AddDate(0, 0, 6).Format("2006-01-02"),
		"Default owner ID: " + owner.ID,
		"Members: " + strings.Join(memberLines, ", "),
		"Categories: " + strings.Join(definitions.Categories, ", "),
		"Types: " + strings.Join(definitions.Types, ", "),
		"Size rubric: " + flowSizeRubricForPrompt(definitions.Sizes),
	}, "\n")
}

func flowDuplicatePrompt(task flowTask, existingTasks []flowTask) string {
	lines := []string{
		"Candidate: " + flowTaskDuplicateLine(task),
		"Existing tasks with the same start and end dates:",
	}
	for _, existingTask := range existingTasks {
		lines = append(lines, "- "+flowTaskDuplicateLine(existingTask))
	}
	return strings.Join(lines, "\n")
}

func flowTaskDuplicateLine(task flowTask) string {
	return strings.Join([]string{
		"id=" + task.ID,
		"owner=" + task.OwnerName,
		"startDate=" + task.StartDate,
		"endDate=" + task.EndDate,
		"type=" + task.Type,
		"size=" + task.Size,
		"status=" + task.Status,
		"content=" + task.Content,
		"goal=" + task.Goal,
	}, " | ")
}

func flowTaskIDs(tasks []flowTask) []string {
	values := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if strings.TrimSpace(task.ID) != "" {
			values = append(values, task.ID)
		}
	}
	return values
}

func flowSizeRubricForPrompt(sizes []flowSizeDefinition) string {
	lines := make([]string, 0, len(sizes))
	for _, size := range sizes {
		lines = append(lines, fmt.Sprintf("%s=%dkm max %dh; dev: %s; other: %s; note: %s", size.Name, size.DistanceKM, size.MaxHours, size.DevelopmentExample, size.OtherExample, size.Note))
	}
	return strings.Join(lines, " | ")
}

func memberIDOptions(members []flowMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.ID)
	}
	return values
}

func flowSizeNames(sizes []flowSizeDefinition) []string {
	values := make([]string, 0, len(sizes))
	for _, size := range sizes {
		if strings.TrimSpace(size.Name) != "" {
			values = append(values, size.Name)
		}
	}
	if len(values) == 0 {
		return []string{"XS", "S", "M", "L", "XL", "XXL"}
	}
	return values
}

func cleanParticipantIDs(values []string, members []flowMember, ownerID string) []string {
	allowed := map[string]bool{}
	for _, member := range members {
		allowed[member.ID] = true
	}
	result := []string{ownerID}
	seen := map[string]bool{ownerID: true}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if !allowed[trimmedValue] || seen[trimmedValue] {
			continue
		}
		seen[trimmedValue] = true
		result = append(result, trimmedValue)
	}
	return result
}

func replaceFlowDefinitionKind(ctx context.Context, transaction *sql.Tx, kind string, values []string) error {
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM flow_definitions WHERE kind = ?", kind); errorValue != nil {
		return errorValue
	}
	for index, value := range values {
		if _, errorValue := transaction.ExecContext(ctx, "INSERT INTO flow_definitions(kind, value, position) VALUES(?, ?, ?)", kind, value, index); errorValue != nil {
			return errorValue
		}
	}
	_, errorValue := transaction.ExecContext(ctx, "INSERT INTO flow_definition_meta(kind, initialized) VALUES(?, 1) ON CONFLICT(kind) DO UPDATE SET initialized = 1", kind)
	return errorValue
}

func replaceFlowSizeDefinitions(ctx context.Context, transaction interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, sizes []flowSizeDefinition) error {
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM flow_size_definitions"); errorValue != nil {
		return errorValue
	}
	for index, size := range sizes {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO flow_size_definitions(name, distance_km, max_hours, development_example, other_example, note, position)
VALUES(?, ?, ?, ?, ?, ?, ?)`,
			size.Name,
			size.DistanceKM,
			size.MaxHours,
			size.DevelopmentExample,
			size.OtherExample,
			size.Note,
			index,
		); errorValue != nil {
			return errorValue
		}
	}
	_, errorValue := transaction.ExecContext(ctx, "INSERT INTO flow_definition_meta(kind, initialized) VALUES(?, 1) ON CONFLICT(kind) DO UPDATE SET initialized = 1", "size")
	return errorValue
}

func scanFlowTask(rows *sql.Rows) (flowTask, error) {
	var task flowTask
	var participantIDsDocument string
	var participantNamesDocument string
	errorValue := rows.Scan(
		&task.ID,
		&task.WeekCode,
		&task.OwnerID,
		&task.OwnerName,
		&participantIDsDocument,
		&participantNamesDocument,
		&task.Business,
		&task.Type,
		&task.Content,
		&task.Goal,
		&task.Size,
		&task.Status,
		&task.StartDate,
		&task.EndDate,
		&task.Flag,
		&task.RequestReason,
		&task.DecisionReason,
		&task.MattermostPostID,
	)
	if errorValue != nil {
		return flowTask{}, errorValue
	}
	_ = json.Unmarshal([]byte(participantIDsDocument), &task.ParticipantIDs)
	_ = json.Unmarshal([]byte(participantNamesDocument), &task.ParticipantNames)
	return task, nil
}

func alignFlowTasksWithMembers(tasks []flowTask, members []flowMember) []flowTask {
	memberByID := map[string]flowMember{}
	for _, member := range members {
		memberByID[member.ID] = member
	}
	for index, task := range tasks {
		if owner, found := memberByID[task.OwnerID]; found {
			tasks[index].OwnerName = owner.Name
		}
		names := make([]string, 0, len(task.ParticipantIDs))
		for _, memberID := range task.ParticipantIDs {
			if member, found := memberByID[memberID]; found {
				names = append(names, member.Name)
			}
		}
		if len(names) > 0 {
			tasks[index].ParticipantNames = names
		}
	}
	return tasks
}

func (service *Service) flowMembers(request *http.Request) []flowMember {
	fleetID := strings.ToLower(strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath)))
	fleetSecret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	if fleetID != "" && fleetSecret != "" {
		if records, errorValue := service.lookupUserRecords(request.Context(), fleetID, fleetSecret); errorValue == nil && len(records) > 0 {
			return membersFromUserRecords(records)
		}
	}
	return defaultFlowMembers()
}

func membersFromUserRecords(records []adminUserMutation) []flowMember {
	members := make([]flowMember, 0, len(records))
	for _, record := range records {
		email := strings.ToLower(strings.TrimSpace(record.Email))
		if email == "" {
			continue
		}
		name := strings.TrimSpace(record.Name)
		if name == "" {
			name = strings.TrimSpace(record.MattermostUsername)
		}
		if name == "" {
			name = strings.TrimSuffix(email, "@"+emailDomain(email))
		}
		members = append(members, flowMember{
			ID:               stableFlowID(email),
			Name:             name,
			Email:            email,
			Role:             normalizeAdminUserRole(record.Role),
			MattermostStatus: firstNonEmpty(record.Status, "active"),
		})
	}
	sort.Slice(members, func(leftIndex int, rightIndex int) bool {
		if members[leftIndex].Role != members[rightIndex].Role {
			return members[leftIndex].Role == "admin"
		}
		return members[leftIndex].Email < members[rightIndex].Email
	})
	return members
}

func resolveCurrentUserName(members []flowMember, email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return ""
	}
	for _, member := range members {
		if strings.EqualFold(strings.TrimSpace(member.Email), normalized) {
			return member.Name
		}
	}
	return strings.TrimSuffix(normalized, "@"+emailDomain(normalized))
}

func defaultFlowMembers() []flowMember {
	names := []string{"김여명", "이샘플", "김테스트", "박예시", "장석민", "곽성재"}
	members := make([]flowMember, 0, len(names))
	for _, name := range names {
		members = append(members, flowMember{
			ID:               stableFlowID(name),
			Name:             name,
			Email:            "",
			Role:             "member",
			MattermostStatus: "sample",
		})
	}
	return members
}

func buildSeedFlowTasks(weekCode string, weekStart time.Time, members []flowMember) []flowTask {
	if len(members) == 0 {
		members = defaultFlowMembers()
	}
	taskTemplates := []struct {
		OwnerOffset       int
		ParticipantOffset int
		Business          string
		Type              string
		Content           string
		Goal              string
		Size              string
		Status            string
		Flag              int
	}{
		{0, 1, "운영", "개선", "Mattermost 초대와 권한 흐름 정리", "초대 후 바로 업무 공간에 들어올 수 있게 한다", "M", "진행", 0},
		{1, 2, "제품", "기능", "Flow 주간 업무 화면 베타 구현", "이번 주 업무와 개인별 탭을 한 화면에서 확인한다", "L", "진행", 1},
		{2, 0, "AI", "리서치", "브라우저 스크린샷 전달 경로 점검", "캡처 파일이 답변 첨부로 전달되는지 검증한다", "S", "완료", 0},
		{3, 4, "운영", "요청", "타인 업무 요청 승인 플로우 설계", "요청/기각/중단 상태를 운영 정책에 맞춘다", "M", "요청", 0},
		{4, 5, "데이터", "분석", "구성원별 점수 산정식 검토", "크기와 상태 기반의 주간 점수를 비교한다", "S", "예정", 0},
		{5, 0, "보안", "점검", "관리자 페이지 접근 권한 재확인", "Access 인증과 내부 admin role을 분리해 확인한다", "M", "일시정지", 1},
	}
	tasks := make([]flowTask, 0, len(taskTemplates))
	for index, template := range taskTemplates {
		owner := members[template.OwnerOffset%len(members)]
		participant := members[template.ParticipantOffset%len(members)]
		participants := []flowMember{owner}
		if participant.ID != owner.ID {
			participants = append(participants, participant)
		}
		task := flowTask{
			ID:               stableFlowID(weekCode + template.Content),
			OwnerID:          owner.ID,
			OwnerName:        owner.Name,
			ParticipantIDs:   memberIDs(participants),
			ParticipantNames: memberNames(participants),
			Business:         template.Business,
			Type:             template.Type,
			Content:          template.Content,
			Goal:             template.Goal,
			Size:             template.Size,
			Status:           template.Status,
			StartDate:        weekStart.AddDate(0, 0, index%5).Format("2006-01-02"),
			WeekCode:         weekCode,
			Flag:             template.Flag,
		}
		if template.Status == "완료" {
			task.EndDate = weekStart.AddDate(0, 0, index%5+1).Format("2006-01-02")
		}
		if template.Status == "요청" {
			task.RequestReason = "공동 작업으로 등록 요청됨"
		}
		tasks = append(tasks, task)
	}
	return tasks
}

func scoreFlowMembers(members []flowMember, tasks []flowTask, definitions flowDefinitions) []flowMember {
	result := append([]flowMember(nil), members...)
	memberIndex := map[string]int{}
	for index, member := range result {
		memberIndex[member.ID] = index
	}
	for _, task := range tasks {
		for _, memberID := range task.ParticipantIDs {
			index, found := memberIndex[memberID]
			if !found {
				continue
			}
			if task.Status == "완료" {
				result[index].CompleteTaskCount++
			} else if task.Status != "기각" && task.Status != "중단" {
				result[index].ActiveTaskCount++
			}
			result[index].Score += scoreForTask(task, definitions)
		}
	}
	return result
}

func buildFlowMetrics(tasks []flowTask, definitions flowDefinitions) flowMetrics {
	metrics := flowMetrics{
		StatusCounts:   map[string]int{},
		BusinessCounts: map[string]int{},
		TypeCounts:     map[string]int{},
		MemberScores:   map[string]int{},
	}
	for _, task := range tasks {
		metrics.TotalTasks++
		metrics.StatusCounts[task.Status]++
		if task.Business != "" {
			metrics.BusinessCounts[task.Business]++
		}
		metrics.TypeCounts[task.Type]++
		if task.Status == "완료" {
			metrics.CompletedTasks++
		}
		if task.Status == "요청" {
			metrics.RequestedTasks++
		}
		if task.Status == "일시정지" {
			metrics.PausedTasks++
		}
		if task.Status == "중단" {
			metrics.StoppedTasks++
		}
		score := scoreForTask(task, definitions)
		metrics.TotalScore += score
		for _, name := range task.ParticipantNames {
			metrics.MemberScores[name] += score
		}
	}
	return metrics
}

func scoreForTask(task flowTask, definitions flowDefinitions) int {
	distance := distanceForTaskSize(task.Size, definitions.Sizes)
	switch task.Status {
	case "완료":
		return distance
	case "진행":
		return distance / 2
	case "요청", "예정", "일시정지":
		return 0
	default:
		return 0
	}
}

func distanceForTaskSize(sizeName string, definitions []flowSizeDefinition) int {
	normalizedName := strings.ToUpper(strings.TrimSpace(sizeName))
	for _, definition := range definitions {
		if strings.EqualFold(definition.Name, normalizedName) {
			return definition.DistanceKM
		}
	}
	for _, definition := range defaultFlowSizeDefinitions() {
		if strings.EqualFold(definition.Name, normalizedName) {
			return definition.DistanceKM
		}
	}
	return 0
}

func defaultFlowTypes() []string {
	return []string{"기능", "개선", "변경", "수정", "기획", "디자인", "마케팅", "운영", "회의", "미팅", "문서", "기타"}
}

func defaultFlowSizeDefinitions() []flowSizeDefinition {
	return []flowSizeDefinition{
		sizeDefinition("XS", 1, 1, "아주 사소한 변경", "전화 / 10분 회의 / 전달 / 정리 / 일정 조율", "잠깐이면 끝낼 것"),
		sizeDefinition("S", 2, 2, "난이도 낮고 영향 범위 좁은 변경", "30분 내외 회의 / 간단 문서 초안 / 고객 및 파트너 대응 / 브리핑", "하루 여러 번도 처리 가능한 것"),
		sizeDefinition("M", 3, 8, "소형 기능 추가 / 영향 있는 변경", "보고서 작성 / 2시간 이내 회의 / 외부 미팅 / 팀 간 조율 / 문서 작성", "하루 날 잡고 해야 할 것"),
		sizeDefinition("L", 5, 16, "중형 기능 추가 / 다수 영향 있는 변경", "중요 외부 미팅 / 중형 리서치 / 기획서 초안 / 정책 변경", "이틀은 걸릴 것"),
		sizeDefinition("XL", 8, 32, "대형 기능 추가 / 복잡한 변경 / 외부 연동", "재정비 / 협상 / 장시간 미팅 / 워크샵", "일주일은 걸릴 것"),
		sizeDefinition("XXL", 13, 128, "마일스톤", "파트너십 설계 / 계약 구조 설계 / 서비스 기획 / 정책 개편", "반드시 하위 항목으로 쪼갤 것"),
	}
}

func sizeDefinition(name string, distanceKM int, maxHours int, developmentExample string, otherExample string, note string) flowSizeDefinition {
	size := flowSizeDefinition{
		Name:               name,
		DistanceKM:         distanceKM,
		MaxHours:           maxHours,
		DevelopmentExample: developmentExample,
		OtherExample:       otherExample,
		Note:               note,
		Score:              distanceKM,
	}
	size.Label = flowSizeLabel(size)
	return size
}

func flowSizeLabel(size flowSizeDefinition) string {
	return fmt.Sprintf("%dkm · 최대 %dh", size.DistanceKM, size.MaxHours)
}

func flowStatusOptions() []string {
	return []string{"요청", "예정", "진행", "완료", "일시정지", "기각", "중단"}
}

func buildFlowWeek(weekCode string, weekStart time.Time, now time.Time) flowWeek {
	return flowWeek{
		Code:      weekCode,
		StartISO:  weekStart.Format("2006-01-02"),
		EndISO:    weekStart.AddDate(0, 0, 6).Format("2006-01-02"),
		Previous:  weekCodeForDate(weekStart.AddDate(0, 0, -7)),
		Next:      weekCodeForDate(weekStart.AddDate(0, 0, 7)),
		IsCurrent: weekCode == weekCodeForDate(now),
	}
}

func weekStartForCode(weekCode string, fallback time.Time) time.Time {
	trimmedCode := strings.TrimSpace(strings.ToUpper(weekCode))
	if len(trimmedCode) != 5 || trimmedCode[2] != 'W' {
		return startOfISOWeek(fallback)
	}
	yearValue, yearError := strconv.Atoi(trimmedCode[:2])
	weekValue, weekError := strconv.Atoi(trimmedCode[3:])
	if yearError != nil || weekError != nil || weekValue < 1 || weekValue > 53 {
		return startOfISOWeek(fallback)
	}
	year := 2000 + yearValue
	janFourth := time.Date(year, time.January, 4, 0, 0, 0, 0, fallback.Location())
	return startOfISOWeek(janFourth).AddDate(0, 0, (weekValue-1)*7)
}

func weekCodeForDate(date time.Time) string {
	year, week := date.ISOWeek()
	return strconv.Itoa(year%100) + "W" + twoDigitNumber(week)
}

func startOfISOWeek(date time.Time) time.Time {
	year, month, day := date.Date()
	start := time.Date(year, month, day, 0, 0, 0, 0, date.Location())
	weekday := int(start.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return start.AddDate(0, 0, 1-weekday)
}

func twoDigitNumber(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func stableFlowID(value string) string {
	digest := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(digest[:])[:12]
}

func emailDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

func memberIDs(members []flowMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.ID)
	}
	return values
}

func memberNames(members []flowMember) []string {
	values := make([]string, 0, len(members))
	for _, member := range members {
		values = append(values, member.Name)
	}
	return values
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" || seen[trimmedValue] {
			continue
		}
		seen[trimmedValue] = true
		result = append(result, trimmedValue)
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsFlowSize(values []flowSizeDefinition, target string) bool {
	for _, value := range values {
		if value.Name == target {
			return true
		}
	}
	return false
}

func cleanFlowDefinitionValues(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		trimmedValue := strings.TrimSpace(value)
		if trimmedValue == "" || seen[trimmedValue] {
			continue
		}
		seen[trimmedValue] = true
		result = append(result, trimmedValue)
	}
	return result
}

func cleanFlowSizeDefinitions(values []flowSizeDefinition) []flowSizeDefinition {
	result := []flowSizeDefinition{}
	seen := map[string]bool{}
	for _, value := range values {
		name := strings.ToUpper(strings.TrimSpace(value.Name))
		if name == "" || seen[name] {
			continue
		}
		distanceKM := value.DistanceKM
		if distanceKM <= 0 {
			distanceKM = 1
		}
		maxHours := value.MaxHours
		if maxHours <= 0 {
			maxHours = distanceKM
		}
		size := flowSizeDefinition{
			Name:               name,
			DistanceKM:         distanceKM,
			MaxHours:           maxHours,
			DevelopmentExample: strings.TrimSpace(value.DevelopmentExample),
			OtherExample:       strings.TrimSpace(value.OtherExample),
			Note:               strings.TrimSpace(value.Note),
			Score:              distanceKM,
		}
		size.Label = flowSizeLabel(size)
		seen[name] = true
		result = append(result, size)
	}
	if len(result) == 0 {
		return defaultFlowSizeDefinitions()
	}
	return result
}

func firstNonEmptySlice(values ...[]string) []string {
	for _, value := range values {
		cleanedValue := uniqueNonEmpty(value)
		if len(cleanedValue) > 0 {
			return cleanedValue
		}
	}
	return []string{}
}
