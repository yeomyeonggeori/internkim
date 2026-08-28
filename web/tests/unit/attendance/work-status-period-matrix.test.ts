import { describe, expect, test } from 'bun:test';
import { parseSupabaseWorkPolicies } from '../../../src/lib/attendance/supabase-work-policy';
import {
	calculateSupabaseEmployeeWorkStatus,
	type SupabaseWorkStatusAttendance
} from '../../../src/lib/attendance/supabase-work-status';
import { shiftedDay } from '../../../src/lib/attendance/supabase-work-status-range';

const noHolidays = new Set<string>();

type WorkMode = 'autonomous' | 'flexible' | 'fixed';

const matrixMember = {
	id: 'matrix-member',
	name: '박예시',
	email: 'matrix@example.com',
	is_admin: false,
	user_id: 'matrix-user',
	joined_at: '2026-01-01T00:00:00Z'
};

function matrixPolicy(workMode: WorkMode) {
	const policies = parseSupabaseWorkPolicies([
		{
			member_id: matrixMember.id,
			work_hours: null,
			minimum_daily_minutes: 480,
			work_mode: workMode,
			work_policy: {
				workMode,
				workingWeekdays: [1, 2, 3, 4, 5],
				dailyTargetMinutes: 480,
				weeklyTargetMinutes: workMode === 'autonomous' ? 0 : 2400,
				referenceStartTime: '09:00',
				fixedStartTime: workMode === 'fixed' ? '09:00' : '',
				fixedEndTime: workMode === 'fixed' ? '18:00' : '',
				coreTimeEnabled: false,
				coreStartTime: '',
				coreEndTime: '',
				breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
				nightStartTime: '22:00',
				nightEndTime: '06:00'
			}
		}
	]);
	const policy = policies.get(matrixMember.id);
	if (!policy) throw new Error('expected matrix work policy fixture');
	return policy;
}

function daysOfPeriod(periodStart: string, periodEnd: string): string[] {
	const days: string[] = [];
	let day = periodStart;
	while (day <= periodEnd) {
		days.push(day);
		day = shiftedDay(day, 1);
	}
	return days;
}

function isWeekday(day: string): boolean {
	const weekday = new Date(`${day}T00:00:00Z`).getUTCDay();
	return weekday >= 1 && weekday <= 5;
}

function fullWorkdayEvents(days: string[]): SupabaseWorkStatusAttendance[] {
	return days
		.filter(isWeekday)
		.flatMap((day) => [
			{ member_id: matrixMember.id, kind: 'clock_in' as const, occurred_at: `${day}T00:00:00Z` },
			{ member_id: matrixMember.id, kind: 'clock_out' as const, occurred_at: `${day}T09:00:00Z` }
		]);
}

type MatrixCase = {
	period: 'day' | 'week' | 'month';
	workMode: WorkMode;
	periodStart: string;
	periodEnd: string;
	expectedActualMinutes: number;
	expectedTargetMinutes: number;
};

const matrixCases: MatrixCase[] = [
	{
		period: 'day',
		workMode: 'autonomous',
		periodStart: '2026-08-20',
		periodEnd: '2026-08-20',
		expectedActualMinutes: 480,
		expectedTargetMinutes: 0
	},
	{
		period: 'week',
		workMode: 'autonomous',
		periodStart: '2026-08-17',
		periodEnd: '2026-08-23',
		expectedActualMinutes: 2400,
		expectedTargetMinutes: 0
	},
	{
		period: 'month',
		workMode: 'autonomous',
		periodStart: '2026-08-01',
		periodEnd: '2026-08-31',
		expectedActualMinutes: 10080,
		expectedTargetMinutes: 0
	},
	{
		period: 'day',
		workMode: 'flexible',
		periodStart: '2026-08-20',
		periodEnd: '2026-08-20',
		expectedActualMinutes: 480,
		expectedTargetMinutes: 480
	},
	{
		period: 'week',
		workMode: 'flexible',
		periodStart: '2026-08-17',
		periodEnd: '2026-08-23',
		expectedActualMinutes: 2400,
		expectedTargetMinutes: 2400
	},
	{
		period: 'month',
		workMode: 'flexible',
		periodStart: '2026-08-01',
		periodEnd: '2026-08-31',
		expectedActualMinutes: 10080,
		expectedTargetMinutes: 10080
	},
	{
		period: 'day',
		workMode: 'fixed',
		periodStart: '2026-08-20',
		periodEnd: '2026-08-20',
		expectedActualMinutes: 480,
		expectedTargetMinutes: 480
	},
	{
		period: 'week',
		workMode: 'fixed',
		periodStart: '2026-08-17',
		periodEnd: '2026-08-23',
		expectedActualMinutes: 2400,
		expectedTargetMinutes: 2400
	},
	{
		period: 'month',
		workMode: 'fixed',
		periodStart: '2026-08-01',
		periodEnd: '2026-08-31',
		expectedActualMinutes: 10080,
		expectedTargetMinutes: 10080
	}
];

describe('calculateSupabaseEmployeeWorkStatus period x work-mode matrix', () => {
	for (const matrixCase of matrixCases) {
		test(`${matrixCase.workMode} ${matrixCase.period} reports actualMinutes ${matrixCase.expectedActualMinutes} and targetMinutes ${matrixCase.expectedTargetMinutes}`, () => {
			const days = daysOfPeriod(matrixCase.periodStart, matrixCase.periodEnd);
			const status = calculateSupabaseEmployeeWorkStatus({
				member: matrixMember,
				days,
				timeZone: 'Asia/Seoul',
				attendance: fullWorkdayEvents(days),
				leave: [],
				policy: matrixPolicy(matrixCase.workMode),
				holidays: noHolidays,
				now: new Date('2026-09-05T00:00:00Z')
			});

			expect(status.actualMinutes).toBe(matrixCase.expectedActualMinutes);
			expect(status.targetMinutes).toBe(matrixCase.expectedTargetMinutes);
		});
	}
});
