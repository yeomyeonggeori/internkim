package admind

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var errAttendanceLeavePolicyAdjustmentConflict = errors.New("leave policy adjustment conflict")

func (service *Service) synchronizeAttendanceLeavePolicyAccrualsBeforeUpdate(
	request *http.Request,
	existing attendanceLeavePolicy,
	updated attendanceLeavePolicy,
	now time.Time,
) error {
	records, found := service.attendanceUserRecordsForMembers(request)
	if !found {
		return fmt.Errorf("attendance employees are unavailable")
	}
	employees := make([]attendanceLeaveEmployee, 0, len(records))
	for _, member := range attendanceMembersFromAdminUserRecords(records) {
		employee, employeeError := service.attendanceLeaveEmployeeWithHireDateForRequest(
			request,
			member.Email,
		)
		if employeeError != nil {
			return employeeError
		}
		if errorValue := service.synchronizeAttendanceLeaveAccruals(
			request.Context(),
			employee,
			existing,
			now,
		); errorValue != nil {
			return errorValue
		}
		employees = append(employees, employee)
	}
	adjustments, errorValue := service.attendanceLeavePolicyAdjustments(
		request,
		employees,
		existing,
		updated,
		now,
	)
	if errorValue != nil {
		return errorValue
	}
	for _, adjustment := range adjustments {
		if _, errorValue := service.adjustAttendanceLeave(
			request.Context(),
			adjustment.Operation,
			adjustment.AmountMilliDays,
			adjustment.ExpiresOn,
		); errorValue != nil {
			return errorValue
		}
	}
	return service.updateAttendanceLeavePolicyExpiries(
		request.Context(),
		employees,
		existing,
		updated,
	)
}

type attendanceLeavePolicyAdjustment struct {
	Operation       attendanceLeaveOperation
	AmountMilliDays int
	ExpiresOn       string
}

func (service *Service) attendanceLeavePolicyAdjustments(
	request *http.Request,
	employees []attendanceLeaveEmployee,
	existing attendanceLeavePolicy,
	updated attendanceLeavePolicy,
	now time.Time,
) ([]attendanceLeavePolicyAdjustment, error) {
	existingByID := make(map[string]attendanceLeaveType, len(existing.LeaveTypes))
	for _, leaveType := range existing.LeaveTypes {
		existingByID[leaveType.ID] = leaveType
	}
	adjustments := []attendanceLeavePolicyAdjustment{}
	for _, employee := range employees {
		for _, updatedType := range updated.LeaveTypes {
			adjustment, required, errorValue := service.attendanceLeavePolicyAdjustment(
				request,
				employee,
				existing,
				existingByID[updatedType.ID],
				updated,
				updatedType,
				now,
			)
			if errorValue != nil {
				return nil, errorValue
			}
			if required {
				adjustments = append(adjustments, adjustment)
			}
		}
	}
	return adjustments, nil
}

