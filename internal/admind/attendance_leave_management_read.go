package admind

import (
	"context"
	"database/sql"
	"net/http"
	"sort"
	"strings"
	"time"
)

func (service *Service) readAttendanceLeaveManagement(
	request *http.Request,
	selectedEmail string,
	now time.Time,
) (attendanceLeaveManagementResponse, error) {
	policy, errorValue := service.readAttendanceLeavePolicy(request.Context())
	if errorValue != nil {
		return attendanceLeaveManagementResponse{}, errorValue
	}
	database, errorValue := service.openAttendanceDatabase(request.Context())
	if errorValue != nil {
		return attendanceLeaveManagementResponse{}, errorValue
	}
	defer database.Close()
	members, errorValue := service.attendanceLeaveManagementMembers(request, database)
	if errorValue != nil {
		return attendanceLeaveManagementResponse{}, errorValue
	}
	for _, member := range members {
		if errorValue := service.synchronizeAttendanceLeaveAccruals(
			request.Context(),
			attendanceLeaveEmployeeFromMember(member),
			policy,
			now,
		); errorValue != nil {
			return attendanceLeaveManagementResponse{}, errorValue
		}
	}
	response := attendanceLeaveManagementResponse{
		LeaveTypes: attendanceLeaveTypeViews(policy, nil),
		Employees:  make([]attendanceLeaveManagementEmployeeView, 0, len(members)),
	}
	for _, member := range members {
		employee, employeeError := attendanceLeaveManagementEmployee(
			request.Context(),
			database,
			member,
			policy,
		)
		if employeeError != nil {
			return attendanceLeaveManagementResponse{}, employeeError
		}
		response.Employees = append(response.Employees, employee)
	}
	normalizedSelectedEmail := normalizeAttendanceLeaveEmail(selectedEmail)
	if normalizedSelectedEmail == "" {
		return response, nil
	}
	for _, employee := range response.Employees {
		if employee.Email != normalizedSelectedEmail {
			continue
		}
		requests, requestsError := readAttendanceLeaveRequestRecords(
			request.Context(),
			database,
			employee.Email,
		)
		if requestsError != nil {
			return attendanceLeaveManagementResponse{}, requestsError
		}
		requestViews := make([]attendanceLeaveRequestView, 0, len(requests))
		for _, record := range requests {
			requestViews = append(
				requestViews,
				projectAttendanceLeaveRequest(record, now, service.workspaceTimeZone().location),
			)
		}
		ledgerEntries, ledgerError := readAttendanceLeaveManagementLedger(
			request.Context(),
			database,
			employee.Email,
			policy,
		)
		if ledgerError != nil {
			return attendanceLeaveManagementResponse{}, ledgerError
		}
		response.Detail = &attendanceLeaveManagementDetailView{
			Employee:      employee,
			Requests:      requestViews,
			LedgerEntries: ledgerEntries,
		}
		break
	}
	return response, nil
}

