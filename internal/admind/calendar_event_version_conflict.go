package admind

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const calendarEventVersionConflictErrorCode = "calendar_event_version_conflict"

var errCalendarEventVersionConflict = errors.New("calendar event version conflict")

func (service *Service) writeCalendarEventIfCurrentVersion(ctx context.Context, event calendarEvent, expectedUpdatedAt string) error {
	candidateUpdatedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, time.Now().UTC())
	if errorValue != nil {
		return errorValue
	}
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	currentEvent, found, errorValue := service.readCalendarEventByID(ctx, event.ID)
	if errorValue != nil {
		return errorValue
	}
	if !found || strings.TrimSpace(currentEvent.UpdatedAt) != strings.TrimSpace(expectedUpdatedAt) {
		return errCalendarEventVersionConflict
	}
	return service.writeCalendarEventWithSourceLocked(ctx, event, calendarSourceLocal, candidateUpdatedAt)
}

func (service *Service) softDeleteCalendarEventIfCurrentVersion(ctx context.Context, eventID string, expectedUpdatedAt string) error {
	candidateDeletedAt, errorValue := service.reserveCalendarConflictCandidateTime(ctx, time.Now().UTC())
	if errorValue != nil {
		return errorValue
	}
	service.calendarStoreWriteMutex.Lock()
	defer service.calendarStoreWriteMutex.Unlock()
	currentEvent, found, errorValue := service.readCalendarEventByID(ctx, eventID)
	if errorValue != nil {
		return errorValue
	}
	if !found || strings.TrimSpace(currentEvent.UpdatedAt) != strings.TrimSpace(expectedUpdatedAt) {
		return errCalendarEventVersionConflict
	}
	return service.softDeleteCalendarEventWithSourceLocked(ctx, eventID, calendarSourceLocal, candidateDeletedAt)
}

func normalizeExpectedCalendarEventUpdatedAt(value string) (string, error) {
	expectedUpdatedAt := strings.TrimSpace(value)
	if expectedUpdatedAt == "" {
		return "", fmt.Errorf("expectedUpdatedAt is required")
	}
	if _, errorValue := time.Parse(time.RFC3339Nano, expectedUpdatedAt); errorValue != nil {
		return "", fmt.Errorf("expectedUpdatedAt must be RFC3339")
	}
	return expectedUpdatedAt, nil
}

func writeCalendarEventVersionConflictError(responseWriter http.ResponseWriter, errorValue error) bool {
	if !errors.Is(errorValue, errCalendarEventVersionConflict) {
		return false
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(responseWriter).Encode(calendarMutationErrorResponse{Code: calendarEventVersionConflictErrorCode})
	return true
}
