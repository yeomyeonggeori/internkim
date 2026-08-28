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

export type ReconciledCompanyHoliday = {
	id: string;
	title: string;
	date: string;
	recursAnnually: boolean;
};

export class InvalidCompanyHolidayError extends Error {
	constructor() {
		super('companyHolidays must carry an id, a title, a YYYY-MM-DD date and a recursAnnually flag');
		this.name = 'InvalidCompanyHolidayError';
	}
}

export function companyHolidaysFromDevice(offered: unknown): ReconciledCompanyHoliday[] | undefined {
	if (offered === undefined) return undefined;
	if (!Array.isArray(offered) || !offered.every(isReconciledCompanyHoliday)) {
		throw new InvalidCompanyHolidayError();
	}
	return offered;
}

export async function mergeCompanyHolidays(
	client: SupabaseClient,
	companyID: string,
	holidays: ReconciledCompanyHoliday[]
): Promise<void> {
	if (holidays.length === 0) return;
	const { error: failed } = await client.rpc('company_holidays_merge', {
		target_company: companyID,
		target_holidays: holidays
	});
	if (failed) throw new Error(failed.message);
}

function isReconciledCompanyHoliday(offered: unknown): offered is ReconciledCompanyHoliday {
	if (typeof offered !== 'object' || offered === null) return false;
	const holiday = offered as Record<string, unknown>;
	if (typeof holiday.id !== 'string' || holiday.id.trim() === '') return false;
	if (typeof holiday.title !== 'string' || holiday.title.trim() === '') return false;
	if (typeof holiday.date !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(holiday.date)) return false;
	const parsed = new Date(`${holiday.date}T00:00:00Z`);
	if (Number.isNaN(parsed.valueOf()) || parsed.toISOString().slice(0, 10) !== holiday.date) return false;
	return typeof holiday.recursAnnually === 'boolean';
}
