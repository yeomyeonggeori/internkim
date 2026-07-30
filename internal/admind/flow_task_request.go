package admind

import (
	"encoding/json"
	"net/http"
	"strings"
)

func (service *Service) flowTaskFromRequest(request *http.Request, members []flowMember, definitions flowDefinitions, taskID string) (flowTask, error) {
	task, _, errorValue := service.flowTaskAndPayloadFromRequest(request, members, definitions, taskID)
	return task, errorValue
}

func (service *Service) flowTaskAndPayloadFromRequest(request *http.Request, members []flowMember, definitions flowDefinitions, taskID string) (flowTask, flowTaskWriteRequest, error) {
	var payload flowTaskWriteRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		return flowTask{}, payload, errorValue
	}
	memberByID := map[string]flowMember{}
	for _, member := range members {
		memberByID[member.ID] = member
	}
	owner, found := memberByID[strings.TrimSpace(payload.OwnerID)]
	if !found {
		return flowTask{}, payload, flowValidationError("ownerID is not a known member")
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
			return flowTask{}, payload, flowValidationError("participantID is not a known member")
		}
		participants = append(participants, member)
	}
	content := strings.TrimSpace(payload.Content)
	if content == "" {
		return flowTask{}, payload, flowValidationError("content is required")
	}
	status := firstNonEmpty(cleanFlowStatus(payload.Status), defaultFlowStatus())
	statusRank := 0
	statusRankProvided := payload.StatusRank != nil
	if payload.StatusRank != nil {
		if *payload.StatusRank < 0 {
			return flowTask{}, payload, flowValidationError("statusRank must be zero or greater")
		}
		statusRank = *payload.StatusRank
	}
	callerEmail := strings.ToLower(strings.TrimSpace(service.flowActorEmail(request)))
	if taskID == "" && callerEmail != "" && !service.isFlowAdminEmail(request.Context(), callerEmail) && !strings.EqualFold(owner.Email, callerEmail) {
		status = flowStatusRequested
		payload.RequestReason = firstNonEmpty(strings.TrimSpace(payload.RequestReason), "타인 업무 추가 요청")
	}
	if !isAllowedFlowStatus(status) {
		return flowTask{}, payload, flowValidationError("status is not allowed")
	}
	category := strings.TrimSpace(firstNonEmpty(payload.Category, payload.Business))
	if category != "" && len(definitions.Categories) > 0 && !containsString(definitions.Categories, category) {
		return flowTask{}, payload, flowValidationError("category is not allowed")
	}
	taskType := firstNonEmpty(strings.TrimSpace(payload.Type), "기타")
	if !containsString(definitions.Types, taskType) {
		return flowTask{}, payload, flowValidationError("type is not allowed")
	}
	size := firstNonEmpty(strings.ToUpper(strings.TrimSpace(payload.Size)), "M")
	if !containsFlowSize(definitions.Sizes, size) {
		return flowTask{}, payload, flowValidationError("size is not allowed")
	}
	now := flowDateNow()
	canonicalWeekCode, errorValue := canonicalFlowTaskWeekCode(payload.WeekCode, now)
	if errorValue != nil {
		return flowTask{}, payload, errorValue
	}
	payload.WeekCode = canonicalWeekCode
	if errorValue := validateFlowTaskDateInput(payload.StartDate, payload.EndDate, now.Location()); errorValue != nil {
		return flowTask{}, payload, errorValue
	}
	dates := normalizeFlowTaskDates(payload, status, now)
	return flowTask{
		ID:                 strings.TrimSpace(taskID),
		OwnerID:            owner.ID,
		OwnerName:          owner.Name,
		ParticipantIDs:     memberIDs(participants),
		ParticipantNames:   memberNames(participants),
		Business:           category,
		Type:               taskType,
		Content:            content,
		Goal:               strings.TrimSpace(payload.Goal),
		Size:               size,
		Status:             status,
		StatusRank:         statusRank,
		StatusRankProvided: statusRankProvided,
		StartDate:          dates.StartDate,
		EndDate:            dates.EndDate,
		WeekCode:           dates.WeekCode,
		Flag:               payload.Flag,
		RequestReason:      strings.TrimSpace(payload.RequestReason),
		DecisionReason:     strings.TrimSpace(payload.DecisionReason),
	}, payload, nil
}
