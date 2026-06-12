package admind

import (
	"net/http"
	"strings"
)

func (service *Service) canDeleteFlowTask(request *http.Request, task flowTask) bool {
	actorEmail := service.flowActorEmail(request)
	if service.isFlowAdminEmail(request.Context(), actorEmail) {
		return true
	}
	for _, member := range service.flowMembers(request) {
		if member.ID == task.OwnerID && strings.EqualFold(member.Email, actorEmail) {
			return true
		}
	}
	return false
}

func (service *Service) canUpdateFlowTask(request *http.Request, task flowTask) bool {
	actorEmail := service.flowActorEmail(request)
	if service.isFlowAdminEmail(request.Context(), actorEmail) {
		return true
	}
	for _, member := range service.flowMembers(request) {
		if !strings.EqualFold(member.Email, actorEmail) {
			continue
		}
		if member.ID == task.OwnerID || containsString(task.ParticipantIDs, member.ID) {
			return true
		}
	}
	return false
}

func (service *Service) canManageFlowTaskAssignment(request *http.Request, task flowTask) bool {
	actorEmail := service.flowActorEmail(request)
	if service.isFlowAdminEmail(request.Context(), actorEmail) {
		return true
	}
	for _, member := range service.flowMembers(request) {
		if member.ID == task.OwnerID && strings.EqualFold(member.Email, actorEmail) {
			return true
		}
	}
	return false
}

func flowTaskAssignmentChanged(existingTask flowTask, nextTask flowTask) bool {
	if existingTask.OwnerID != nextTask.OwnerID {
		return true
	}
	return !sameFlowStringSet(existingTask.ParticipantIDs, nextTask.ParticipantIDs)
}

func sameFlowStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := map[string]int{}
	for _, value := range left {
		seen[value]++
	}
	for _, value := range right {
		seen[value]--
		if seen[value] < 0 {
			return false
		}
	}
	return true
}