func (service *Service) attendanceLeavePolicyAdjustment(
	request *http.Request,
	employee attendanceLeaveEmployee,
	existingPolicy attendanceLeavePolicy,
	existingType attendanceLeaveType,
	updatedPolicy attendanceLeavePolicy,
	updatedType attendanceLeaveType,
	now time.Time,
) (attendanceLeavePolicyAdjustment, bool, error) {
	if !updatedType.IsActive {
		return attendanceLeavePolicyAdjustment{}, false, nil
	}
	policyNow := now.In(service.workspaceTimeZone().location)
	updatedTarget, updatedGrantDate, errorValue := attendanceLeaveCurrentGrantTarget(
		employee,
		updatedPolicy,
		updatedType,
		policyNow,
	)
	if errorValue != nil {
		return attendanceLeavePolicyAdjustment{}, false, errorValue
	}
	if updatedType.GrantCadence == "none" {
		return attendanceLeavePolicyAdjustment{}, false, nil
	}
	existingTarget := 0
	if existingType.ID != "" &&
		attendanceLeaveBalanceAccountID(existingType.ID, existingType.BalanceMode) ==
			attendanceLeaveBalanceAccountID(updatedType.ID, updatedType.BalanceMode) {
		existingTarget, _, errorValue = attendanceLeaveCurrentGrantTarget(
			employee,
			existingPolicy,
			existingType,
			policyNow,
		)
		if errorValue != nil {
			return attendanceLeavePolicyAdjustment{}, false, errorValue
		}
	}
	delta := updatedTarget - existingTarget
	if delta == 0 {
		return attendanceLeavePolicyAdjustment{}, false, nil
	}
	accountID := attendanceLeaveBalanceAccountID(updatedType.ID, updatedType.BalanceMode)
	if delta < 0 {
		balance, balanceError := service.readAttendanceLeaveBalance(
			request.Context(),
			employee,
			accountID,
		)
		if balanceError != nil {
			return attendanceLeavePolicyAdjustment{}, false, balanceError
		}
		if balance.AvailableMilliDays < -delta {
			return attendanceLeavePolicyAdjustment{}, false, fmt.Errorf(
				"%w: cannot reduce %s balance below used or reserved leave for %s",
				errAttendanceLeavePolicyAdjustmentConflict,
				updatedType.Name,
				employee.Email,
			)
		}
	}
	expiresOn := ""
	if delta > 0 {
		grantDate, parseError := time.ParseInLocation(
			time.DateOnly,
			updatedGrantDate,
			service.workspaceTimeZone().location,
		)
		if parseError != nil {
			return attendanceLeavePolicyAdjustment{}, false, parseError
		}
		accrual, expiryError := attendanceLeaveAccrualWithExpiry(
			attendanceLeaveAccrual{GrantDate: updatedGrantDate},
			grantDate,
			updatedPolicy,
			updatedType,
		)
		if expiryError != nil {
			return attendanceLeavePolicyAdjustment{}, false, expiryError
		}
		expiresOn = accrual.ExpiresOn
	}
	return attendanceLeavePolicyAdjustment{
		Operation: attendanceLeaveOperation{
			OperationKey: fmt.Sprintf(
				"policy-leave-adjustment:%s:%s:%s",
				strings.TrimSpace(updatedPolicy.UpdatedAt),
				strings.TrimSpace(accountID),
				normalizeAttendanceLeaveEmail(employee.Email),
			),
			Employee:    employee,
			LeaveTypeID: accountID,
			ReferenceID: "policy:update",
			EffectiveOn: policyNow.Format(time.DateOnly),
		},
		AmountMilliDays: delta,
		ExpiresOn:       expiresOn,
	}, true, nil
}

func attendanceLeaveCurrentGrantTarget(
	employee attendanceLeaveEmployee,
	policy attendanceLeavePolicy,
	leaveType attendanceLeaveType,
	now time.Time,
) (int, string, error) {
	if !leaveType.IsActive ||
		!attendanceLeaveTypeOwnsBalance(leaveType) ||
		leaveType.GrantCadence == "none" {
		return 0, "", nil
	}
	location := now.Location()
	hireDate, errorValue := time.ParseInLocation(
		time.DateOnly,
		strings.TrimSpace(employee.HireDate),
		location,
	)
	if errorValue != nil {
		if strings.TrimSpace(employee.HireDate) == "" {
			return 0, "", nil
		}
		return 0, "", errorValue
	}
	asOf := attendanceLeaveCalendarDate(
		now.In(location).Year(),
		now.In(location).Month(),
		now.In(location).Day(),
		location,
	)
	if hireDate.After(asOf) {
		return 0, "", nil
	}
	switch leaveType.GrantCadence {
	case "annual":
		fiscalStart, fiscalEnd, boundsError := attendanceLeaveFiscalYearBounds(
			asOf,
			policy.FiscalYearStartMonth,
			policy.FiscalYearStartDay,
		)
		if boundsError != nil {
			return 0, "", boundsError
		}
		grantDate := fiscalStart
		amount := leaveType.GrantAmountMilliDays
		if hireDate.After(fiscalStart) {
			grantDate = hireDate
			amount = attendanceLeaveProportionalGrantMilliDays(
				hireDate,
				fiscalStart,
				fiscalEnd,
				leaveType.GrantAmountMilliDays,
			)
		}
		return amount, grantDate.Format(time.DateOnly), nil
	case "monthly":
		fiscalStart, _, boundsError := attendanceLeaveFiscalYearBounds(
			asOf,
			policy.FiscalYearStartMonth,
			policy.FiscalYearStartDay,
		)
		if boundsError != nil {
			return 0, "", boundsError
		}
		count := 0
		lastGrantDate := time.Time{}
		for completedMonths := 1; ; completedMonths++ {
			grantDate := attendanceLeaveCalendarAnniversary(hireDate, 0, completedMonths)
			if grantDate.After(asOf) {
				break
			}
			if grantDate.Before(fiscalStart) {
				continue
			}
			count++
			lastGrantDate = grantDate
		}
		if count == 0 {
			return 0, "", nil
		}
		return count * leaveType.GrantAmountMilliDays, lastGrantDate.Format(time.DateOnly), nil
	default:
		return 0, "", fmt.Errorf("unsupported automatic leave cadence %q", leaveType.GrantCadence)
	}
}
