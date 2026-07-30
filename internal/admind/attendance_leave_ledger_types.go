package admind

import "errors"

const (
	attendanceLeaveOperationGrant           = "grant"
	attendanceLeaveOperationReserve         = "reserve"
	attendanceLeaveOperationUse             = "use"
	attendanceLeaveOperationRelease         = "release"
	attendanceLeaveOperationExpire          = "expire"
	attendanceLeaveOperationCarryover       = "carryover"
	attendanceLeaveOperationLegalCorrection = "legalCorrection"
	attendanceLeaveOperationAdjustment      = "adjustment"
	attendanceLeaveOperationUntrackedUse    = "untrackedUse"
)

var (
	errAttendanceLeaveInsufficientBalance = errors.New("insufficient leave balance")
	errAttendanceLeaveOperationConflict   = errors.New("leave operation conflicts with an existing operation")
	errAttendanceLeaveReservationClosed   = errors.New("leave reservation is already closed")
	errAttendanceLeaveReservationNotFound = errors.New("leave reservation not found")
)

type attendanceLeaveEmployee struct {
	Email    string
	UserID   string
	HireDate string
}

type attendanceLeaveOperation struct {
	OperationKey string
	Employee     attendanceLeaveEmployee
	LeaveTypeID  string
	Kind         string
	ReferenceID  string
	Amount       int
	EffectiveOn  string
}

type attendanceLeaveGrant struct {
	Operation attendanceLeaveOperation
	ExpiresOn string
}

type attendanceLeaveBalance struct {
	EmployeeEmail       string
	UserID              string
	LeaveTypeID         string
	GrantedMilliDays    int
	AvailableMilliDays  int
	ReservedMilliDays   int
	UsedMilliDays       int
	ExpiredMilliDays    int
	NextGrantDate       string
	NextGrantMilliDays  int
	NextExpiryDate      string
	NextExpiryMilliDays int
}

type attendanceLeaveLedgerEntry struct {
	ID                      string
	OperationKey            string
	Sequence                int
	GrantLotID              string
	Kind                    string
	AmountMilliDays         int
	AvailableDeltaMilliDays int
	ReservedDeltaMilliDays  int
	UsedDeltaMilliDays      int
	ExpiredDeltaMilliDays   int
	AvailableAfterMilliDays int
	EffectiveAt             string
	CreatedAt               string
}

type attendanceLeaveReservationAllocation struct {
	GrantLotID      string
	AmountMilliDays int
	EffectiveOn     string
}

type attendanceLeaveAccrual struct {
	GrantDate       string
	AmountMilliDays int
	ExpiresOn       string
	Kind            string
	ReferenceID     string
}
