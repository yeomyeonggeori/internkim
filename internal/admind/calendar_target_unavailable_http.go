package admind

import (
	"encoding/json"
	"errors"
	"net/http"
)

const calendarTargetUnavailableErrorCode = "calendar_target_unavailable"

type calendarMutationErrorResponse struct {
	Code string `json:"code"`
}

func writeCalendarTargetUnavailableError(responseWriter http.ResponseWriter, errorValue error) bool {
	if !errors.Is(errorValue, errCalendarTargetUnavailable) {
		return false
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(responseWriter).Encode(calendarMutationErrorResponse{Code: calendarTargetUnavailableErrorCode})
	return true
}
