package admind

import "strings"

const attendanceAnnualLeaveTypeID = "annual"

func attendanceLeaveBalanceAccountID(leaveTypeID string, balanceMode string) string {
	if balanceMode == "annual" {
		return attendanceAnnualLeaveTypeID
	}
	return strings.TrimSpace(leaveTypeID)
}

func attendanceLeaveTypeOwnsBalance(leaveType attendanceLeaveType) bool {
	return leaveType.BalanceMode == "separate" ||
		(leaveType.BalanceMode == "annual" && leaveType.ID == attendanceAnnualLeaveTypeID)
}
