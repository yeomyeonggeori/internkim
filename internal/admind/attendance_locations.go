package admind

import (
	"encoding/json"
	"net/http"
)

func (service *Service) writeAttendanceLocations(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.canManageAttendance(request) {
		http.Error(responseWriter, "admin access required", http.StatusForbidden)
		return
	}
	locations, errorValue := service.readAttendanceLocations()
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, attendanceLocationsResponse{Locations: locations})
}

func (service *Service) updateAttendanceLocations(responseWriter http.ResponseWriter, request *http.Request) {
	if !service.canManageAttendance(request) {
		http.Error(responseWriter, "admin access required", http.StatusForbidden)
		return
	}
	var payload attendanceLocationsResponse
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	locations, errorValue := normalizeAttendanceLocations(payload.Locations)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := service.writeAttendanceLocationsFile(locations); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.syncMattermostAttendanceChannelAfterLocationUpdate(request)
	service.writeJSON(responseWriter, attendanceLocationsResponse{Locations: locations})
}
