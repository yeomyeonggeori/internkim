import type { SupabaseClient } from '@supabase/supabase-js';
import {
	currentAttendanceWorkPolicy,
	type CurrentAttendanceWorkPolicy
} from '../attendance/current-work-policy';
import { isAttendanceWorkMode, type AttendanceWorkMode } from '$lib/attendance/work-mode';

const attendanceWorkCalendarDayMilliseconds = 24 * 60 * 60 * 1000;
const maximumAttendanceWorkCalendarDays = 31;

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

export function attendanceWorkCalendarFromDevice(
	offered: unknown,
	from: string,
	to: string
): AttendanceWorkCalendarDay[] | undefined {
	if (offered === undefined) return undefined;
	const expectedDates = attendanceWorkCalendarDates(from, to);
	if (!Array.isArray(offered) || !offered.every(isAttendanceWorkCalendarDay)) {
		throw new InvalidAttendanceWorkCalendarError();
	}
	if (
		offered.length !== expectedDates.length ||
		!offered.every((day, index) => day.date === expectedDates[index])
	) {
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

export async function saveAttendanceWorkPolicy(
	client: SupabaseClient,
	companyID: string,
	workPolicy: CurrentAttendanceWorkPolicy
): Promise<void> {
	const { error: failed } = await client.rpc('save_attendance_work_policy', {
		target_company: companyID,
		attendance_work_policy: workPolicy
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

function attendanceWorkCalendarDates(from: string, to: string): string[] {
	const startMoment = new Date(from);
	const endMoment = new Date(to);
	if (
		Number.isNaN(startMoment.valueOf()) ||
		Number.isNaN(endMoment.valueOf()) ||
		startMoment > endMoment
	) {
		throw new InvalidAttendanceWorkCalendarError();
	}
	if (startMoment.valueOf() === endMoment.valueOf()) return [];
	const date = new Date(
		Date.UTC(startMoment.getUTCFullYear(), startMoment.getUTCMonth(), startMoment.getUTCDate())
	);
	const calendarDaySpan = Math.ceil(
		(endMoment.valueOf() - date.valueOf()) / attendanceWorkCalendarDayMilliseconds
	);
	if (calendarDaySpan > maximumAttendanceWorkCalendarDays) {
		throw new InvalidAttendanceWorkCalendarError();
	}
	const dates: string[] = [];
	for (; date < endMoment; date.setUTCDate(date.getUTCDate() + 1)) {
		dates.push(date.toISOString().slice(0, 10));
	}
	return dates;
}
