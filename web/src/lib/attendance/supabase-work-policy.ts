import type { SupabaseClient } from '@supabase/supabase-js';
import { isAttendanceWorkMode, type AttendanceWorkMode } from '$lib/attendance/work-mode';
import type { WorkHoursCycle } from '$lib/attendance/supabase-work-calendar';

export type SupabaseWorkPolicy = {
	memberID: string;
	workHours: WorkHoursCycle;
	minimumDailyMinutes: number | null;
	workMode: AttendanceWorkMode;
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
		const policy: SupabaseWorkPolicy = {
			memberID: row.member_id,
			workHours: workHoursCycle(row.work_hours, row.member_id),
			minimumDailyMinutes: row.minimum_daily_minutes,
			workMode: row.work_mode
		};
		return [policy.memberID, policy];
	}));
}

function workHoursCycle(value: unknown, memberID: string): WorkHoursCycle {
	if (value === null) return null;
	if (!Array.isArray(value) || !value.every((week) => Array.isArray(week))) {
		throw new Error(`attendance work hours are invalid for member ${memberID}`);
	}
	return value;
}
