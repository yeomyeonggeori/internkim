package admind

import (
	"encoding/json"
	"net/http"
)

func (service *Service) writeAttendanceSettings(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.isAuthorized(request) {
		http.Error(responseWriter, "admin required", http.StatusForbidden)
		return
	}
	var body attendanceSettingsRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&body); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if body.TeamViewVisibleToAll != nil {
		if errorValue := service.writeAttendanceTeamViewVisibleToAll(request.Context(), *body.TeamViewVisibleToAll); errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
	}
	teamVisible, errorValue := service.readAttendanceTeamViewVisibleToAll(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, map[string]any{
		"teamViewVisibleToAll": teamVisible,
	})
}
