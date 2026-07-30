package admind

import (
	"encoding/json"
	"errors"
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
	service.attendanceLeavePolicyMutationMutex.Lock()
	defer service.attendanceLeavePolicyMutationMutex.Unlock()
	normalizeLegacyAttendanceLeavePolicy(&policy)
	existing, errorValue := service.readAttendanceLeavePolicy(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue = service.preserveUsedRemovedAttendanceLeaveTypes(
		request.Context(),
		existing,
		&policy,
	); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue = validateAttendanceLeavePolicy(&policy, &existing); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadRequest)
		return
	}
	now := time.Now()
	setAttendanceLeavePolicyTimestamp(&policy, now)
	if errorValue = service.synchronizeAttendanceLeavePolicyAccrualsBeforeUpdate(
		request,
		existing,
		policy,
		now,
	); errorValue != nil {
		status := http.StatusInternalServerError
		if errors.Is(errorValue, errAttendanceLeavePolicyAdjustmentConflict) {
			status = http.StatusBadRequest
		}
		http.Error(responseWriter, errorValue.Error(), status)
		return
	}
	if errorValue = service.synchronizeAttendanceLeaveBalanceTrackingBeforeUpdate(
		request.Context(),
		existing,
		policy,
	); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	if errorValue = service.writeAttendanceLeavePolicy(request.Context(), policy); errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusInternalServerError)
		return
	}
	service.writeJSON(responseWriter, policy)
}
