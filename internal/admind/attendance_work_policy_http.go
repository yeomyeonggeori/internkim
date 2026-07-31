package admind

import (
	"encoding/json"
	"io"
	"net/http"
	"time"
)

func (service *Service) handleAttendanceWorkPolicy(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	if request.Method == http.MethodGet {
		policy, errorValue := service.readAttendanceWorkPolicy(request.Context())
		if errorValue != nil {
			http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
			return
		}
		service.writeJSON(responseWriter, policy)
		return
	}
	var revision attendanceWorkPolicyRevision
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&revision); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	if errorValue := decoder.Decode(&struct{}{}); errorValue != io.EOF {
		http.Error(responseWriter, "request must contain one JSON object", http.StatusBadRequest)
		return
	}
	now := time.Now()
	effectiveDate := now.In(service.workspaceTimeZone().location).Format(time.DateOnly)
	policy, errorValue := service.saveAttendanceWorkPolicyRevision(
		request.Context(),
		revision,
		effectiveDate,
		now,
	)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	service.writeJSON(responseWriter, policy)
}
