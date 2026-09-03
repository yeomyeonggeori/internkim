import { answeredWorkPolicy, type AnsweredWorkPolicy } from './supabase-work-policy-settings';
import {
	currentAttendanceWorkPolicy,
	type CurrentAttendanceWorkPolicy
} from './current-work-policy';
import { isAttendanceWorkMode, type AttendanceWorkMode } from '$lib/attendance/work-mode';
import { defaultWorkPolicy } from './work-policy-defaults';
import type { AttendanceWorkPolicyRevision } from '$lib/attendance/work-calendar-derivation';
import { initialWorkPolicyEffectiveDate } from '$lib/attendance/work-policy-defaults';
import { storedWorkPolicyRevisions } from '$lib/attendance/stored-work-policy';
import type { WorkHoursCycle } from '$lib/attendance/supabase-work-calendar';

export type SupabaseWorkPolicy = {
	memberID: string;
	workHours: WorkHoursCycle;
	minimumDailyMinutes: number | null;
	workMode: AttendanceWorkMode;
	revisions: AttendanceWorkPolicyRevision[];
	currentPolicy: CurrentAttendanceWorkPolicy;
};

export async function supabaseWorkPolicies(): Promise<Map<string, SupabaseWorkPolicy>> {
	return workPoliciesByMember(await answeredWorkPolicy());
}

export function workPoliciesByMember(
	answered: AnsweredWorkPolicy
): Map<string, SupabaseWorkPolicy> {
	return parseSupabaseWorkPolicies(
		answered.people.map((person) => ({
			member_id: person.personID,
			work_hours: person.workHours,
			minimum_daily_minutes: person.minimumDailyMinutes,
			work_mode: answered.workMode,
			work_policy: answered.policy
		}))
	);
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
		const stored = 'work_policy' in row ? row.work_policy : null;
		const revisions =
			stored === null || stored === undefined
				? [
						{
							...legacyCurrentPolicy(row.work_mode, row.minimum_daily_minutes),
							effectiveDate: initialWorkPolicyEffectiveDate
						}
					]
				: storedWorkPolicyRevisions(stored, `member ${row.member_id}`);
		const currentPolicy = revisions[revisions.length - 1];
		const policy: SupabaseWorkPolicy = {
			memberID: row.member_id,
			workHours: workHoursCycle(row.work_hours, row.member_id),
			minimumDailyMinutes: row.minimum_daily_minutes,
			workMode: currentPolicy.workMode,
			revisions,
			currentPolicy
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

function workHoursCycle(value: unknown, memberID: string): WorkHoursCycle {
	if (value === null) return null;
	if (!Array.isArray(value) || !value.every((week) => Array.isArray(week))) {
		throw new Error(`attendance work hours are invalid for member ${memberID}`);
	}
	return value;
}
