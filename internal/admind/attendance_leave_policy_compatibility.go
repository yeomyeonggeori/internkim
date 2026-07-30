package admind

func normalizeLegacyAttendanceLeavePolicy(policy *attendanceLeavePolicy) {
	for index := range policy.LeaveTypes {
		switch policy.LeaveTypes[index].GrantCadence {
		case "statutory":
			policy.LeaveTypes[index].GrantCadence = "annual"
		case "manual":
			policy.LeaveTypes[index].GrantCadence = "none"
		}
	}
	if policy.Version >= attendanceLeavePolicyVersion {
		for index := range policy.LeaveTypes {
			if !policy.LeaveTypes[index].IsActive {
				policy.LeaveTypes[index].IncludeInSummary = false
			}
		}
		return
	}
	migrateAttendanceLeavePolicyV1(policy)
}

func migrateAttendanceLeavePolicyV1(policy *attendanceLeavePolicy) {
	if len(policy.LeaveTypes) == 0 {
		defaultPolicy := defaultAttendanceLeavePolicy()
		*policy = defaultPolicy
		return
	}
	existingByID := make(map[string]attendanceLeaveType, len(policy.LeaveTypes))
	customTypes := make([]attendanceLeaveType, 0, len(policy.LeaveTypes))
	defaultByID := make(map[string]bool, len(defaultAttendanceLeaveTypes()))
	for _, leaveType := range defaultAttendanceLeaveTypes() {
		defaultByID[leaveType.ID] = true
	}
	for _, leaveType := range policy.LeaveTypes {
		if attendanceLeaveTypeOwnsBalance(leaveType) && leaveType.IsActive {
			leaveType.IncludeInSummary = true
		}
		if defaultByID[leaveType.ID] {
			existingByID[leaveType.ID] = leaveType
			continue
		}
		customTypes = append(customTypes, leaveType)
	}
	migrated := make([]attendanceLeaveType, 0, len(defaultAttendanceLeaveTypes())+len(customTypes))
	for _, defaultType := range defaultAttendanceLeaveTypes() {
		if existing, found := existingByID[defaultType.ID]; found {
			existing.SortOrder = len(migrated)
			migrated = append(migrated, existing)
			continue
		}
		defaultType.SortOrder = len(migrated)
		migrated = append(migrated, defaultType)
	}
	for _, customType := range customTypes {
		customType.SortOrder = len(migrated)
		migrated = append(migrated, customType)
	}
	policy.Version = attendanceLeavePolicyVersion
	policy.LeaveTypes = migrated
}
