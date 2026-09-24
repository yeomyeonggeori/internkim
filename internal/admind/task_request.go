package admind

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (service *Service) taskFromRequest(request *http.Request, members []taskMember, definitions taskDefinitions, taskID string) (Task, error) {
	task, _, errorValue := service.taskAndPayloadFromRequest(request, members, definitions, taskID)
	return task, errorValue
}

func (service *Service) taskAndPayloadFromRequest(request *http.Request, members []taskMember, definitions taskDefinitions, taskID string) (Task, taskWriteRequest, error) {
	var payload taskWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		return Task{}, payload, errorValue
	}
	memberByID := map[string]taskMember{}
	for _, member := range members {
		memberByID[member.ID] = member
	}
	owner, found := memberByID[strings.TrimSpace(payload.OwnerID)]
	if !found {
		return Task{}, payload, taskValidationError("ownerID is not a known member")
	}
	participantIDs := uniqueNonEmpty(payload.ParticipantIDs)
	if len(participantIDs) == 0 {
		participantIDs = []string{owner.ID}
	}
	if !containsString(participantIDs, owner.ID) {
		participantIDs = append([]string{owner.ID}, participantIDs...)
	}
	participants := make([]taskMember, 0, len(participantIDs))
	for _, memberID := range participantIDs {
		member, found := memberByID[memberID]
		if !found {
			return Task{}, payload, taskValidationError("participantID is not a known member")
		}
		participants = append(participants, member)
	}
	content := strings.TrimSpace(payload.Content)
	if content == "" {
		return Task{}, payload, taskValidationError("content is required")
	}
	status := firstNonEmpty(cleanTaskStatus(payload.Status), defaultTaskStatus())
	if !isAllowedTaskStatus(status) {
		return Task{}, payload, taskValidationError("status is not allowed")
	}
	category := strings.TrimSpace(firstNonEmpty(payload.Category, payload.Business))
	if category != "" && len(definitions.Categories) > 0 && !containsString(definitions.Categories, category) {
		return Task{}, payload, taskValidationError("category is not allowed")
	}
	taskType := strings.TrimSpace(payload.Type)
	if taskType != "" && !containsString(definitions.Types, taskType) {
		return Task{}, payload, taskValidationError("type is not allowed")
	}
	size := strings.ToUpper(strings.TrimSpace(payload.Size))
	if size != "" && !containsTaskSize(definitions.Sizes, size) {
		return Task{}, payload, taskValidationError("size is not allowed")
	}
	now := service.companyDateNow(request.Context())
	canonicalWeekCode, errorValue := canonicalTaskWeekCode(payload.WeekCode, now)
	if errorValue != nil {
		return Task{}, payload, errorValue
	}
	payload.WeekCode = canonicalWeekCode
	if errorValue := validateTaskDateInput(payload.StartDate, payload.EndDate, now.Location()); errorValue != nil {
		return Task{}, payload, errorValue
	}
	dates := normalizeTaskDates(payload, status, now)
	return Task{
		ID:               strings.TrimSpace(taskID),
		OwnerID:          owner.ID,
		OwnerName:        owner.Name,
		ParticipantIDs:   memberIDs(participants),
		ParticipantNames: memberNames(participants),
		Business:         category,
		Type:             taskType,
		Content:          content,
		Size:             size,
		Status:           status,
		StartDate:        dates.StartDate,
		EndDate:          dates.EndDate,
		WeekCode:         dates.WeekCode,
	}, payload, nil
}
