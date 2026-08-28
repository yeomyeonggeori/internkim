package admind

import (
	"net/http"
	"strings"
)

func taskOwnerFromQuickRequest(payload taskQuickTaskRequest, members []taskMember, callerEmail string) (taskMember, error) {
	memberByID := map[string]taskMember{}
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
	return taskMember{}, taskValidationError("ownerID is not a known member")
}

func (service *Service) taskRequesterEmail(request *http.Request, payload taskQuickTaskRequest) string {
	if actorEmail := service.taskActorEmail(request); actorEmail != "" {
		return actorEmail
	}
	if isLocalRequest(request) && strings.TrimSpace(payload.RequesterEmail) != "" {
		return strings.ToLower(strings.TrimSpace(payload.RequesterEmail))
	}
	return strings.ToLower(strings.TrimSpace(service.authenticatedCallerEmail(request)))
}

func shouldForceQuickTaskRequest(owner taskMember, requesterEmail string) bool {
	if strings.TrimSpace(requesterEmail) == "" {
		return false
	}
	if strings.EqualFold(owner.Email, requesterEmail) {
		return false
	}
	return true
}

func preferExplicitTaskValue(explicitValue string, inferredValue string) string {
	return firstNonEmpty(strings.TrimSpace(explicitValue), inferredValue)
}

// Work whose end date has already passed is finished work, whatever the note's
// tense suggested; statuses that say why the work stopped are left alone.
func statusCompletedWhenEnded(status string, endDate string, today string) string {
	if !isTaskPlannedStatus(status) && !isTaskInProgressStatus(status) {
		return status
	}
	trimmedEndDate := strings.TrimSpace(endDate)
	if trimmedEndDate == "" || trimmedEndDate > today {
		return status
	}
	return taskStatusCompleted
}

func memberIDForEmail(members []taskMember, email string) string {
	for _, member := range members {
		if strings.EqualFold(member.Email, strings.TrimSpace(email)) {
			return member.ID
		}
	}
	return ""
}

func cleanParticipantIDs(values []string, members []taskMember) []string {
	allowed := map[string]bool{}
	for _, member := range members {
		allowed[member.ID] = true
	}
	result := []string{}
	seen := map[string]bool{}
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

// A note that names nobody is the requester's own task; one that includes the
// requester is shared work they keep; one that names only other people hands
// the work to the first person named, and the requester-versus-owner gate then
// turns it into a request.
func quickTaskOwner(participantIDs []string, members []taskMember, requester taskMember) taskMember {
	if len(participantIDs) == 0 || containsString(participantIDs, requester.ID) {
		return requester
	}
	for _, member := range members {
		if member.ID == participantIDs[0] {
			return member
		}
	}
	return requester
}
