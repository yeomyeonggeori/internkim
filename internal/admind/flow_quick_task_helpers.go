package admind

import (
	"net/http"
	"strings"
)

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
	return strings.ToLower(strings.TrimSpace(service.authenticatedCallerEmail(request)))
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

func preferExplicitFlowTaskValue(explicitValue string, inferredValue string) string {
	return firstNonEmpty(strings.TrimSpace(explicitValue), inferredValue)
}

func memberIDForEmail(members []flowMember, email string) string {
	for _, member := range members {
		if strings.EqualFold(member.Email, strings.TrimSpace(email)) {
			return member.ID
		}
	}
	return ""
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
