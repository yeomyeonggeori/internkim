package admind

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	calendarDeleteIntentConflictErrorCode = "calendar_delete_intent_conflict"
	calendarDeleteIntentNotFoundErrorCode = "calendar_delete_intent_not_found"
)

type calendarDeleteIntentCreateRequest struct {
	ClientID          string `json:"clientID"`
	Sequence          int64  `json:"sequence"`
	ExpectedUpdatedAt string `json:"expectedUpdatedAt"`
}

type calendarDeleteIntentCancelRequest struct {
	ClientID string `json:"clientID"`
	Sequence int64  `json:"sequence"`
}

type calendarDeleteIntentResponse struct {
	OperationID string `json:"operationID"`
	ExecuteAt   string `json:"executeAt"`
}

func (service *Service) handleCalendarDeleteIntent(responseWriter http.ResponseWriter, request *http.Request, eventID string, operationID string) {
	switch request.Method {
	case http.MethodPut:
		service.createCalendarDeleteIntentRequest(responseWriter, request, eventID, operationID)
	case http.MethodDelete:
		service.cancelCalendarDeleteIntentRequest(responseWriter, request, eventID, operationID)
	default:
		http.NotFound(responseWriter, request)
	}
}

func (service *Service) createCalendarDeleteIntentRequest(responseWriter http.ResponseWriter, request *http.Request, eventID string, operationID string) {
	var payload calendarDeleteIntentCreateRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		writeCalendarErrorCode(responseWriter, http.StatusBadRequest, calendarMutationInvalidRequestErrorCode)
		return
	}
	expectedUpdatedAt, errorValue := validateCalendarDeleteIntentCreateRequest(payload)
	if errorValue != nil {
		writeCalendarErrorCode(responseWriter, http.StatusBadRequest, calendarMutationInvalidRequestErrorCode)
		return
	}
	intent, errorValue := service.createCalendarDeleteIntent(request.Context(), eventID, operationID, calendarDeleteIntentCreate{
		ClientID:          payload.ClientID,
		Sequence:          payload.Sequence,
		ExpectedUpdatedAt: expectedUpdatedAt,
	}, time.Now().UTC())
	if errorValue != nil {
		writeCalendarDeleteIntentError(responseWriter, request, eventID, operationID, errorValue)
		return
	}
	if intent.Status == calendarDeleteIntentStatusCanceled || intent.Status == calendarDeleteIntentStatusConflicted {
		writeCalendarErrorCode(responseWriter, http.StatusConflict, calendarDeleteIntentConflictErrorCode)
		return
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(responseWriter).Encode(calendarDeleteIntentResponse{OperationID: intent.OperationID, ExecuteAt: intent.ExecuteAt})
}

func (service *Service) cancelCalendarDeleteIntentRequest(responseWriter http.ResponseWriter, request *http.Request, eventID string, operationID string) {
	var payload calendarDeleteIntentCancelRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		writeCalendarErrorCode(responseWriter, http.StatusBadRequest, calendarMutationInvalidRequestErrorCode)
		return
	}
	if strings.TrimSpace(payload.ClientID) == "" {
		writeCalendarErrorCode(responseWriter, http.StatusBadRequest, calendarMutationInvalidRequestErrorCode)
		return
	}
	if payload.Sequence <= 0 {
		writeCalendarErrorCode(responseWriter, http.StatusBadRequest, calendarMutationInvalidRequestErrorCode)
		return
	}
	if errorValue := service.cancelCalendarDeleteIntent(request.Context(), eventID, operationID, payload.ClientID, payload.Sequence, time.Now().UTC()); errorValue != nil {
		writeCalendarDeleteIntentError(responseWriter, request, eventID, operationID, errorValue)
		return
	}
	responseWriter.WriteHeader(http.StatusNoContent)
}

func validateCalendarDeleteIntentCreateRequest(payload calendarDeleteIntentCreateRequest) (string, error) {
	if strings.TrimSpace(payload.ClientID) == "" {
		return "", errors.New("clientID is required")
	}
	if payload.Sequence <= 0 {
		return "", errors.New("sequence must be a positive integer")
	}
	return normalizeExpectedCalendarEventUpdatedAt(payload.ExpectedUpdatedAt)
}

func writeCalendarDeleteIntentError(responseWriter http.ResponseWriter, request *http.Request, eventID string, operationID string, errorValue error) {
	switch {
	case errors.Is(errorValue, sql.ErrNoRows):
		writeCalendarErrorCode(responseWriter, http.StatusNotFound, calendarDeleteIntentNotFoundErrorCode)
	case errors.Is(errorValue, errCalendarEventVersionConflict):
		writeCalendarErrorCode(responseWriter, http.StatusConflict, calendarEventVersionConflictErrorCode)
	case errors.Is(errorValue, errCalendarDeleteIntentPayloadMismatch), errors.Is(errorValue, errCalendarDeleteIntentNotPending):
		writeCalendarErrorCode(responseWriter, http.StatusConflict, calendarDeleteIntentConflictErrorCode)
	default:
		slog.ErrorContext(
			request.Context(),
			"calendar delete intent request failed",
			"method", request.Method,
			"path", request.URL.Path,
			"event_id", strings.TrimSpace(eventID),
			"operation_id", strings.TrimSpace(operationID),
			"error", errorValue,
		)
		writeCalendarErrorCode(responseWriter, http.StatusInternalServerError, calendarInternalErrorCode)
	}
}

func parseCalendarDeleteIntentPath(path string) (string, string, bool) {
	if !strings.HasPrefix(path, "/events/") {
		return "", "", false
	}
	trimmedPath := strings.TrimPrefix(path, "/events/")
	eventID, remainingPath, found := strings.Cut(trimmedPath, "/delete-intents/")
	if !found || strings.TrimSpace(eventID) == "" || strings.TrimSpace(remainingPath) == "" || strings.Contains(remainingPath, "/") {
		return "", "", false
	}
	return strings.TrimSpace(eventID), strings.TrimSpace(remainingPath), true
}