func (service *Service) attendanceLeaveManagementMembers(
	request *http.Request,
	database *sql.DB,
) ([]attendanceMember, error) {
	membersByEmail := map[string]attendanceMember{}
	for _, member := range service.attendanceMembersForSummary(
		request,
		service.webStaffActorEmail(request),
		true,
		true,
	) {
		member.Email = normalizeAttendanceLeaveEmail(member.Email)
		if member.Email != "" {
			membersByEmail[member.Email] = member
		}
	}
	rows, errorValue := database.QueryContext(request.Context(), `
SELECT employee_email
FROM attendance_leave_operations
UNION
SELECT employee_email
FROM attendance_leave_requests`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		if errorValue := rows.Scan(&email); errorValue != nil {
			return nil, errorValue
		}
		email = normalizeAttendanceLeaveEmail(email)
		if email == "" {
			continue
		}
		if _, found := membersByEmail[email]; !found {
			membersByEmail[email] = attendanceMember{Email: email, DisplayName: email}
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	members := make([]attendanceMember, 0, len(membersByEmail))
	for _, member := range membersByEmail {
		members = append(members, member)
	}
	sort.Slice(members, func(firstIndex int, secondIndex int) bool {
		firstName := strings.ToLower(members[firstIndex].DisplayName)
		secondName := strings.ToLower(members[secondIndex].DisplayName)
		if firstName == secondName {
			return members[firstIndex].Email < members[secondIndex].Email
		}
		return firstName < secondName
	})
	return members, nil
}

func attendanceLeaveManagementEmployee(
	ctx context.Context,
	database *sql.DB,
	member attendanceMember,
	policy attendanceLeavePolicy,
) (attendanceLeaveManagementEmployeeView, error) {
	employee := attendanceLeaveManagementEmployeeView{
		Email:       normalizeAttendanceLeaveEmail(member.Email),
		DisplayName: strings.TrimSpace(member.DisplayName),
		Balances:    []attendanceLeaveManagementBalanceView{},
	}
	if employee.DisplayName == "" {
		employee.DisplayName = employee.Email
	}
	for _, leaveType := range policy.LeaveTypes {
		if !attendanceLeaveTypeOwnsBalance(leaveType) {
			continue
		}
		balance, errorValue := queryAttendanceLeaveBalance(
			ctx,
			database,
			attendanceLeaveEmployee{Email: employee.Email},
			leaveType.ID,
		)
		if errorValue != nil {
			return attendanceLeaveManagementEmployeeView{}, errorValue
		}
		employee.Balances = append(employee.Balances, attendanceLeaveManagementBalanceView{
			LeaveTypeID:         leaveType.ID,
			LeaveTypeName:       leaveType.Name,
			GrantedMilliDays:    balance.GrantedMilliDays,
			AvailableMilliDays:  balance.AvailableMilliDays,
			ReservedMilliDays:   balance.ReservedMilliDays,
			UsedMilliDays:       balance.UsedMilliDays,
			ExpiredMilliDays:    balance.ExpiredMilliDays,
			NextExpiryDate:      balance.NextExpiryDate,
			NextExpiryMilliDays: balance.NextExpiryMilliDays,
		})
		employee.GrantedMilliDays += balance.GrantedMilliDays
		employee.AvailableMilliDays += balance.AvailableMilliDays
		employee.ReservedMilliDays += balance.ReservedMilliDays
		employee.UsedMilliDays += balance.UsedMilliDays
		employee.ExpiringMilliDays += balance.NextExpiryMilliDays
	}
	return employee, nil
}

func readAttendanceLeaveManagementLedger(
	ctx context.Context,
	database *sql.DB,
	employeeEmail string,
	policy attendanceLeavePolicy,
) ([]attendanceLeaveManagementLedgerView, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT
	MIN(entry.id),
	operation.kind,
	operation.leave_type_id,
	CASE operation.kind
		WHEN 'use' THEN -SUM(entry.used_delta_milli_days)
		WHEN 'untrackedUse' THEN -SUM(entry.used_delta_milli_days)
		ELSE SUM(entry.available_delta_milli_days)
	END,
	(
		SELECT final_entry.available_after_milli_days
		FROM attendance_leave_ledger_entries final_entry
		WHERE final_entry.operation_key = operation.operation_key
		ORDER BY final_entry.sequence DESC
		LIMIT 1
	),
	operation.effective_date,
	operation.created_at,
	CASE
		WHEN operation.kind IN ('adjustment', 'legalCorrection') THEN operation.reference_id
		ELSE ''
	END
FROM attendance_leave_operations operation
JOIN attendance_leave_ledger_entries entry ON entry.operation_key = operation.operation_key
WHERE operation.employee_email = ?
GROUP BY operation.operation_key
ORDER BY operation.created_at DESC, operation.operation_key DESC`,
		normalizeAttendanceLeaveEmail(employeeEmail),
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	leaveTypeNames := map[string]string{}
	for _, leaveType := range policy.LeaveTypes {
		leaveTypeNames[leaveType.ID] = leaveType.Name
	}
	entries := []attendanceLeaveManagementLedgerView{}
	for rows.Next() {
		var entry attendanceLeaveManagementLedgerView
		if errorValue := rows.Scan(
			&entry.ID,
			&entry.OperationType,
			&entry.LeaveTypeID,
			&entry.DeltaMilliDays,
			&entry.BalanceAfterMilliDays,
			&entry.EffectiveOn,
			&entry.OccurredAt,
			&entry.Reason,
		); errorValue != nil {
			return nil, errorValue
		}
		entry.LeaveTypeName = leaveTypeNames[entry.LeaveTypeID]
		if entry.LeaveTypeName == "" {
			entry.LeaveTypeName = entry.LeaveTypeID
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}
