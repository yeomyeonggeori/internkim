package admind

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (service *Service) createQuickTask(responseWriter http.ResponseWriter, request *http.Request) {
	members := service.taskMembers(request)
	definitions, errorValue := service.readTaskDefinitions(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	var payload taskQuickTaskRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		writeTaskRequestError(responseWriter, errorValue)
		return
	}
	prompt := strings.TrimSpace(payload.Prompt)
	if prompt == "" {
		writeTaskRequestError(responseWriter, taskValidationError("prompt is required"))
		return
	}
	requesterEmail := service.taskRequesterEmail(request, payload)
	owner, errorValue := taskOwnerFromQuickRequest(payload, members, requesterEmail)
	if errorValue != nil {
		writeTaskRequestError(responseWriter, errorValue)
		return
	}
	inferredTask, errorValue := service.inferTask(request.Context(), prompt, payload.WeekCode, owner, members, definitions)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	owner = quickTaskOwner(inferredTask.ParticipantIDs, members, owner)
	inferredTask.Status = statusCompletedWhenEnded(inferredTask.Status, preferExplicitTaskValue(payload.EndDate, inferredTask.EndDate), taskDateNow().Format("2006-01-02"))
	writeRequest := taskWriteRequest{
		OwnerID:        owner.ID,
		ParticipantIDs: firstNonEmptySlice(inferredTask.ParticipantIDs, payload.ParticipantIDs, []string{owner.ID}),
		Category:       inferredTask.Category,
		Type:           inferredTask.Type,
		Content:        preferExplicitTaskValue(payload.Title, inferredTask.Content),
		Goal:           inferredTask.Goal,
		Size:           inferredTask.Size,
		Status:         inferredTask.Status,
		StartDate:      inferredTask.StartDate,
		EndDate:        preferExplicitTaskValue(payload.EndDate, inferredTask.EndDate),
		WeekCode:       payload.WeekCode,
	}
	if shouldForceQuickTaskRequest(owner, requesterEmail) {
		writeRequest.Status = taskStatusRequested
	}
	body, _ := json.Marshal(writeRequest)
	clonedRequest := request.Clone(request.Context())
	clonedRequest.Body = io.NopCloser(bytes.NewReader(body))
	task, errorValue := service.taskFromRequest(clonedRequest, members, definitions, "")
	if errorValue != nil {
		writeTaskRequestError(responseWriter, errorValue)
		return
	}
	task.Business = firstNonEmpty(task.Business, defaultTaskBusiness(definitions))
	if !payload.AllowDuplicate {
		duplicateTask, reason, found, errorValue := service.findQuickTaskDuplicate(request.Context(), task, members)
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
			return
		}
		if found {
			service.writeQuickTaskDuplicate(responseWriter, duplicateTask, reason)
			return
		}
	}
	saved, answered, saveError := service.saveCentralTask(request, task, service.taskPeopleByID(request.Context()))
	if !answered {
		http.Error(responseWriter, errTaskWriterUnnamed.Error(), http.StatusFailedDependency)
		return
	}
	if saveError != nil {
		http.Error(responseWriter, saveError.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, saved)
}

func (service *Service) writeQuickTaskDuplicate(responseWriter http.ResponseWriter, task Task, reason string) {
	service.writeJSON(responseWriter, map[string]any{
		"status":        "skipped_duplicate",
		"message":       "이미 추가된 업무라 건너뛰었습니다. 그래도 추가하려면 다시 추가하라고 확인해 주세요.",
		"duplicateTask": task,
		"reason":        strings.TrimSpace(reason),
	})
}
