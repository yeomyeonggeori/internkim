package admind

import "errors"

const (
	attendanceLeaveApprovalActionApprove      = "approve"
	attendanceLeaveApprovalActionNeedsChanges = "needsChanges"
	attendanceLeaveApprovalActionReject       = "reject"
	attendanceLeaveApprovalChangeEarlyReturn  = "earlyReturn"
)

var errAttendanceLeaveApprovalConflict = errors.New("leave request is no longer pending")

type attendanceLeaveApprovalInput struct {
	Action   string `json:"action"`
	Response string `json:"response"`
}

type attendanceLeaveApprovalBalanceView struct {
	AvailableMilliDays int `json:"availableMilliDays"`
	ReservedMilliDays  int `json:"reservedMilliDays"`
	UsedMilliDays      int `json:"usedMilliDays"`
}

type attendanceLeaveApprovalRequestView struct {
	ID                 string                             `json:"id"`
	EmployeeEmail      string                             `json:"employeeEmail"`
	LeaveTypeID        string                             `json:"leaveTypeID"`
	LeaveTypeName      string                             `json:"leaveTypeName"`
	BalanceMode        string                             `json:"balanceMode"`
	Status             string                             `json:"status"`
	Unit               string                             `json:"unit"`
	StartDate          string                             `json:"startDate"`
	EndDate            string                             `json:"endDate,omitempty"`
	PartialPeriod      string                             `json:"partialPeriod,omitempty"`
	StartTime          string                             `json:"startTime,omitempty"`
	EndTime            string                             `json:"endTime,omitempty"`
	DeductionMilliDays int                                `json:"deductionMilliDays"`
	Reason             string                             `json:"reason"`
	AdminResponse      string                             `json:"adminResponse,omitempty"`
	Attachments        []attendanceLeaveRequestAttachment `json:"attachments"`
	Balance            attendanceLeaveApprovalBalanceView `json:"balance"`
	CreatedAt          string                             `json:"createdAt"`
	UpdatedAt          string                             `json:"updatedAt"`
}

type attendanceLeaveApprovalChangeView struct {
	Request    attendanceLeaveApprovalRequestView `json:"request"`
	Change     string                             `json:"change"`
	Response   string                             `json:"response,omitempty"`
	ReturnedAt string                             `json:"returnedAt,omitempty"`
	ChangedAt  string                             `json:"changedAt"`
}

type attendanceLeaveApprovalInbox struct {
	PendingCount  int                                  `json:"pendingCount"`
	Pending       []attendanceLeaveApprovalRequestView `json:"pending"`
	RecentChanges []attendanceLeaveApprovalChangeView  `json:"recentChanges"`
}
