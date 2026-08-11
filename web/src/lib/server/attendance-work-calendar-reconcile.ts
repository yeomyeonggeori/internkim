import type { SupabaseClient } from '@supabase/supabase-js';
import { isAttendanceWorkMode, type AttendanceWorkMode } from '$lib/attendance/work-mode';

export type AttendanceWorkCalendarDay = {
	date: string;
	workMode: AttendanceWorkMode;
	workingDate: boolean;
};

export class InvalidAttendanceWorkModeError extends Error {
	constructor() {
		super('workMode must be autonomous, flexible, or fixed');
		this.name = 'InvalidAttendanceWorkModeError';
	}
}

export class InvalidAttendanceWorkCalendarError extends Error {
	constructor() {
		super('workCalendar must contain valid date, workMode, and workingDate entries');
		this.name = 'InvalidAttendanceWorkCalendarError';
	}
}

export function attendanceWorkModeFromDevice(offered: unknown): AttendanceWorkMode | undefined {
	if (offered === undefined) return undefined;
	if (isAttendanceWorkMode(offered)) return offered;
	throw new InvalidAttendanceWorkModeError();
}

export function attendanceWorkCalendarFromDevice(
	offered: unknown
): AttendanceWorkCalendarDay[] | undefined {
	if (offered === undefined) return undefined;
	if (!Array.isArray(offered) || !offered.every(isAttendanceWorkCalendarDay)) {
		throw new InvalidAttendanceWorkCalendarError();
	}
	return offered;
}

export async function saveAttendanceWorkCalendar(
	client: SupabaseClient,
	companyID: string,
	workCalendar: AttendanceWorkCalendarDay[]
): Promise<void> {
	const { error: failed } = await client.rpc('save_attendance_calendar', {
		target_company: companyID,
		attendance_calendar: workCalendar
	});
	if (failed) throw new Error(failed.message);
}

function isAttendanceWorkCalendarDay(offered: unknown): offered is AttendanceWorkCalendarDay {
	if (typeof offered !== 'object' || offered === null) return false;
	if (Object.keys(offered).length !== 3) return false;
	if (!('date' in offered) || !isDateOnly(offered.date)) return false;
	if (!('workMode' in offered) || !isAttendanceWorkMode(offered.workMode)) return false;
	return 'workingDate' in offered && typeof offered.workingDate === 'boolean';
}

function isDateOnly(offered: unknown): offered is string {
	if (typeof offered !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(offered)) return false;
	const date = new Date(`${offered}T00:00:00Z`);
	return !Number.isNaN(date.valueOf()) && date.toISOString().slice(0, 10) === offered;
}
