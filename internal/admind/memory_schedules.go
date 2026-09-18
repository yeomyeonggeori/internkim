package admind

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type memoryScheduleCancelRequest struct {
	TaskScheduleID string `json:"taskScheduleID"`
}

type scheduleToolListInput struct {
	Status string `json:"status"`
	Limit  int    `json:"limit"`
}

// The shapes Blueclaw's signed schedule contract accepts. They are decoded here
// only to refuse a document that contract would refuse anyway; what travels on
// is the exact body that was validated, so a field the caller left out stays
// left out rather than arriving as a zero value the schema rejects.
type scheduleToolCreateInput struct {
	TaskRunID       *string `json:"taskRunID"`
	TaskInstruction string  `json:"taskInstruction"`
	Description     *string `json:"description"`
	Kind            string  `json:"kind"`
	RunAt           *string `json:"runAt"`
	ExpiresAt       *string `json:"expiresAt"`
	IntervalSecond  *int    `json:"intervalSecond"`
	CronExpression  *string `json:"cronExpression"`
	TimeZone        *string `json:"timeZone"`
	MaxRunCount     *int    `json:"maxRunCount"`
	RepeatPolicy    *string `json:"repeatPolicy"`
	Platform        *string `json:"platform"`
	ConversationID  *string `json:"conversationID"`
	ReplyTargetID   *string `json:"replyTargetID"`
}

type scheduleToolUpdateInput struct {
	TaskRunID       *string `json:"taskRunID"`
	ScheduleHint    string  `json:"scheduleHint"`
	TaskInstruction *string `json:"taskInstruction"`
	Description     *string `json:"description"`
	Kind            *string `json:"kind"`
	RunAt           *string `json:"runAt"`
	ExpiresAt       *string `json:"expiresAt"`
	IntervalSecond  *int    `json:"intervalSecond"`
	CronExpression  *string `json:"cronExpression"`
	TimeZone        *string `json:"timeZone"`
	MaxRunCount     *int    `json:"maxRunCount"`
	RepeatPolicy    *string `json:"repeatPolicy"`
}

type scheduleToolCancelInput struct {
	ScheduleHints []string `json:"scheduleHints"`
}

type memoryScheduleCancelBlueclawRequest struct {
	TaskScheduleID  string `json:"taskScheduleID"`
	CreatorPersonID string `json:"creatorPersonID"`
}

type memoryScheduleDeleteRequest struct {
	TaskScheduleID string `json:"taskScheduleID"`
}

type memoryScheduleDeleteBlueclawRequest struct {
	TaskScheduleID  string `json:"taskScheduleID"`
	CreatorPersonID string `json:"creatorPersonID"`
}

type memoryScheduleUpdateRequest struct {
	TaskScheduleID string  `json:"taskScheduleID"`
	Name           *string `json:"name"`
	Kind           *string `json:"kind"`
	RunAt          *string `json:"runAt"`
	IntervalSecond *int    `json:"intervalSecond"`
	CronExpression *string `json:"cronExpression"`
	TimeZone       *string `json:"timeZone"`
	ExpiresAt      *string `json:"expiresAt"`
	MaxRunCount    *int    `json:"maxRunCount"`
	RepeatPolicy   *string `json:"repeatPolicy"`
}

type memoryScheduleUpdateBlueclawRequest struct {
	TaskScheduleID  string  `json:"taskScheduleID"`
	CreatorPersonID string  `json:"creatorPersonID"`
	Name            *string `json:"name,omitempty"`
	Kind            *string `json:"kind,omitempty"`
	RunAt           *string `json:"runAt,omitempty"`
	IntervalSecond  *int    `json:"intervalSecond,omitempty"`
	CronExpression  *string `json:"cronExpression,omitempty"`
	TimeZone        *string `json:"timeZone,omitempty"`
	ExpiresAt       *string `json:"expiresAt,omitempty"`
	MaxRunCount     *int    `json:"maxRunCount,omitempty"`
	RepeatPolicy    *string `json:"repeatPolicy,omitempty"`
}

const (
	memorySchedulesDefaultPage     = 1
	memorySchedulesDefaultPageSize = 25
	memorySchedulesMaxPageSize     = 100
	scheduleToolRequestByteLimit   = 16 * 1024
)

