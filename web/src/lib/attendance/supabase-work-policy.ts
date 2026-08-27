import type { SupabaseClient } from '@supabase/supabase-js';
import {
	currentAttendanceWorkPolicy,
	type CurrentAttendanceWorkPolicy
} from './current-work-policy';
import { isAttendanceWorkMode, type AttendanceWorkMode } from '$lib/attendance/work-mode';
import { defaultWorkPolicy } from './work-policy-defaults';
import type {
	ProjectedWorkCalendarDay,
	WorkCalendarProjection,
	WorkHoursCycle
} from '$lib/attendance/supabase-work-calendar';

export type SupabaseWorkPolicy = {
	memberID: string;
	workHours: WorkHoursCycle;
	minimumDailyMinutes: number | null;
	workMode: AttendanceWorkMode;
	workCalendar: WorkCalendarProjection;
	currentPolicy: CurrentAttendanceWorkPolicy;
	currentPolicyStored: boolean;
};

export async function supabaseWorkPolicies(
	client: SupabaseClient
): Promise<Map<string, SupabaseWorkPolicy>> {
	const response = await client.rpc('attendance_work_policies');
	if (response.error) throw new Error(response.error.message);
	return parseSupabaseWorkPolicies(response.data);
}

export function parseSupabaseWorkPolicies(value: unknown): Map<string, SupabaseWorkPolicy> {
	if (!Array.isArray(value)) throw new Error('attendance work policies must be an array');
	return new Map(value.map((row) => {
		if (typeof row !== 'object' || row === null) {
			throw new Error('attendance work policy must be an object');
		}
		if (!('member_id' in row) || typeof row.member_id !== 'string') {
			throw new Error('attendance work policy member_id must be a string');
		}
		if (
			!('minimum_daily_minutes' in row) ||
			(row.minimum_daily_minutes !== null && typeof row.minimum_daily_minutes !== 'number')
		) {
			throw new Error(`attendance work policy minimum is invalid for member ${row.member_id}`);
		}
		if (!('work_hours' in row)) {
			throw new Error(`attendance work hours are missing for member ${row.member_id}`);
		}
		if (!('work_mode' in row) || !isAttendanceWorkMode(row.work_mode)) {
			throw new Error(`attendance work mode is invalid for member ${row.member_id}`);
		}
			const currentPolicyStored = 'work_policy' in row && row.work_policy !== null;
			const currentPolicy =
				currentPolicyStored
					? currentAttendanceWorkPolicy(row.work_policy)
					: legacyCurrentPolicy(row.work_mode, row.minimum_daily_minutes);
		const policy: SupabaseWorkPolicy = {
			memberID: row.member_id,
			workHours: workHoursCycle(row.work_hours, row.member_id),
			minimumDailyMinutes: row.minimum_daily_minutes,
			workMode: currentPolicy.workMode,
			workCalendar: workCalendarProjection(
				'work_calendar' in row ? row.work_calendar : undefined,
				row.member_id
				),
				currentPolicy,
				currentPolicyStored
		};
		return [policy.memberID, policy];
	}));
}

function legacyCurrentPolicy(
	workMode: AttendanceWorkMode,
	minimumDailyMinutes: number | null
): CurrentAttendanceWorkPolicy {
	const fallback = defaultWorkPolicy();
	const dailyTargetMinutes = minimumDailyMinutes ?? fallback.dailyTargetMinutes;
	return {
		...fallback,
		workMode,
		dailyTargetMinutes,
		weeklyTargetMinutes: dailyTargetMinutes * fallback.workingWeekdays.length
	};
}

function workCalendarProjection(value: unknown, memberID: string): WorkCalendarProjection {
	if (value === null || value === undefined) return null;
	if (!Array.isArray(value) || !value.every(isProjectedWorkCalendarDay)) {
		throw new Error(`attendance work calendar is invalid for member ${memberID}`);
	}
	return value;
}

function isProjectedWorkCalendarDay(value: unknown): value is ProjectedWorkCalendarDay {
	if (typeof value !== 'object' || value === null) return false;
	if (!('date' in value) || typeof value.date !== 'string') return false;
	if (!('workMode' in value) || !isAttendanceWorkMode(value.workMode)) return false;
	if (!('workingDate' in value) || typeof value.workingDate !== 'boolean') return false;
	return !('holiday' in value) || typeof value.holiday === 'boolean';
}

function workHoursCycle(value: unknown, memberID: string): WorkHoursCycle {
	if (value === null) return null;
	if (!Array.isArray(value) || !value.every((week) => Array.isArray(week))) {
		throw new Error(`attendance work hours are invalid for member ${memberID}`);
	}
	return value;
}
