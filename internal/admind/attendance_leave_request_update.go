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
