package admind

import (
	"context"
	"time"
)

func (service *Service) updateAttendanceLeavePolicyExpiries(
	ctx context.Context,
	employees []attendanceLeaveEmployee,
	existing attendanceLeavePolicy,
	updated attendanceLeavePolicy,
) error {
	existingByID := make(map[string]attendanceLeaveType, len(existing.LeaveTypes))
	for _, leaveType := range existing.LeaveTypes {
		existingByID[leaveType.ID] = leaveType
	}
	for _, employee := range employees {
		for _, updatedType := range updated.LeaveTypes {
			existingType, exists := existingByID[updatedType.ID]
			if !exists ||
				!attendanceLeaveTypeOwnsBalance(updatedType) ||
				attendanceLeaveBalanceAccountID(existingType.ID, existingType.BalanceMode) !=
					attendanceLeaveBalanceAccountID(updatedType.ID, updatedType.BalanceMode) ||
				attendanceLeaveExpiryPolicyEqual(existingType, updatedType) {
				continue
			}
			if errorValue := service.updateAttendanceLeaveGrantLotExpiries(
				ctx,
				employee,
				updated,
				updatedType,
			); errorValue != nil {
				return errorValue
			}
		}
	}
	return nil
}

func attendanceLeaveExpiryPolicyEqual(
	first attendanceLeaveType,
	second attendanceLeaveType,
) bool {
	if first.ExpiryMode != second.ExpiryMode ||
		first.CarryoverEnabled != second.CarryoverEnabled {
		return false
	}
	if (first.ExpiryMonths == nil) != (second.ExpiryMonths == nil) ||
		(first.CarryoverLimitMilliDays == nil) != (second.CarryoverLimitMilliDays == nil) {
		return false
	}
	if first.ExpiryMonths != nil && *first.ExpiryMonths != *second.ExpiryMonths {
		return false
	}
	return first.CarryoverLimitMilliDays == nil ||
		*first.CarryoverLimitMilliDays == *second.CarryoverLimitMilliDays
}

func (service *Service) updateAttendanceLeaveGrantLotExpiries(
	ctx context.Context,
	employee attendanceLeaveEmployee,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
) error {
	database, errorValue := service.openAttendanceLeaveMutationDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	defer transaction.Rollback()
	rows, errorValue := transaction.QueryContext(ctx, `
SELECT id, granted_on
FROM attendance_leave_grant_lots
WHERE employee_email = ?
	AND leave_type_id = ?
	AND available_milli_days > 0`,
		normalizeAttendanceLeaveEmail(employee.Email),
		attendanceLeaveBalanceAccountID(leaveType.ID, leaveType.BalanceMode),
	)
	if errorValue != nil {
		return errorValue
	}
	type grantLot struct {
		ID        string
		GrantedOn string
	}
	lots := []grantLot{}
	for rows.Next() {
		var lot grantLot
		if errorValue := rows.Scan(&lot.ID, &lot.GrantedOn); errorValue != nil {
			rows.Close()
			return errorValue
		}
		lots = append(lots, lot)
	}
	if errorValue := rows.Err(); errorValue != nil {
		rows.Close()
		return errorValue
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for _, lot := range lots {
		grantDate, parseError := time.ParseInLocation(
			time.DateOnly,
			lot.GrantedOn,
			service.workspaceTimeZone().location,
		)
		if parseError != nil {
			return parseError
		}
		accrual, expiryError := attendanceLeaveAccrualWithExpiry(
			attendanceLeaveAccrual{GrantDate: lot.GrantedOn},
			grantDate,
			policy,
			leaveType,
		)
		if expiryError != nil {
			return expiryError
		}
		var expiresOn any
		if accrual.ExpiresOn != "" {
			expiresOn = accrual.ExpiresOn
		}
		if _, errorValue := transaction.ExecContext(ctx, `
UPDATE attendance_leave_grant_lots
SET expires_on = ?, updated_at = ?
WHERE id = ?`,
			expiresOn,
			updatedAt,
			lot.ID,
		); errorValue != nil {
			return errorValue
		}
	}
	return transaction.Commit()
}