func (service *Service) writeUserMemorySchedules(responseWriter http.ResponseWriter, request *http.Request) {
	actorEmail := service.memoryActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "memory access required", http.StatusForbidden)
		return
	}
	personID, errorValue := service.resolveMemoryPersonID(request.Context(), actorEmail)
	if errorValue != nil {
		log.Printf("memory schedules identity resolution failed: %v", errorValue)
		http.Error(responseWriter, "memory identity unavailable", http.StatusBadGateway)
		return
	}
	if personID == "" {
		http.Error(responseWriter, "memory person not found", http.StatusNotFound)
		return
	}

	var schedules map[string]any
	path := "/admin/api/schedule?" + memorySchedulesQuery(request.URL.Query(), personID)
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, path, nil, &schedules); errorValue != nil {
		log.Printf("memory schedules upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedules unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, schedules)
}

func (service *Service) writeUserScheduleToolList(responseWriter http.ResponseWriter, request *http.Request) {
	personID, ok := service.writeMemorySchedulePersonID(responseWriter, request)
	if !ok {
		return
	}
	var input scheduleToolListInput
	decoder := json.NewDecoder(http.MaxBytesReader(responseWriter, request.Body, 16*1024))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&input); errorValue != nil {
		http.Error(responseWriter, "invalid schedule list request", http.StatusBadRequest)
		return
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		http.Error(responseWriter, "invalid schedule list request", http.StatusBadRequest)
		return
	}
	body, errorValue := json.Marshal(input)
	if errorValue != nil {
		http.Error(responseWriter, "invalid schedule list request", http.StatusBadRequest)
		return
	}
	var output json.RawMessage
	if errorValue := service.blueclawSignedRequest(request.Context(), http.MethodPost, "/admin/api/schedule/tool-list", body, personID, &output); errorValue != nil {
		log.Printf("schedule tool list upstream failed: %v", errorValue)
		http.Error(responseWriter, "schedule list unavailable", http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	_, _ = responseWriter.Write(output)
}

func (service *Service) writeUserScheduleToolCreate(responseWriter http.ResponseWriter, request *http.Request) {
	service.forwardScheduleToolWrite(responseWriter, request, "/admin/api/schedule/tool-create", &scheduleToolCreateInput{})
}

func (service *Service) writeUserScheduleToolUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	service.forwardScheduleToolWrite(responseWriter, request, "/admin/api/schedule/tool-update", &scheduleToolUpdateInput{})
}

func (service *Service) writeUserScheduleToolCancel(responseWriter http.ResponseWriter, request *http.Request) {
	service.forwardScheduleToolWrite(responseWriter, request, "/admin/api/schedule/tool-cancel", &scheduleToolCancelInput{})
}

// Blueclaw owns what a schedule write means, including which hint resolved and
// which did not, so its status and its body reach the caller unchanged.
func (service *Service) forwardScheduleToolWrite(responseWriter http.ResponseWriter, request *http.Request, upstreamPath string, input any) {
	personID, isKnown := service.writeMemorySchedulePersonID(responseWriter, request)
	if !isKnown {
		return
	}
	body, isValid := readOneScheduleToolDocument(responseWriter, request, input)
	if !isValid {
		return
	}
	statusCode, answer, errorValue := service.blueclawSignedAnswer(request.Context(), http.MethodPost, upstreamPath, body, personID)
	if errorValue != nil {
		log.Printf("schedule tool write upstream failed: %v", errorValue)
		http.Error(responseWriter, "schedule write unavailable", http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", contentTypeOfScheduleToolAnswer(answer))
	responseWriter.WriteHeader(statusCode)
	_, _ = responseWriter.Write(answer)
}

func readOneScheduleToolDocument(responseWriter http.ResponseWriter, request *http.Request, input any) ([]byte, bool) {
	body, errorValue := io.ReadAll(http.MaxBytesReader(responseWriter, request.Body, scheduleToolRequestByteLimit))
	if errorValue != nil {
		http.Error(responseWriter, "invalid schedule request", http.StatusBadRequest)
		return nil, false
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(input); errorValue != nil {
		http.Error(responseWriter, "invalid schedule request", http.StatusBadRequest)
		return nil, false
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		http.Error(responseWriter, "invalid schedule request", http.StatusBadRequest)
		return nil, false
	}
	return body, true
}

func contentTypeOfScheduleToolAnswer(answer []byte) string {
	if json.Valid(answer) {
		return "application/json"
	}
	return "text/plain; charset=utf-8"
}

func (service *Service) cancelUserMemorySchedule(responseWriter http.ResponseWriter, request *http.Request) {
	var cancelRequest memoryScheduleCancelRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&cancelRequest); errorValue != nil {
		log.Printf("memory schedule cancel request decode failed: %v", errorValue)
		http.Error(responseWriter, "invalid memory schedule request", http.StatusBadRequest)
		return
	}
	personID, ok := service.writeMemorySchedulePersonID(responseWriter, request)
	if !ok {
		return
	}
	blueclawRequest := memoryScheduleCancelBlueclawRequest{
		TaskScheduleID:  cancelRequest.TaskScheduleID,
		CreatorPersonID: personID,
	}
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/schedule/cancel", blueclawRequest, nil); errorValue != nil {
		log.Printf("memory schedule cancel upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedule cancel unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) deleteUserMemorySchedule(responseWriter http.ResponseWriter, request *http.Request) {
	var deleteRequest memoryScheduleDeleteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&deleteRequest); errorValue != nil {
		log.Printf("memory schedule delete request decode failed: %v", errorValue)
		http.Error(responseWriter, "invalid memory schedule request", http.StatusBadRequest)
		return
	}
	personID, ok := service.writeMemorySchedulePersonID(responseWriter, request)
	if !ok {
		return
	}
	blueclawRequest := memoryScheduleDeleteBlueclawRequest{
		TaskScheduleID:  deleteRequest.TaskScheduleID,
		CreatorPersonID: personID,
	}
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/schedule/delete", blueclawRequest, nil); errorValue != nil {
		log.Printf("memory schedule delete upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedule delete unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) updateUserMemorySchedule(responseWriter http.ResponseWriter, request *http.Request) {
	var updateRequest memoryScheduleUpdateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&updateRequest); errorValue != nil {
		log.Printf("memory schedule update request decode failed: %v", errorValue)
		http.Error(responseWriter, "invalid memory schedule request", http.StatusBadRequest)
		return
	}
	personID, ok := service.writeMemorySchedulePersonID(responseWriter, request)
	if !ok {
		return
	}
	blueclawRequest := memoryScheduleUpdateBlueclawRequest{
		TaskScheduleID:  updateRequest.TaskScheduleID,
		CreatorPersonID: personID,
		Name:            updateRequest.Name,
		Kind:            updateRequest.Kind,
		RunAt:           updateRequest.RunAt,
		IntervalSecond:  updateRequest.IntervalSecond,
		CronExpression:  updateRequest.CronExpression,
		TimeZone:        updateRequest.TimeZone,
		ExpiresAt:       updateRequest.ExpiresAt,
		MaxRunCount:     updateRequest.MaxRunCount,
		RepeatPolicy:    updateRequest.RepeatPolicy,
	}
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodPost, "/admin/api/schedule/update", blueclawRequest, nil); errorValue != nil {
		log.Printf("memory schedule update upstream failed: %v", errorValue)
		http.Error(responseWriter, "memory schedule update unavailable", http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, map[string]bool{"ok": true})
}

func (service *Service) writeMemorySchedulePersonID(responseWriter http.ResponseWriter, request *http.Request) (string, bool) {
	actorEmail := service.memoryActorEmail(request)
	if actorEmail == "" {
		http.Error(responseWriter, "memory access required", http.StatusForbidden)
		return "", false
	}
	personID, errorValue := service.resolveMemoryPersonID(request.Context(), actorEmail)
	if errorValue != nil {
		log.Printf("memory schedules identity resolution failed: %v", errorValue)
		http.Error(responseWriter, "memory identity unavailable", http.StatusBadGateway)
		return "", false
	}
	if personID == "" {
		http.Error(responseWriter, "memory person not found", http.StatusNotFound)
		return "", false
	}
	return personID, true
}

func memorySchedulesQuery(values url.Values, personID string) string {
	query := url.Values{}
	query.Set("creatorPersonID", personID)
	query.Set("includeExpired", memorySchedulesIncludeExpired(values.Get("includeExpired")))
	query.Set("page", strconv.Itoa(memorySchedulesPage(values.Get("page"))))
	query.Set("pageSize", strconv.Itoa(memorySchedulesPageSize(values.Get("pageSize"))))
	return query.Encode()
}

func memorySchedulesIncludeExpired(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "false") {
		return "false"
	}
	return "true"
}

func memorySchedulesPage(value string) int {
	page, errorValue := strconv.Atoi(value)
	if errorValue != nil || page < 1 {
		return memorySchedulesDefaultPage
	}
	return page
}

func memorySchedulesPageSize(value string) int {
	pageSize, errorValue := strconv.Atoi(value)
	if errorValue != nil || pageSize < 1 {
		return memorySchedulesDefaultPageSize
	}
	if pageSize > memorySchedulesMaxPageSize {
		return memorySchedulesMaxPageSize
	}
	return pageSize
}
