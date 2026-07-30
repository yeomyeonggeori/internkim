package admind

const attendanceDefaultAnnualGrantMilliDays = 15000

func defaultAttendanceLeavePolicy() attendanceLeavePolicy {
	return attendanceLeavePolicy{
		Version:              attendanceLeavePolicyVersion,
		FiscalYearStartMonth: 1,
		FiscalYearStartDay:   1,
		LeaveTypes:           defaultAttendanceLeaveTypes(),
	}
}

func defaultAttendanceLeaveTypes() []attendanceLeaveType {
	return []attendanceLeaveType{
		defaultAttendanceLeaveType("annual", "annual", "연차", true, "annual", "annual", attendanceDefaultAnnualGrantMilliDays, "fiscalYearEnd", attendanceDefaultPartialLeaveUnits(), true, 0),
		defaultAttendanceLeaveType("sick", "sick", "병가", false, "none", "none", 0, "none", attendanceDefaultPartialLeaveUnits(), false, 1),
		defaultAttendanceLeaveType("bereavement", "bereavement", "경조휴가", true, "none", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), false, 2),
		defaultAttendanceLeaveType("public", "public", "공가", true, "none", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), false, 3),
		defaultAttendanceLeaveType("maternity", "maternity", "출산휴가", true, "none", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), false, 4),
		defaultAttendanceLeaveType("spouse-maternity", "spouseMaternity", "배우자 출산휴가", true, "none", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), false, 5),
		defaultAttendanceLeaveType("miscarriage-stillbirth", "miscarriageStillbirth", "유산·사산휴가", true, "none", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), false, 6),
		defaultAttendanceLeaveType("fertility-treatment", "fertilityTreatment", "난임치료휴가", true, "none", "none", 0, "none", attendanceDefaultPartialLeaveUnits(), false, 7),
		defaultAttendanceLeaveType("family-care", "familyCare", "가족돌봄휴가", false, "none", "none", 0, "none", attendanceDefaultPartialLeaveUnits(), false, 8),
		defaultAttendanceLeaveType("reward", "reward", "포상휴가", true, "separate", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), true, 9),
		defaultAttendanceLeaveType("compensatory", "compensatory", "보상휴가", true, "separate", "none", 0, "none", attendanceDefaultPartialLeaveUnits(), true, 10),
		defaultAttendanceLeaveType("long-service", "longService", "장기근속휴가", true, "separate", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), true, 11),
		defaultAttendanceLeaveType("refresh", "refresh", "리프레시휴가", true, "separate", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), true, 12),
		defaultAttendanceLeaveType("parental-leave", "parentalLeave", "육아휴직", false, "none", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), false, 13),
		defaultAttendanceLeaveType("unpaid", "unpaid", "무급휴가", false, "none", "none", 0, "none", attendanceDefaultPartialLeaveUnits(), false, 14),
		defaultAttendanceLeaveType("other", "other", "기타 휴가", false, "none", "none", 0, "none", attendanceDefaultFullDayLeaveUnits(), false, 15),
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
