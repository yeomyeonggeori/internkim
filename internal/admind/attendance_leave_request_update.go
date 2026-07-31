package admind

import (
	"context"
	"time"
)

func (service *Service) updateAttendanceLeaveRequest(
	ctx context.Context,
	requestID string,
	employee attendanceLeaveEmployee,
	input attendanceLeaveRequestInput,
	preview attendanceLeaveRequestPreview,
	leaveType attendanceLeaveType,
	policy attendanceLeavePolicy,
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
		policy,
		attachments,
		attendanceLeaveRequestRevisionOperation{
			ExpectedStatus:       attendanceLeaveRequestStatusPending,
			NextStatus:           attendanceLeaveRequestStatusPending,
			ExpectedRevision:     input.Revision,
			EventKind:            "updated",
			RemovedAttachmentIDs: input.RemovedAttachmentIDs,
			ConflictError:        errAttendanceLeaveRequestCannotUpdate,
		},
		now,
	)
}
