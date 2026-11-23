import type {
	LeaveManagementAdjustment,
	LeaveManagementDaysCorrection,
	LeaveManagementPastLeave,
	LeaveManagementPayload,
	LeaveManagementTimeCorrection
} from './leave-management-types';
import {
	adjustSupabaseManagedLeave,
	cancelSupabaseManagedLeaveRequest,
	correctSupabaseManagedLeaveDays,
	correctSupabaseManagedLeaveTime,
	createSupabaseManagedPastLeave,
	supabaseLeaveManagement
} from '$lib/attendance/supabase-leave-management';

export function fetchLeaveManagement(employeeEmail = ''): Promise<LeaveManagementPayload> {
	return supabaseLeaveManagement(employeeEmail);
}

export function adjustManagedLeave(input: LeaveManagementAdjustment): Promise<void> {
	return adjustSupabaseManagedLeave(input);
}

export function createManagedPastLeave(input: LeaveManagementPastLeave): Promise<void> {
	return createSupabaseManagedPastLeave(input);
}

export function cancelManagedLeaveRequest(requestID: string): Promise<void> {
	return cancelSupabaseManagedLeaveRequest(requestID);
}

export function correctManagedLeaveDays(
	requestID: string,
	input: LeaveManagementDaysCorrection
): Promise<void> {
	return correctSupabaseManagedLeaveDays(requestID, input);
}

export function correctManagedLeaveTime(
	requestID: string,
	input: LeaveManagementTimeCorrection
): Promise<void> {
	return correctSupabaseManagedLeaveTime(requestID, input);
}
