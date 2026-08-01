package admind

import (
	"context"
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
		replaced := false
		for index := range policy.Revisions {
			if policy.Revisions[index].EffectiveDate != effectiveDate {
				continue
			}
			policy.Revisions[index] = revision
			replaced = true
			break
		}
		if !replaced {
			policy.Revisions = append(policy.Revisions, revision)
		}
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
