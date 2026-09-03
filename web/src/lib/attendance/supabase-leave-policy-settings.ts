import { invokeTool } from '$lib/public-api-call';
import type { AttendanceLeavePolicy } from '../../routes/admin/admin-types';

export function supabaseAttendanceLeavePolicy(): Promise<AttendanceLeavePolicy> {
	return invokeTool<AttendanceLeavePolicy>('attendance_leave_policy_get', {});
}

export function saveSupabaseAttendanceLeavePolicy(
	policy: AttendanceLeavePolicy
): Promise<AttendanceLeavePolicy> {
	return invokeTool<AttendanceLeavePolicy>('attendance_leave_policy_set', {
		balanceTrackingMode: policy.balanceTrackingMode,
		fiscalYearStartMonth: policy.fiscalYearStartMonth,
		fiscalYearStartDay: policy.fiscalYearStartDay,
		leaveTypes: policy.leaveTypes
	});
}
