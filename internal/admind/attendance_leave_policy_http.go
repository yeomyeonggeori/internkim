package admind

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func (service *Service) handleAttendanceLeavePolicy(responseWriter http.ResponseWriter, request *http.Request) {
	if request.Method == http.MethodGet {
		policy, errorValue := service.readAttendanceLeavePolicy(request.Context())
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		service.writeJSON(responseWriter, policy)
		return
	}
	var policy attendanceLeavePolicy
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&policy); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		http.Error(responseWriter, "request must contain one JSON object", http.StatusBadRequest)
		return
	}
	existing, errorValue := service.readAttendanceLeavePolicy(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue = validateAttendanceLeavePolicy(&policy, &existing); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now()
	if errorValue = service.synchronizeAttendanceLeavePolicyAccrualsBeforeUpdate(
		request,
		existing,
		now,
	); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	setAttendanceLeavePolicyTimestamp(&policy, now)
	if errorValue = service.writeAttendanceLeavePolicy(request.Context(), policy); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, policy)
}
