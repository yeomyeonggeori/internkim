package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type attendanceEventsCachePayload struct {
	Events []attendanceEvent `json:"events"`
}

type attendanceAbsencesCachePayload struct {
	Absences []attendanceAbsence `json:"absences"`
}

func (service *Service) readCachedAttendanceEvents(ctx context.Context, month string, email string) ([]attendanceEvent, error) {
	if strings.TrimSpace(email) != "" {
		return service.readAttendanceEvents(ctx, month, email)
	}
	now := time.Now().UTC()
	payload, revision, found, cacheError := service.readAttendanceSummaryCachePayload(ctx, attendanceSummaryCacheKindEvents, month, now)
	if cacheError != nil {
		logAttendanceSummaryCacheFailure("read", attendanceSummaryCacheKindEvents, month, email, cacheError)
	} else if found {
		var cachedPayload attendanceEventsCachePayload
		if errorValue := json.Unmarshal(payload, &cachedPayload); errorValue == nil && cachedPayload.Events != nil {
			return cachedPayload.Events, nil
		}
		if errorValue := service.deleteAttendanceSummaryCachePayloadIfUnchanged(ctx, attendanceSummaryCacheKindEvents, month, revision, payload); errorValue != nil {
			logAttendanceSummaryCacheFailure("delete corrupt payload", attendanceSummaryCacheKindEvents, month, email, errorValue)
		}
	}
	events, errorValue := service.readAttendanceEvents(ctx, month, email)
	if errorValue != nil {
		return nil, errorValue
	}
	if cacheError == nil {
		if errorValue := service.storeAttendanceEventsCachePayload(ctx, month, revision, events, now); errorValue != nil {
			logAttendanceSummaryCacheFailure("write", attendanceSummaryCacheKindEvents, month, email, errorValue)
		}
	}
	return events, nil
}

func (service *Service) storeAttendanceEventsCachePayload(
	ctx context.Context,
	month string,
	revision int64,
	events []attendanceEvent,
	now time.Time,
) error {
	payload, errorValue := json.Marshal(attendanceEventsCachePayload{Events: events})
	if errorValue != nil {
		return fmt.Errorf("encode attendance events cache payload: %w", errorValue)
	}
	_, errorValue = service.writeAttendanceSummaryCachePayloadIfCurrent(ctx, attendanceSummaryCacheKindEvents, month, revision, payload, now)
	return errorValue
}

func (service *Service) readCachedAttendanceAbsences(ctx context.Context, month string, email string) ([]attendanceAbsence, error) {
	if strings.TrimSpace(email) != "" {
		return service.readAttendanceAbsences(ctx, month, email)
	}
	now := time.Now().UTC()
	payload, revision, found, cacheError := service.readAttendanceSummaryCachePayload(ctx, attendanceSummaryCacheKindAbsences, month, now)
	if cacheError != nil {
		logAttendanceSummaryCacheFailure("read", attendanceSummaryCacheKindAbsences, month, email, cacheError)
	} else if found {
		var cachedPayload attendanceAbsencesCachePayload
		if errorValue := json.Unmarshal(payload, &cachedPayload); errorValue == nil && cachedPayload.Absences != nil {
			return cachedPayload.Absences, nil
		}
		if errorValue := service.deleteAttendanceSummaryCachePayloadIfUnchanged(ctx, attendanceSummaryCacheKindAbsences, month, revision, payload); errorValue != nil {
			logAttendanceSummaryCacheFailure("delete corrupt payload", attendanceSummaryCacheKindAbsences, month, email, errorValue)
		}
	}
	absences, errorValue := service.readAttendanceAbsences(ctx, month, email)
	if errorValue != nil {
		return nil, errorValue
	}
	if cacheError == nil {
		if errorValue := service.storeAttendanceAbsencesCachePayload(ctx, month, revision, absences, now); errorValue != nil {
			logAttendanceSummaryCacheFailure("write", attendanceSummaryCacheKindAbsences, month, email, errorValue)
		}
	}
	return absences, nil
}

func (service *Service) storeAttendanceAbsencesCachePayload(
	ctx context.Context,
	month string,
	revision int64,
	absences []attendanceAbsence,
	now time.Time,
) error {
	payload, errorValue := json.Marshal(attendanceAbsencesCachePayload{Absences: absences})
	if errorValue != nil {
		return fmt.Errorf("encode attendance absences cache payload: %w", errorValue)
	}
	_, errorValue = service.writeAttendanceSummaryCachePayloadIfCurrent(ctx, attendanceSummaryCacheKindAbsences, month, revision, payload, now)
	return errorValue
}

func logAttendanceSummaryCacheFailure(operation string, kind attendanceSummaryCacheKind, month string, email string, errorValue error) {
	slog.Warn(
		"attendance summary cache operation failed",
		"operation", operation,
		"cache_kind", string(kind),
		"month", month,
		"target_email", strings.ToLower(strings.TrimSpace(email)),
		"error", errorValue.Error(),
	)
}
