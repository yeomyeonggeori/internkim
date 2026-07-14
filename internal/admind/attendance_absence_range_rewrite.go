package admind

import (
	"context"
	"database/sql"
)

type attendanceAbsenceRangeRewriteRequest struct {
	Email      string
	Kind       string
	Reason     string
	ActorEmail string
	StartDate  string
	EndDate    string
	Timestamp  string
	Dates      []string
}

type attendanceAbsenceRangeRewriteResult struct {
	CreatedRanges []attendanceAbsenceRange
	CreatedDates  []string
	AffectedDates []string
}

func rewriteAttendanceAbsenceRanges(ctx context.Context, transaction *sql.Tx, request attendanceAbsenceRangeRewriteRequest) (attendanceAbsenceRangeRewriteResult, error) {
	requestedDates := attendanceDateSet(request.Dates)
	alreadyCoveredDates := map[string]struct{}{}
	mergeDates := map[string]struct{}{}
	mergeRangeIDs := map[string]struct{}{}
	affectedDates := map[string]struct{}{}
	existingRanges, errorValue := readAttendanceAbsenceRangesForUpdate(ctx, transaction, request.Email, previousAttendanceBusinessDate(request.StartDate), nextAttendanceBusinessDate(request.EndDate))
	if errorValue != nil {
		return attendanceAbsenceRangeRewriteResult{}, errorValue
	}
	for _, existingRange := range existingRanges {
		existingDates, errorValue := attendanceAbsenceDates(existingRange.StartDate, existingRange.EndDate)
		if errorValue != nil {
			return attendanceAbsenceRangeRewriteResult{}, errorValue
		}
		overlapDates := attendanceDateRangeOverlapDates(existingRange.StartDate, existingRange.EndDate, requestedDates)
		if len(overlapDates) == 0 && (existingRange.Kind != request.Kind || existingRange.Reason != request.Reason) {
			continue
		}
		if existingRange.Kind == request.Kind && existingRange.Reason == request.Reason {
			for _, date := range overlapDates {
				alreadyCoveredDates[date] = struct{}{}
			}
			for _, date := range existingDates {
				mergeDates[date] = struct{}{}
			}
			mergeRangeIDs[existingRange.ID] = struct{}{}
			continue
		}
		replacementID := "absence-range-replace-" + randomHex(12)
		if errorValue := cancelAttendanceAbsenceRangeInTransaction(ctx, transaction, existingRange.ID, request.Timestamp, replacementID); errorValue != nil {
			return attendanceAbsenceRangeRewriteResult{}, errorValue
		}
		for _, date := range existingDates {
			affectedDates[date] = struct{}{}
		}
		remainingDates := subtractAttendanceDates(existingDates, requestedDates)
		if _, errorValue := insertAttendanceAbsenceRangesForDates(ctx, transaction, existingRange.Email, existingRange.Kind, existingRange.Reason, existingRange.CreatedBy, existingRange.CreatedAt, request.Timestamp, remainingDates); errorValue != nil {
			return attendanceAbsenceRangeRewriteResult{}, errorValue
		}
	}
	createdDates := subtractAttendanceDates(request.Dates, alreadyCoveredDates)
	if len(createdDates) > 0 && len(mergeRangeIDs) > 0 {
		for _, date := range request.Dates {
			mergeDates[date] = struct{}{}
		}
		for rangeID := range mergeRangeIDs {
			if errorValue := cancelAttendanceAbsenceRangeInTransaction(ctx, transaction, rangeID, request.Timestamp, ""); errorValue != nil {
				return attendanceAbsenceRangeRewriteResult{}, errorValue
			}
		}
		createdRanges, errorValue := insertAttendanceAbsenceRangesForDates(ctx, transaction, request.Email, request.Kind, request.Reason, request.ActorEmail, request.Timestamp, request.Timestamp, sortedAttendanceDatesFromSet(mergeDates))
		if errorValue != nil {
			return attendanceAbsenceRangeRewriteResult{}, errorValue
		}
		for date := range mergeDates {
			affectedDates[date] = struct{}{}
		}
		return attendanceAbsenceRangeRewriteResult{
			CreatedRanges: createdRanges,
			CreatedDates:  createdDates,
			AffectedDates: sortedAttendanceDatesFromSet(affectedDates),
		}, nil
	}
	createdRanges, errorValue := insertAttendanceAbsenceRangesForDates(ctx, transaction, request.Email, request.Kind, request.Reason, request.ActorEmail, request.Timestamp, request.Timestamp, createdDates)
	if errorValue != nil {
		return attendanceAbsenceRangeRewriteResult{}, errorValue
	}
	for _, date := range createdDates {
		affectedDates[date] = struct{}{}
	}
	return attendanceAbsenceRangeRewriteResult{
		CreatedRanges: createdRanges,
		CreatedDates:  createdDates,
		AffectedDates: sortedAttendanceDatesFromSet(affectedDates),
	}, nil
}
