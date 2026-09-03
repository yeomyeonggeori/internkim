package admind

import (
	"encoding/json"
	"errors"
	"net/http"
)

const calendarEventVersionConflictErrorCode = "calendar_event_version_conflict"

var errCalendarEventVersionConflict = errors.New("calendar event version conflict")

func writeCalendarEventVersionConflictError(responseWriter http.ResponseWriter, errorValue error) bool {
	if !errors.Is(errorValue, errCalendarEventVersionConflict) {
		return false
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(responseWriter).Encode(calendarMutationErrorResponse{Code: calendarEventVersionConflictErrorCode})
	return true
}
