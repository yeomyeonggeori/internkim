package admind

import (
	"net/http"
)

const skillInventoryPath = "/skills/api"

func (service *Service) handleSkillInventory(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		http.NotFound(responseWriter, request)
		return
	}
	actorEmail := service.actorEmailAllowingAssertedRequester(request)
	if actorEmail == "" {
		http.Error(responseWriter, "authentication required", http.StatusUnauthorized)
		return
	}
	if !service.canManageTaskRuns(request.Context(), actorEmail) {
		http.Error(responseWriter, "the skill inventory is an administrator's view", http.StatusForbidden)
		return
	}
	var inventory map[string]any
	if errorValue := service.blueclawJSONRequest(request.Context(), http.MethodGet, "/admin/api/skills", nil, &inventory); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, inventory)
}
