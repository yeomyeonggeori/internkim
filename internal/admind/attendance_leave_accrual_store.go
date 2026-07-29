package admind

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (service *Service) synchronizeAttendanceLeaveAccruals(
	ctx context.Context,
	employee attendanceLeaveEmployee,
	policy attendanceLeavePolicy,
	now time.Time,
) error {
	hireDateValue := strings.TrimSpace(employee.HireDate)
	if hireDateValue == "" {
		return nil
	}
	hireDate, errorValue := time.ParseInLocation(
		time.DateOnly,
		hireDateValue,
		service.workspaceTimeZone().location,
	)
	if errorValue != nil {
		return fmt.Errorf("invalid employee hire date %q: %w", hireDateValue, errorValue)
	}
	asOf := attendanceLeaveCalendarDate(
		now.In(hireDate.Location()).Year(),
		now.In(hireDate.Location()).Month(),
		now.In(hireDate.Location()).Day(),
		hireDate.Location(),
	)
	if hireDate.After(asOf) {
		return nil
	}
	automaticOperationKeys, errorValue := service.attendanceLeaveAutomaticOperationKeys(
		ctx,
		employee.Email,
	)
	if errorValue != nil {
		return errorValue
	}
	accrualPolicy := policy
	if len(automaticOperationKeys) == 0 {
		accrualPolicy.UpdatedAt = ""
	}
	operationEmployee := attendanceLeaveEmployee{Email: employee.Email}
	for _, leaveType := range policy.LeaveTypes {
		if leaveType.BalanceMode == "none" {
			continue
		}
		if leaveType.IsActive &&
			leaveType.GrantCadence != "manual" &&
			leaveType.GrantCadence != "none" {
			accruals, accrualError := attendanceLeaveAccrualsThrough(
				hireDate,
				asOf,
				accrualPolicy,
				leaveType,
			)
			if accrualError != nil {
				return accrualError
			}
			for _, accrual := range accruals {
				operationKey := attendanceLeaveAutomaticOperationKey(
					accrual,
					leaveType.ID,
					employee.Email,
				)
				if _, recorded := automaticOperationKeys[operationKey]; recorded {
					continue
				}
				if _, errorValue := service.grantAttendanceLeave(ctx, attendanceLeaveGrant{
					Operation: attendanceLeaveOperation{
						OperationKey: operationKey,
						Employee:     operationEmployee,
						LeaveTypeID:  leaveType.ID,
						Kind:         accrual.Kind,
						ReferenceID:  accrual.ReferenceID,
						Amount:       accrual.AmountMilliDays,
						EffectiveOn:  accrual.GrantDate,
					},
					ExpiresOn: accrual.ExpiresOn,
				}); errorValue != nil {
					return errorValue
				}
				automaticOperationKeys[operationKey] = struct{}{}
			}
		}
		if errorValue := service.synchronizeAttendanceLeaveExpiries(
			ctx,
			operationEmployee,
			policy,
			leaveType,
			asOf,
		); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func attendanceLeaveAutomaticOperationKey(
	accrual attendanceLeaveAccrual,
	leaveTypeID string,
	employeeEmail string,
) string {
	prefix := "automatic-leave"
	switch accrual.ReferenceID {
	case "automatic:statutory:fiscal":
		prefix = "automatic-leave-fiscal"
	case "automatic:statutory:legal-correction":
		prefix = "automatic-leave-legal-correction"
	}
	return fmt.Sprintf(
		"%s:%s:%s:%s",
		prefix,
		strings.TrimSpace(leaveTypeID),
		normalizeAttendanceLeaveEmail(employeeEmail),
		accrual.GrantDate,
	)
}

func (service *Service) attendanceLeaveAutomaticOperationKeys(
	ctx context.Context,
	employeeEmail string,
) (map[string]struct{}, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(
		ctx,
		`SELECT operation_key
FROM attendance_leave_operations
WHERE employee_email = ?
	AND operation_key LIKE 'automatic-leave%'`,
		normalizeAttendanceLeaveEmail(employeeEmail),
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	operationKeys := map[string]struct{}{}
	for rows.Next() {
		var operationKey string
		if errorValue := rows.Scan(&operationKey); errorValue != nil {
			return nil, errorValue
		}
		operationKeys[operationKey] = struct{}{}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return operationKeys, nil
}

func (service *Service) synchronizeAttendanceLeaveExpiries(
	ctx context.Context,
	employee attendanceLeaveEmployee,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
	asOf time.Time,
) error {
	expiryDates, errorValue := service.attendanceLeaveOutstandingExpiryDates(
		ctx,
		employee,
		leaveType.ID,
		asOf.Format(time.DateOnly),
	)
	if errorValue != nil {
		return errorValue
	}
	for _, expiryDate := range expiryDates {
		operation := attendanceLeaveOperation{
			OperationKey: fmt.Sprintf(
				"automatic-leave-expiry:%s:%s:%s",
				leaveType.ID,
				normalizeAttendanceLeaveEmail(employee.Email),
				expiryDate,
			),
			Employee:    employee,
			LeaveTypeID: leaveType.ID,
			ReferenceID: "automatic:expiry",
			EffectiveOn: expiryDate,
		}
		if leaveType.CarryoverEnabled {
			nextExpiry := ""
			if leaveType.ExpiryMode == "fiscalYearEnd" {
				expiry, parseError := time.ParseInLocation(time.DateOnly, expiryDate, asOf.Location())
				if parseError != nil {
					return parseError
				}
				nextExpiry = attendanceLeaveNextFiscalYearBoundary(
					expiry,
					policy.FiscalYearStartMonth,
					policy.FiscalYearStartDay,
				).Format(time.DateOnly)
			} else if leaveType.ExpiryMode == "monthsAfterGrant" && leaveType.ExpiryMonths != nil {
				expiry, parseError := time.ParseInLocation(time.DateOnly, expiryDate, asOf.Location())
				if parseError != nil {
					return parseError
				}
				nextExpiry = attendanceLeaveCalendarAnniversary(
					expiry,
					0,
					*leaveType.ExpiryMonths,
				).Format(time.DateOnly)
			}
			if _, errorValue := service.carryoverAttendanceLeave(
				ctx,
				operation,
				leaveType.CarryoverLimitMilliDays,
				nextExpiry,
			); errorValue != nil {
				return errorValue
			}
			continue
		}
		if _, errorValue := service.expireAttendanceLeave(ctx, operation); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

func (service *Service) attendanceLeaveOutstandingExpiryDates(
	ctx context.Context,
	employee attendanceLeaveEmployee,
	leaveTypeID string,
	asOf string,
) ([]string, error) {
	database, errorValue := service.openAttendanceDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT DISTINCT expires_on
FROM attendance_leave_grant_lots
WHERE employee_email = ?
	AND leave_type_id = ?
	AND expires_on IS NOT NULL
	AND expires_on <= ?
	AND available_milli_days > 0`,
		normalizeAttendanceLeaveEmail(employee.Email),
		strings.TrimSpace(leaveTypeID),
		asOf,
	)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	expiryDates := []string{}
	for rows.Next() {
		var expiryDate sql.NullString
		if errorValue := rows.Scan(&expiryDate); errorValue != nil {
			return nil, errorValue
		}
		if expiryDate.Valid {
			expiryDates = append(expiryDates, expiryDate.String)
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	sort.Strings(expiryDates)
	return expiryDates, nil
}
