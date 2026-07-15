package admind

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

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
	status := firstNonEmpty(cleanFlowStatus(payload.Status), defaultFlowStatus())
	statusRank := 0
	statusRankProvided := payload.StatusRank != nil
	if payload.StatusRank != nil {
		if *payload.StatusRank < 0 {
			return flowTask{}, flowValidationError("statusRank must be zero or greater")
		}
		statusRank = *payload.StatusRank
	}
	callerEmail := strings.ToLower(strings.TrimSpace(service.flowActorEmail(request)))
	if taskID == "" && callerEmail != "" && !service.isFlowAdminEmail(request.Context(), callerEmail) && !strings.EqualFold(owner.Email, callerEmail) {
		status = flowStatusRequested
		payload.RequestReason = firstNonEmpty(strings.TrimSpace(payload.RequestReason), "타인 업무 추가 요청")
	}
	if !isAllowedFlowStatus(status) {
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
	now := flowDateNow()
	if errorValue := validateFlowTaskWeekCode(payload.WeekCode, now); errorValue != nil {
		return flowTask{}, errorValue
	}
	if errorValue := validateFlowTaskDateInput(payload.StartDate, payload.EndDate, now.Location()); errorValue != nil {
		return flowTask{}, errorValue
	}
	dates := normalizeFlowTaskDates(payload, status, now)
	id := strings.TrimSpace(taskID)
	if id == "" {
		id = stableFlowID(dates.WeekCode + owner.ID + content + time.Now().UTC().Format(time.RFC3339Nano))
	}
	return flowTask{
		ID:                 id,
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
	}, nil
}
