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
	decidedLabels := service.decideTaskLabelsAside(request.Context(), taskLabelDraft{Title: prompt}, definitions)
	inferredTask, errorValue := service.inferTask(request.Context(), prompt, payload.WeekCode, owner, members, definitions)
	labels := <-decidedLabels
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	owner = quickTaskOwner(inferredTask.ParticipantIDs, members, owner)
	inferredTask.Status = statusCompletedWhenEnded(inferredTask.Status, preferExplicitTaskValue(payload.EndDate, inferredTask.EndDate), service.companyDateNow(request.Context()).Format("2006-01-02"))
	writeRequest := taskWriteRequest{
		OwnerID:        owner.ID,
		ParticipantIDs: firstNonEmptySlice(inferredTask.ParticipantIDs, payload.ParticipantIDs, []string{owner.ID}),
		Category:       labels.Business,
		Type:           labels.Type,
		Content:        preferExplicitTaskValue(payload.Title, inferredTask.Content),
		Goal:           inferredTask.Goal,
		Size:           labels.Size,
		Status:         inferredTask.Status,
		StartDate:      inferredTask.StartDate,
		EndDate:        preferExplicitTaskValue(payload.EndDate, inferredTask.EndDate),
		WeekCode:       payload.WeekCode,
	}
	body, _ := json.Marshal(writeRequest)
	clonedRequest := request.Clone(request.Context())
	clonedRequest.Body = io.NopCloser(bytes.NewReader(body))
	task, errorValue := service.taskFromRequest(clonedRequest, members, definitions, "")
	if errorValue != nil {
		writeTaskRequestError(responseWriter, errorValue)
		return
	}
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
	answer, answered, addError := service.addTaskThroughTheRecord(request, task, inferredTask.Goal, service.taskPeopleByID(request.Context()))
	if !answered {
		http.Error(responseWriter, errTaskWriterUnnamed.Error(), http.StatusFailedDependency)
		return
	}
	if addError != nil {
		http.Error(responseWriter, addError.Error(), http.StatusBadGateway)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(answer.Status)
	responseWriter.Write(answer.Body)
}

func (service *Service) writeQuickTaskDuplicate(responseWriter http.ResponseWriter, task Task, reason string) {
	service.writeJSON(responseWriter, map[string]any{
		"status":        "skipped_duplicate",
		"message":       "이미 추가된 업무라 건너뛰었습니다. 그래도 추가하려면 다시 추가하라고 확인해 주세요.",
		"duplicateTask": task,
		"reason":        strings.TrimSpace(reason),
	})
}
