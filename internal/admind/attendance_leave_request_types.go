package admind

import "errors"

const (
	attendanceLeaveRequestStatusPending      = "pending"
	attendanceLeaveRequestStatusNeedsChanges = "needsChanges"
	attendanceLeaveRequestStatusApproved     = "approved"
	attendanceLeaveRequestStatusRejected     = "rejected"
	attendanceLeaveRequestStatusCancelled    = "cancelled"

	attendanceLeavePartialPeriodMorning   = "morning"
	attendanceLeavePartialPeriodAfternoon = "afternoon"
	attendanceLeavePartialPeriodCustom    = "custom"
)

var errAttendanceLeaveRequestConflict = errors.New("leave request conflicts with an existing attendance record")
var errAttendanceLeaveRequestNotFound = errors.New("leave request not found")
var errAttendanceLeaveRequestCannotCancel = errors.New("leave request cannot be cancelled")
var errAttendanceLeaveRequestAlreadyStarted = errors.New("approved leave request has already started")
var errAttendanceLeaveRequestCannotResubmit = errors.New("leave request does not need changes")
var errAttendanceLeaveRequestCannotUpdate = errors.New("leave request can no longer be updated")
var errAttendanceLeaveRequestAttachmentLimit = errors.New("leave request attachment limit exceeded")

type attendanceLeaveRequestInput struct {
	LeaveTypeID          string   `json:"leaveTypeID"`
	Unit                 string   `json:"unit"`
	StartDate            string   `json:"startDate"`
	EndDate              string   `json:"endDate"`
	PartialPeriod        string   `json:"partialPeriod"`
	StartTime            string   `json:"startTime"`
	Reason               string   `json:"reason"`
	Response             string   `json:"response"`
	Revision             int      `json:"revision"`
	RemovedAttachmentIDs []string `json:"removedAttachmentIDs"`
}

type attendanceLeaveRequestRecord struct {
	ID                      string
	EmployeeEmail           string
	UserID                  string
	LeaveTypeID             string
	LeaveTypeName           string
	BalanceMode             string
	Status                  string
	Unit                    string
	StartDate               string
	EndDate                 string
	PartialPeriod           string
	StartTime               string
	Reason                  string
	AdminResponse           string
	TotalDeductionMilliDays int
	Revision                int
	CreatedAt               string
	UpdatedAt               string
	CancelledAt             string
	Occurrences             []attendanceLeaveRequestOccurrence
	Attachments             []attendanceLeaveRequestAttachment
}

type attendanceLeaveRequestAttachment struct {
	ID          string `json:"id"`
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	SizeBytes   int64  `json:"sizeBytes"`
	DownloadURL string `json:"downloadURL"`
	StorageKey  string `json:"-"`
}

type attendanceLeaveRequestAttachmentUpload struct {
	FileName    string
	ContentType string
	Content     []byte
}

type attendanceLeaveRequestView struct {
	ID                 string                             `json:"id"`
	LeaveTypeID        string                             `json:"leaveTypeID"`
	LeaveTypeName      string                             `json:"leaveTypeName"`
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
	CanCancel          bool                               `json:"canCancel"`
	CanEdit            bool                               `json:"canEdit"`
	CanResubmit        bool                               `json:"canResubmit"`
	Revision           int                                `json:"revision"`
	CreatedAt          string                             `json:"createdAt"`
	UpdatedAt          string                             `json:"updatedAt"`
}

type attendanceLeaveDashboardSummary struct {
	UsedMilliDays      int `json:"usedMilliDays"`
	ReservedMilliDays  int `json:"reservedMilliDays"`
	AvailableMilliDays int `json:"availableMilliDays"`
}

type attendanceLeaveTypeView struct {
	ID               string                           `json:"id"`
	Name             string                           `json:"name"`
	BalanceMode      string                           `json:"balanceMode"`
	AllowedUnits     []string                         `json:"allowedUnits"`
	IncludeInSummary bool                             `json:"includeInSummary"`
	Balance          *attendanceLeaveDashboardSummary `json:"balance,omitempty"`
	IsActive         bool                             `json:"isActive"`
	RequiresHireDate bool                             `json:"requiresHireDate"`
}

type attendanceLeaveLedgerView struct {
	ID                    string `json:"id"`
	OperationKey          string `json:"operationKey"`
	OperationType         string `json:"operationType"`
	OccurredAt            string `json:"occurredAt"`
	LeaveTypeID           string `json:"leaveTypeID"`
	LeaveTypeName         string `json:"leaveTypeName"`
	DeltaMilliDays        int    `json:"deltaMilliDays"`
	BalanceAfterMilliDays int    `json:"balanceAfterMilliDays"`
	IsUntracked           bool   `json:"isUntracked"`
	RequestID             string `json:"requestID,omitempty"`
}

type attendanceLeaveDashboard struct {
	LeaveTypes       []attendanceLeaveTypeView       `json:"leaveTypes"`
	Summary          attendanceLeaveDashboardSummary `json:"summary"`
	Requests         []attendanceLeaveRequestView    `json:"requests"`
	LedgerEntries    []attendanceLeaveLedgerView     `json:"ledgerEntries"`
	HireDateRequired bool                            `json:"hireDateRequired"`
}

type attendanceLeaveRequestOccurrence struct {
	Date               string `json:"date"`
	StartTime          string `json:"startTime"`
	EndTime            string `json:"endTime"`
	DeductionMilliDays int    `json:"deductionMilliDays"`
}

type attendanceLeaveRequestExcludedDate struct {
	Date   string `json:"date"`
	Reason string `json:"reason"`
}

type attendanceLeaveRequestPreview struct {
	Occurrences             []attendanceLeaveRequestOccurrence   `json:"occurrences"`
	ExcludedDates           []attendanceLeaveRequestExcludedDate `json:"excludedDates"`
	TotalDeductionMilliDays int                                  `json:"totalDeductionMilliDays"`
}
