import type { SupabaseClient } from '@supabase/supabase-js';
import { isAttendanceWorkMode, type AttendanceWorkMode } from '$lib/attendance/work-mode';

export class InvalidAttendanceWorkModeError extends Error {
	constructor() {
		super('workMode must be autonomous, flexible, or fixed');
		this.name = 'InvalidAttendanceWorkModeError';
	}
}

export function attendanceWorkModeFromDevice(offered: unknown): AttendanceWorkMode | undefined {
	if (offered === undefined) return undefined;
	if (isAttendanceWorkMode(offered)) return offered;
	throw new InvalidAttendanceWorkModeError();
}

export async function saveAttendanceWorkMode(
	client: SupabaseClient,
	companyID: string,
	workMode: AttendanceWorkMode
): Promise<void> {
	const { error: failed } = await client
		.from('company')
		.update({ attendance_work_mode: workMode })
		.eq('id', companyID);
	if (failed) throw new Error(failed.message);
}
