import { supabase } from '$lib/supabase';
import { companySettings, type CompanySettings } from '$lib/company/company-settings';
import {
	annualLeaveTypeID,
	defaultLeavePolicy,
	systemLeaveTypeKind
} from './leave-policy-defaults';
import type { AttendanceLeavePolicy, LeaveType } from '../../routes/admin/admin-types';

export async function supabaseAttendanceLeavePolicy(): Promise<AttendanceLeavePolicy> {
	return leavePolicyOf(await companySettings());
}

export async function saveSupabaseAttendanceLeavePolicy(
	policy: AttendanceLeavePolicy
): Promise<AttendanceLeavePolicy> {
	const saved = await supabase().rpc('attendance_leave_policy_save', {
		target_policy: policyPayload(policy)
	});
	if (saved.error) throw new Error(saved.error.message);
	return supabaseAttendanceLeavePolicy();
}

export function leavePolicyOf(settings: CompanySettings): AttendanceLeavePolicy {
	const stored = settings.rules.attendanceLeavePolicy ?? defaultLeavePolicy();
	return {
		...stored,
		balanceTrackingMode: settings.leaveDays === null ? 'unlimited' : 'managed',
		leaveTypes: stored.leaveTypes.map((leaveType) =>
			annualGrantOf(leaveType, settings.leaveDays)
		)
	};
}

function annualGrantOf(leaveType: LeaveType, leaveDays: number | null): LeaveType {
	if (leaveType.id !== annualLeaveTypeID || leaveDays === null) return leaveType;
	return { ...leaveType, grantAmountMilliDays: Math.round(leaveDays * 1000) };
}

function policyPayload(policy: AttendanceLeavePolicy): AttendanceLeavePolicy {
	for (const leaveType of policy.leaveTypes) {
		const systemKind = systemLeaveTypeKind(leaveType.id);
		if (systemKind === undefined && (leaveType.isSystem || leaveType.systemKind)) {
			throw new Error(`leave type ${leaveType.id || leaveType.name} cannot claim a system identity`);
		}
		if (systemKind !== undefined && leaveType.isSystem && leaveType.systemKind !== systemKind) {
			throw new Error(`the system leave type ${leaveType.id} cannot change its kind`);
		}
	}
	return { ...policy, version: 2 };
}
