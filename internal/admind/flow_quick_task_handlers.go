package admind

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

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
		writeRequest.Status = flowStatusRequested
		writeRequest.RequestReason = firstNonEmpty(writeRequest.RequestReason, prompt)
	}
	body, _ := json.Marshal(writeRequest)
	clonedRequest := request.Clone(request.Context())
	clonedRequest.Body = io.NopCloser(bytes.NewReader(body))
	task, errorValue := service.flowTaskFromRequest(clonedRequest, members, definitions, "")
	if errorValue != nil {
		writeFlowRequestError(responseWriter, errorValue)
		return
	}
	task.Business = firstNonEmpty(task.Business, defaultFlowTaskBusiness(definitions))
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
	task = flowTaskWithCreatedAt(task)
	if errorValue := service.writeFlowTask(request.Context(), task); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	task = service.applyFlowMattermostProjection(request.Context(), task)
	service.writeJSON(responseWriter, task)
}

func (service *Service) writeQuickFlowTaskDuplicate(responseWriter http.ResponseWriter, task flowTask, reason string) {
	service.writeJSON(responseWriter, map[string]any{
		"status":        "skipped_duplicate",
		"message":       "이미 추가된 업무라 건너뛰었습니다. 그래도 추가하려면 다시 추가하라고 확인해 주세요.",
		"duplicateTask": task,
		"reason":        strings.TrimSpace(reason),
	})
}
