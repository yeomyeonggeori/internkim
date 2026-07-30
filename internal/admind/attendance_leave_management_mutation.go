package admind

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (service *Service) adjustManagedAttendanceLeave(
	ctx context.Context,
	input attendanceLeaveManagementAdjustmentInput,
	now time.Time,
) (attendanceLeaveBalance, error) {
	input.EmployeeEmail = normalizeAttendanceLeaveEmail(input.EmployeeEmail)
	input.LeaveTypeID = strings.TrimSpace(input.LeaveTypeID)
	input.Kind = strings.TrimSpace(input.Kind)
	input.Reason = strings.TrimSpace(input.Reason)
	input.EffectiveOn = strings.TrimSpace(input.EffectiveOn)
	input.ExpiresOn = strings.TrimSpace(input.ExpiresOn)
	if input.EmployeeEmail == "" ||
		input.LeaveTypeID == "" ||
		input.AmountMilliDays == 0 {
		return attendanceLeaveBalance{}, attendanceLeaveInvalidInputErrorf(
			"employee, leave type, and amount are required",
		)
	}
	if input.Kind != attendanceLeaveOperationAdjustment &&
		input.Kind != attendanceLeaveOperationLegalCorrection {
		return attendanceLeaveBalance{}, attendanceLeaveInvalidInputErrorf(
			"unsupported leave adjustment kind",
		)
	}
	if input.EffectiveOn == "" {
		input.EffectiveOn = now.In(service.workspaceTimeZone().location).Format(time.DateOnly)
	}
	if errorValue := validateOptionalAttendanceLeaveDate(input.EffectiveOn); errorValue != nil {
		return attendanceLeaveBalance{}, attendanceLeaveInvalidInputError(errorValue)
	}
	if errorValue := validateOptionalAttendanceLeaveDate(input.ExpiresOn); errorValue != nil {
		return attendanceLeaveBalance{}, attendanceLeaveInvalidInputError(errorValue)
	}
	service.attendanceLeavePolicyMutationMutex.Lock()
	defer service.attendanceLeavePolicyMutationMutex.Unlock()
	policy, errorValue := service.readAttendanceLeavePolicy(ctx)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	if policy.BalanceTrackingMode == attendanceLeaveBalanceTrackingUnlimited {
		return attendanceLeaveBalance{}, attendanceLeaveInvalidInputErrorf(
			"leave balances are not managed in unlimited mode",
		)
	}
	leaveType, found := attendanceLeaveTypeByID(policy, input.LeaveTypeID)
	if !found || leaveType.BalanceMode == "none" {
		return attendanceLeaveBalance{}, attendanceLeaveInvalidInputErrorf(
			"leave type does not use a balance",
		)
	}
	token, errorValue := generateRandomURLToken(12)
	if errorValue != nil {
		return attendanceLeaveBalance{}, errorValue
	}
	operation := attendanceLeaveOperation{
		OperationKey: "admin-leave-adjustment:" + token,
		Employee:     attendanceLeaveEmployee{Email: input.EmployeeEmail},
		LeaveTypeID:  attendanceLeaveBalanceAccountID(leaveType.ID, leaveType.BalanceMode),
		Kind:         input.Kind,
		ReferenceID:  input.Reason,
		EffectiveOn:  input.EffectiveOn,
	}
	if input.AmountMilliDays > 0 {
		operation.Amount = input.AmountMilliDays
		return service.grantAttendanceLeave(
			ctx,
			attendanceLeaveGrant{Operation: operation, ExpiresOn: input.ExpiresOn},
		)
	}
	operation.Amount = -input.AmountMilliDays
	return service.removeAvailableAttendanceLeave(ctx, operation, input.Kind)
}

