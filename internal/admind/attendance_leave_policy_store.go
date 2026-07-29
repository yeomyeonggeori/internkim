package admind

import (
	"context"
	"time"
)

func (service *Service) readAttendanceLeavePolicy(ctx context.Context) (attendanceLeavePolicy, error) {
	document, errorValue := service.readAttendanceSettingsDocument(ctx)
	if errorValue != nil {
		return attendanceLeavePolicy{}, errorValue
	}
	return document.LeavePolicy, nil
}

func (service *Service) writeAttendanceLeavePolicy(ctx context.Context, policy attendanceLeavePolicy) error {
	return service.updateAttendanceSettingsDocument(ctx, func(document *attendanceSettingsDocument) error {
		document.LeavePolicy = policy
		return nil
	})
}

func setAttendanceLeavePolicyTimestamp(
	policy *attendanceLeavePolicy,
	now time.Time,
) {
	policy.UpdatedAt = now.UTC().Format(time.RFC3339)
}
