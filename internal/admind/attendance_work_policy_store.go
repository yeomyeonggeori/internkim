package admind

import (
	"context"
	"slices"
	"time"
)

func (service *Service) readAttendanceWorkPolicy(ctx context.Context) (attendanceWorkPolicy, error) {
	document, errorValue := service.readAttendanceSettingsDocument(ctx)
	if errorValue != nil {
		return attendanceWorkPolicy{}, errorValue
	}
	return document.WorkPolicy, nil
}

func (service *Service) saveAttendanceWorkPolicyRevision(
	ctx context.Context,
	revision attendanceWorkPolicyRevision,
	effectiveDate string,
	now time.Time,
) (attendanceWorkPolicy, error) {
	revision.EffectiveDate = effectiveDate
	if errorValue := validateAndNormalizeAttendanceWorkPolicyRevision(&revision); errorValue != nil {
		return attendanceWorkPolicy{}, errorValue
	}
	var saved attendanceWorkPolicy
	errorValue := service.updateAttendanceSettingsDocument(ctx, func(document *attendanceSettingsDocument) error {
		policy := document.WorkPolicy
		policy.Revisions = attendanceWorkPolicyRevisionsWith(policy.Revisions, revision)
		policy.UpdatedAt = now.UTC().Format(time.RFC3339)
		if errorValue := validateAndNormalizeAttendanceWorkPolicy(&policy); errorValue != nil {
			return errorValue
		}
		document.WorkPolicy = policy
		saved = policy
		return nil
	})
	if errorValue != nil {
		return attendanceWorkPolicy{}, errorValue
	}
	return saved, nil
}

func attendanceWorkPolicyRevisionsWith(
	existing []attendanceWorkPolicyRevision,
	saved attendanceWorkPolicyRevision,
) []attendanceWorkPolicyRevision {
	revisions := append([]attendanceWorkPolicyRevision(nil), existing...)
	for index, revision := range revisions {
		if revision.EffectiveDate == saved.EffectiveDate {
			revisions[index] = saved
			return revisions
		}
	}
	inForce, found := attendanceWorkPolicyRevisionInForce(revisions, saved.EffectiveDate)
	if found && attendanceWorkPolicyRevisionsAgree(inForce, saved) {
		return revisions
	}
	return append(revisions, saved)
}

func attendanceWorkPolicyRevisionsAgree(left attendanceWorkPolicyRevision, right attendanceWorkPolicyRevision) bool {
	left.EffectiveDate = ""
	right.EffectiveDate = ""
	return left.WorkMode == right.WorkMode &&
		slices.Equal(left.WorkingWeekdays, right.WorkingWeekdays) &&
		left.DailyTargetMinutes == right.DailyTargetMinutes &&
		left.WeeklyTargetMinutes == right.WeeklyTargetMinutes &&
		left.ReferenceStartTime == right.ReferenceStartTime &&
		left.FixedStartTime == right.FixedStartTime &&
		left.FixedEndTime == right.FixedEndTime &&
		left.CoreTimeEnabled == right.CoreTimeEnabled &&
		left.CoreStartTime == right.CoreStartTime &&
		left.CoreEndTime == right.CoreEndTime &&
		slices.Equal(left.BreakPeriods, right.BreakPeriods) &&
		left.NightStartTime == right.NightStartTime &&
		left.NightEndTime == right.NightEndTime
}