func (service *Service) createManagedPastAttendanceLeave(
	ctx context.Context,
	input attendanceLeaveManagementPastLeaveInput,
	administratorEmail string,
	now time.Time,
) (attendanceLeaveApprovalRequestView, error) {
	employeeEmail := normalizeAttendanceLeaveEmail(input.EmployeeEmail)
	requestInput := normalizeAttendanceLeaveRequestInput(attendanceLeaveRequestInput{
		LeaveTypeID:   input.LeaveTypeID,
		Unit:          input.Unit,
		StartDate:     input.StartDate,
		EndDate:       input.EndDate,
		PartialPeriod: input.PartialPeriod,
		StartTime:     input.StartTime,
		Reason:        input.Reason,
	})
	if employeeEmail == "" {
		return attendanceLeaveApprovalRequestView{}, attendanceLeaveInvalidInputErrorf(
			"employee is required",
		)
	}
	timeZone := service.workspaceTimeZone()
	startDate, errorValue := attendanceWorkScheduleDate(requestInput.StartDate, timeZone.location)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, attendanceLeaveInvalidInputError(errorValue)
	}
	endDateValue := requestInput.EndDate
	if endDateValue == "" {
		endDateValue = requestInput.StartDate
	}
	endDate, errorValue := attendanceWorkScheduleDate(endDateValue, timeZone.location)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, attendanceLeaveInvalidInputError(errorValue)
	}
	today := now.In(timeZone.location)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, timeZone.location)
	if startDate.After(today) || endDate.After(today) {
		return attendanceLeaveApprovalRequestView{}, attendanceLeaveInvalidInputErrorf(
			"past leave date must not be in the future",
		)
	}
	service.attendanceLeavePolicyMutationMutex.Lock()
	defer service.attendanceLeavePolicyMutationMutex.Unlock()
	preview, errorValue := service.previewAttendanceLeaveRequestAllowPast(ctx, requestInput, now)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	policy, errorValue := service.readAttendanceLeavePolicy(ctx)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	leaveType, found := attendanceLeaveTypeByID(policy, requestInput.LeaveTypeID)
	if !found || !leaveType.IsActive {
		return attendanceLeaveApprovalRequestView{}, attendanceLeaveInvalidInputErrorf(
			"leave type is not active",
		)
	}
	leaveType = attendanceLeaveTypeForRequest(policy, leaveType)
	record, errorValue := service.createAttendanceLeaveRequest(
		ctx,
		attendanceLeaveEmployee{Email: employeeEmail},
		requestInput,
		preview,
		leaveType,
		nil,
		now,
	)
	if errorValue != nil {
		return attendanceLeaveApprovalRequestView{}, errorValue
	}
	view, errorValue := service.decideAttendanceLeaveRequest(
		ctx,
		record.ID,
		administratorEmail,
		attendanceLeaveApprovalInput{Action: attendanceLeaveApprovalActionApprove},
		now,
	)
	if errorValue == nil {
		return view, nil
	}
	cancelError := service.cancelAttendanceLeaveRequest(
		ctx,
		record.ID,
		attendanceLeaveEmployee{Email: employeeEmail},
		now,
	)
	if cancelError != nil {
		return attendanceLeaveApprovalRequestView{}, fmt.Errorf(
			"approve managed past leave: %w; cleanup pending request: %v",
			errorValue,
			cancelError,
		)
	}
	return attendanceLeaveApprovalRequestView{}, errorValue
}

func (service *Service) cancelManagedAttendanceLeave(
	ctx context.Context,
	requestID string,
	employeeEmail string,
	administratorEmail string,
	now time.Time,
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
	record, errorValue := readAttendanceLeaveRequestRecordInTransaction(
		ctx,
		transaction,
		requestID,
		employeeEmail,
	)
	if errorValue != nil {
		return errorValue
	}
	if record.Status != attendanceLeaveRequestStatusApproved {
		return errAttendanceLeaveRequestCannotCancel
	}
	if errorValue := service.cancelApprovedAttendanceLeaveRequestInTransaction(
		ctx,
		transaction,
		record,
		attendanceLeaveEmployee{Email: record.EmployeeEmail, UserID: record.UserID},
		administratorEmail,
		now,
		true,
	); errorValue != nil {
		return errorValue
	}
	return transaction.Commit()
}
