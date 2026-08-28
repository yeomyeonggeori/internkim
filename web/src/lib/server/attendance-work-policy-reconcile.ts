import type { SupabaseClient } from '@supabase/supabase-js';
import {
	currentAttendanceWorkPolicy,
	type CurrentAttendanceWorkPolicy
} from '../attendance/current-work-policy';
import { isAttendanceWorkMode, type AttendanceWorkMode } from '$lib/attendance/work-mode';

export class InvalidAttendanceWorkModeError extends Error {
	constructor() {
		super('workMode must be autonomous, flexible, or fixed');
		this.name = 'InvalidAttendanceWorkModeError';
	}
}

export class InvalidAttendanceWorkPolicyError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'InvalidAttendanceWorkPolicyError';
	}
}

export function attendanceWorkPolicyFromDevice(
	offered: unknown
): CurrentAttendanceWorkPolicy | undefined {
	if (offered === undefined) return undefined;
	try {
		return currentAttendanceWorkPolicy(offered);
	} catch (thrown) {
		throw new InvalidAttendanceWorkPolicyError(
			thrown instanceof Error ? thrown.message : 'workPolicy is invalid'
		);
	}
}

export function attendanceWorkModeFromDevice(offered: unknown): AttendanceWorkMode | undefined {
	if (offered === undefined) return undefined;
	if (isAttendanceWorkMode(offered)) return offered;
	throw new InvalidAttendanceWorkModeError();
}

export async function saveAttendanceReconciliationSettings(
	client: SupabaseClient,
	companyID: string,
	workPolicy: CurrentAttendanceWorkPolicy | undefined
): Promise<void> {
	const { error: failed } = await client.rpc('attendance_reconciliation_save', {
		target_company: companyID,
		attendance_work_policy: workPolicy ?? null,
		attendance_calendar: null
	});
	if (failed) throw new Error(failed.message);
}
