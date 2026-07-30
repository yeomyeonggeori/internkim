package admind

import (
	"context"
	"time"
)

func (service *Service) resubmitAttendanceLeaveRequest(
	ctx context.Context,
	requestID string,
	employee attendanceLeaveEmployee,
	input attendanceLeaveRequestInput,
	preview attendanceLeaveRequestPreview,
	leaveType attendanceLeaveType,
	attachments []attendanceLeaveRequestAttachment,
	now time.Time,
) (attendanceLeaveRequestRecord, error) {
	return service.reviseAttendanceLeaveRequest(
		ctx,
		requestID,
		employee,
		input,
		preview,
		leaveType,
		attachments,
		attendanceLeaveRequestRevisionOperation{
			ExpectedStatus: attendanceLeaveRequestStatusNeedsChanges,
			NextStatus:     attendanceLeaveRequestStatusPending,
			EventKind:      "resubmitted",
			EventResponse:  input.Response,
			ConflictError:  errAttendanceLeaveRequestCannotResubmit,
		},
		now,
	)
}
