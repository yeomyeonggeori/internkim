package admind

import (
	"net/http"
	"strings"
)

func (service *Service) canDeleteTask(request *http.Request, task Task) bool {
	actorEmail := service.taskActorEmail(request)
	if service.isTaskAdminEmail(request.Context(), actorEmail) {
		return true
	}
	for _, member := range service.taskMembers(request) {
		if member.ID == task.OwnerID && strings.EqualFold(member.Email, actorEmail) {
			return true
		}
	}
	return false
}

func (service *Service) canUpdateTask(request *http.Request, task Task) bool {
	actorEmail := service.taskActorEmail(request)
	if service.isTaskAdminEmail(request.Context(), actorEmail) {
		return true
	}
	for _, member := range service.taskMembers(request) {
		if !strings.EqualFold(member.Email, actorEmail) {
			continue
		}
		if member.ID == task.OwnerID || containsString(task.ParticipantIDs, member.ID) {
			return true
		}
	}
	return false
}

func (service *Service) canManageTaskAssignment(request *http.Request, task Task) bool {
	actorEmail := service.taskActorEmail(request)
	if service.isTaskAdminEmail(request.Context(), actorEmail) {
		return true
	}
	for _, member := range service.taskMembers(request) {
		if member.ID == task.OwnerID && strings.EqualFold(member.Email, actorEmail) {
			return true
		}
	}
	return false
}

func taskAssignmentChanged(existingTask Task, nextTask Task) bool {
	if existingTask.OwnerID != nextTask.OwnerID {
		return true
	}
	return !sameTaskStringSet(existingTask.ParticipantIDs, nextTask.ParticipantIDs)
}

func sameTaskStringSet(left []string, right []string) bool {
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
