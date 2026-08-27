package admind

const attendanceDefaultAnnualGrantMilliDays = 15000

func defaultAttendanceLeavePolicy() attendanceLeavePolicy {
	return attendanceLeavePolicy{
		Version:              attendanceLeavePolicyVersion,
		BalanceTrackingMode:  attendanceLeaveBalanceTrackingManaged,
		FiscalYearStartMonth: 1,
		FiscalYearStartDay:   1,
		LeaveTypes:           defaultAttendanceLeaveTypes(),
	}
}

func defaultAttendanceLeaveTypes() []attendanceLeaveType {
	return []attendanceLeaveType{
		defaultAttendanceLeaveType("annual", "annual", "연차", true, "annual", "annual", attendanceDefaultAnnualGrantMilliDays, "fiscalYearEnd", attendanceDefaultPartialLeaveUnits(), true, 0),
		defaultAttendanceLeaveType("sick", "sick", "병가", true, "none", "none", 0, "none", attendanceDefaultPartialLeaveUnits(), false, 1),
		defaultAttendanceLeaveType("maternity", "maternity", "출산·육아휴가", true, "none", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), false, 2),
		defaultAttendanceLeaveType("unpaid", "unpaid", "무급휴가", false, "none", "none", 0, "none", attendanceDefaultPartialLeaveUnits(), false, 3),
	}
}

func attendanceSystemLeaveTypeKinds() map[string]string {
	return map[string]string{
		"annual":                 "annual",
		"sick":                   "sick",
		"unpaid":                 "unpaid",
		"bereavement":            "bereavement",
		"public":                 "public",
		"maternity":              "maternity",
		"spouse-maternity":       "spouseMaternity",
		"miscarriage-stillbirth": "miscarriageStillbirth",
		"fertility-treatment":    "fertilityTreatment",
		"family-care":            "familyCare",
		"reward":                 "reward",
		"compensatory":           "compensatory",
		"long-service":           "longService",
		"refresh":                "refresh",
		"parental-leave":         "parentalLeave",
		"other":                  "other",
	}
}

func defaultAttendanceLeaveType(
	id string,
	systemKind string,
	name string,
	paid bool,
	balanceMode string,
	grantCadence string,
	grantAmountMilliDays int,
	expiryMode string,
	allowedUnits []string,
	includeInSummary bool,
	sortOrder int,
) attendanceLeaveType {
	return attendanceLeaveType{
		ID:                   id,
		SystemKind:           systemKind,
		Name:                 name,
		Paid:                 paid,
		BalanceMode:          balanceMode,
		GrantCadence:         grantCadence,
		GrantAmountMilliDays: grantAmountMilliDays,
		ExpiryMode:           expiryMode,
		AllowedUnits:         allowedUnits,
		IncludeInSummary:     includeInSummary,
		IsActive:             true,
		IsSystem:             true,
		SortOrder:            sortOrder,
	}
}

func attendanceDefaultPartialLeaveUnits() []string {
	return []string{"fullDay", "halfDay", "quarterDay"}
}

func attendanceDefaultFullDayLeaveUnits() []string {
	return []string{"fullDay"}
}
