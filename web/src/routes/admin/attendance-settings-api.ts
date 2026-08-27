import { isSupabaseConfigured } from '$lib/supabase';
import {
	saveSupabaseAttendanceLeavePolicy,
	supabaseAttendanceLeavePolicy
} from '$lib/attendance/supabase-leave-policy-settings';
import {
	saveSupabaseAttendanceWorkPolicy,
	supabaseAttendanceWorkPolicy
} from '$lib/attendance/supabase-work-policy-settings';
import * as deviceAdminAPI from './admin-api';
import type {
	AttendanceLeavePolicy,
	AttendanceWorkPolicyResponse,
	AttendanceWorkPolicyRevision
} from './admin-types';

export async function fetchAttendanceWorkPolicy(
	adminBaseURL: string,
	fallbackMessage: string
): Promise<AttendanceWorkPolicyResponse> {
	if (isSupabaseConfigured()) return supabaseAttendanceWorkPolicy();
	return deviceAdminAPI.fetchAttendanceWorkPolicy(adminBaseURL, fallbackMessage);
}

export async function updateAttendanceWorkPolicy(
	adminBaseURL: string,
	revision: AttendanceWorkPolicyRevision,
	fallbackMessage: string
): Promise<AttendanceWorkPolicyResponse> {
	if (isSupabaseConfigured()) return saveSupabaseAttendanceWorkPolicy(revision);
	return deviceAdminAPI.updateAttendanceWorkPolicy(adminBaseURL, revision, fallbackMessage);
}

export async function fetchAttendanceLeavePolicy(
	adminBaseURL: string,
	fallbackMessage: string
): Promise<AttendanceLeavePolicy> {
	if (isSupabaseConfigured()) return supabaseAttendanceLeavePolicy();
	return deviceAdminAPI.fetchAttendanceLeavePolicy(adminBaseURL, fallbackMessage);
}

export async function updateAttendanceLeavePolicy(
	adminBaseURL: string,
	policy: AttendanceLeavePolicy,
	fallbackMessage: string
): Promise<AttendanceLeavePolicy> {
	if (isSupabaseConfigured()) return saveSupabaseAttendanceLeavePolicy(policy);
	return deviceAdminAPI.updateAttendanceLeavePolicy(adminBaseURL, policy, fallbackMessage);
}
