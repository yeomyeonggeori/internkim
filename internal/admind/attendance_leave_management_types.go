package admind

type attendanceLeaveManagementBalanceView struct {
	LeaveTypeID         string `json:"leaveTypeID"`
	LeaveTypeName       string `json:"leaveTypeName"`
	GrantedMilliDays    int    `json:"grantedMilliDays"`
	AvailableMilliDays  int    `json:"availableMilliDays"`
	ReservedMilliDays   int    `json:"reservedMilliDays"`
	UsedMilliDays       int    `json:"usedMilliDays"`
	ExpiredMilliDays    int    `json:"expiredMilliDays"`
	NextExpiryDate      string `json:"nextExpiryDate,omitempty"`
	NextExpiryMilliDays int    `json:"nextExpiryMilliDays"`
}

type attendanceLeaveManagementEmployeeView struct {
	Email              string                                 `json:"email"`
	DisplayName        string                                 `json:"displayName"`
	GrantedMilliDays   int                                    `json:"grantedMilliDays"`
	AvailableMilliDays int                                    `json:"availableMilliDays"`
	ReservedMilliDays  int                                    `json:"reservedMilliDays"`
	UsedMilliDays      int                                    `json:"usedMilliDays"`
	ExpiringMilliDays  int                                    `json:"expiringMilliDays"`
	Balances           []attendanceLeaveManagementBalanceView `json:"balances"`
}

type attendanceLeaveManagementLedgerView struct {
	ID                    string `json:"id"`
	OperationType         string `json:"operationType"`
	LeaveTypeID           string `json:"leaveTypeID"`
	LeaveTypeName         string `json:"leaveTypeName"`
	DeltaMilliDays        int    `json:"deltaMilliDays"`
	BalanceAfterMilliDays int    `json:"balanceAfterMilliDays"`
	EffectiveOn           string `json:"effectiveOn"`
	OccurredAt            string `json:"occurredAt"`
	Reason                string `json:"reason,omitempty"`
}

type attendanceLeaveManagementDetailView struct {
	Employee      attendanceLeaveManagementEmployeeView `json:"employee"`
	Requests      []attendanceLeaveRequestView          `json:"requests"`
	LedgerEntries []attendanceLeaveManagementLedgerView `json:"ledgerEntries"`
}

type attendanceLeaveManagementResponse struct {
	LeaveTypes []attendanceLeaveTypeView               `json:"leaveTypes"`
	Employees  []attendanceLeaveManagementEmployeeView `json:"employees"`
	Detail     *attendanceLeaveManagementDetailView    `json:"detail,omitempty"`
}

type attendanceLeaveManagementAdjustmentInput struct {
	EmployeeEmail   string `json:"employeeEmail"`
	LeaveTypeID     string `json:"leaveTypeID"`
	AmountMilliDays int    `json:"amountMilliDays"`
	Kind            string `json:"kind"`
	Reason          string `json:"reason"`
	EffectiveOn     string `json:"effectiveOn"`
	ExpiresOn       string `json:"expiresOn"`
}

type attendanceLeaveManagementPastLeaveInput struct {
	EmployeeEmail string `json:"employeeEmail"`
	LeaveTypeID   string `json:"leaveTypeID"`
	Unit          string `json:"unit"`
	StartDate     string `json:"startDate"`
	EndDate       string `json:"endDate"`
	PartialPeriod string `json:"partialPeriod"`
	StartTime     string `json:"startTime"`
	Reason        string `json:"reason"`
}

type attendanceLeaveManagementCancelInput struct {
	EmployeeEmail string `json:"employeeEmail"`
}

type attendanceLeaveManagementTimeInput struct {
	EmployeeEmail string `json:"employeeEmail"`
	StartTime     string `json:"startTime"`
	EndTime       string `json:"endTime"`
	Reason        string `json:"reason"`
}
