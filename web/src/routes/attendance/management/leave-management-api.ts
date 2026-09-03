import type {
	LeaveManagementAdjustment,
	LeaveManagementPastLeave,
	LeaveManagementPayload,
	LeaveManagementTimeCorrection
} from './leave-management-types';
import {
	adjustSupabaseManagedLeave,
	cancelSupabaseManagedLeaveRequest,
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

export function correctManagedLeaveTime(
	requestID: string,
	input: LeaveManagementTimeCorrection
): Promise<void> {
	return correctSupabaseManagedLeaveTime(requestID, input);
}
