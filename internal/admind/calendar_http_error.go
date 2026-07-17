package admind

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

const (
	calendarMutationInvalidRequestErrorCode = "calendar_mutation_invalid_request"
	calendarInternalErrorCode               = "calendar_internal_error"
)

func writeCalendarErrorCode(responseWriter http.ResponseWriter, statusCode int, errorCode string) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(statusCode)
	_ = json.NewEncoder(responseWriter).Encode(calendarMutationErrorResponse{Code: errorCode})
}

func writeCalendarMutationInternalError(responseWriter http.ResponseWriter, request *http.Request, eventID string, errorValue error) {
	slog.ErrorContext(
		request.Context(),
		"calendar mutation request failed",
		"method", request.Method,
		"path", request.URL.Path,
		"event_id", strings.TrimSpace(eventID),
		"error", errorValue,
	)
	writeCalendarErrorCode(responseWriter, http.StatusInternalServerError, calendarInternalErrorCode)
}
