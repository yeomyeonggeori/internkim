import {
	saveSupabaseAttendanceLeavePolicy,
	supabaseAttendanceLeavePolicy
} from '$lib/attendance/supabase-leave-policy-settings';
import {
	saveSupabaseAttendanceWorkPolicy,
	supabaseAttendanceWorkPolicy
} from '$lib/attendance/supabase-work-policy-settings';
import type {
	AttendanceLeavePolicy,
	AttendanceWorkPolicyResponse,
	AttendanceWorkPolicyRevision
} from './admin-types';

export function fetchAttendanceWorkPolicy(): Promise<AttendanceWorkPolicyResponse> {
	return supabaseAttendanceWorkPolicy();
}

export function updateAttendanceWorkPolicy(
	revision: AttendanceWorkPolicyRevision
): Promise<AttendanceWorkPolicyResponse> {
	return saveSupabaseAttendanceWorkPolicy(revision);
}

export function fetchAttendanceLeavePolicy(): Promise<AttendanceLeavePolicy> {
	return supabaseAttendanceLeavePolicy();
}

export function updateAttendanceLeavePolicy(
	policy: AttendanceLeavePolicy
): Promise<AttendanceLeavePolicy> {
	return saveSupabaseAttendanceLeavePolicy(policy);
}
