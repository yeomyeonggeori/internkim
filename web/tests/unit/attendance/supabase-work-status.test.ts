import { describe, expect, test } from 'bun:test';
import { parseSupabaseWorkPolicies } from '../../../src/lib/attendance/supabase-work-policy';
import { supabaseWorkStatusTimeRange } from '../../../src/lib/attendance/supabase-work-status-range';
import { calculateSupabaseEmployeeWorkStatus } from '../../../src/lib/attendance/supabase-work-status';

const secondsPerDay = 24 * 60 * 60;
const weekdayWorkHours = [[[], [], [], [], [], null, null]];

const member = {
	id: 'member-one',
	name: '이샘플',
	email: 'sample@example.com',
	is_admin: false,
	user_id: 'user-one',
	joined_at: '2026-01-01T00:00:00Z'
};

const noHolidays = new Set<string>();

function revisionOf(
	effectiveDate: string,
	workMode: 'autonomous' | 'flexible' | 'fixed',
	overrides: Record<string, unknown> = {}
) {
	return {
		effectiveDate,
		workMode,
		workingWeekdays: [1, 2, 3, 4, 5],
		dailyTargetMinutes: workMode === 'autonomous' ? 0 : 480,
		weeklyTargetMinutes: workMode === 'autonomous' ? 0 : 2400,
		referenceStartTime: '09:00',
		fixedStartTime: workMode === 'fixed' ? '09:00' : '',
		fixedEndTime: workMode === 'fixed' ? '18:00' : '',
		coreTimeEnabled: workMode === 'flexible',
		coreStartTime: workMode === 'flexible' ? '11:00' : '',
		coreEndTime: workMode === 'flexible' ? '16:00' : '',
		breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
		nightStartTime: '22:00',
		nightEndTime: '06:00',
		...overrides
	};
}

function policyOfRevisions(revisions: Record<string, unknown>[]) {
	const policies = parseSupabaseWorkPolicies([
		{
			member_id: member.id,
			work_hours: weekdayWorkHours,
			minimum_daily_minutes: 480,
			work_mode: revisions[revisions.length - 1].workMode,
			work_policy: { version: 1, revisions }
		}
	]);
	const policy = policies.get(member.id);
	if (!policy) throw new Error('expected work policy fixture');
	return policy;
}

function policyFrom(
	workMode: 'autonomous' | 'flexible' | 'fixed' = 'autonomous',
	overrides: Record<string, unknown> = {}
) {
	const policies = parseSupabaseWorkPolicies([
		{
			member_id: member.id,
			work_hours: weekdayWorkHours,
			minimum_daily_minutes: 480,
			work_mode: workMode,
			work_policy: {
				workMode,
				workingWeekdays: [1, 2, 3, 4, 5],
				dailyTargetMinutes: workMode === 'autonomous' ? 0 : 480,
				weeklyTargetMinutes: workMode === 'autonomous' ? 0 : 2400,
				referenceStartTime: '09:00',
				fixedStartTime: workMode === 'fixed' ? '09:00' : '',
				fixedEndTime: workMode === 'fixed' ? '18:00' : '',
				coreTimeEnabled: workMode === 'flexible',
				coreStartTime: workMode === 'flexible' ? '11:00' : '',
				coreEndTime: workMode === 'flexible' ? '16:00' : '',
				breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
				nightStartTime: '22:00',
				nightEndTime: '06:00',
				...overrides
			}
		}
	]);
	const policy = policies.get(member.id);
	if (!policy) throw new Error('expected work policy fixture');
	return policy;
}

describe('calculateSupabaseEmployeeWorkStatus', () => {
	test('includes an early Asia/Seoul clock-in in the requested day as provisional work', () => {
		const timeRange = supabaseWorkStatusTimeRange(['2026-08-14'], 'Asia/Seoul');
		const clockIn = '2026-08-13T22:44:00Z';
		const now = new Date('2026-08-14T00:00:00Z');
		const policy = policyFrom('fixed');
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-08-14'],
			timeZone: 'Asia/Seoul',
			attendance: [{ member_id: member.id, kind: 'clock_in', occurred_at: clockIn }],
			leave: [],
			policy,
			holidays: noHolidays,
			now
		});
		const day = status.days[0];
		if (!day) throw new Error('expected requested day status');

		expect(timeRange).toEqual({
			from: '2026-08-13T15:00:00.000Z',
			until: '2026-08-14T15:00:00.000Z'
		});
		expect(Date.parse(clockIn) >= Date.parse(timeRange.from)).toBe(true);
		expect(Date.parse(clockIn) < Date.parse(timeRange.until)).toBe(true);
		expect(day.provisionalSeconds).toBe(76 * 60);
		expect(day.provisionalMinutes).toBe(76);
		expect(day.actualMinutes).toBe(0);
		expect(day.fulfilledMinutes).toBe(76);
		expect(day.remainingMinutes).toBe(404);
		expect(status.fulfilledMinutes).toBe(76);
		expect(status.remainingMinutes).toBe(404);
		expect(day.workSegments).toEqual([
			{ startTime: '07:44', endTime: '09:00', provisional: true }
		]);
	});

	test('includes active overnight work in night and baseline calculations', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2027-01-04'],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2027-01-04T13:00:00Z' }
			],
			leave: [],
			policy: policyFrom('flexible'),
			holidays: noHolidays,
			now: new Date('2027-01-04T14:30:00Z')
		});

		expect(status.actualMinutes).toBe(0);
		expect(status.provisionalMinutes).toBe(90);
		expect(status.fulfilledMinutes).toBe(90);
		expect(status.remainingMinutes).toBe(390);
		expect(status.nightMinutes).toBe(90);
		expect(status.days[0]?.nightMinutes).toBe(90);
	});

	test('counts only working dates in stage-two capacity for a holiday week', () => {
		const days = [
			'2027-01-04',
			'2027-01-05',
			'2027-01-06',
			'2027-01-07',
			'2027-01-08',
			'2027-01-09',
			'2027-01-10'
		];
		const policy = policyFrom('fixed');
		const holidays = new Set(['2027-01-06']);

		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days,
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [],
			policy,
			holidays
		});

		expect(status.workingCapacitySeconds).toBe(4 * secondsPerDay);
		expect(status.calendarCapacitySeconds).toBe(7 * secondsPerDay);
		expect(status.days.find((day) => day.date === '2027-01-06')?.workingDate).toBe(false);
	});

	test('judges each date by the revision that was in force on it', () => {
		const policy = policyOfRevisions([
			revisionOf('1970-01-01', 'fixed'),
			revisionOf('2027-01-05', 'autonomous')
		]);

		const historical = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2027-01-04'],
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [],
			policy,
			holidays: noHolidays
		});
		const current = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2027-01-05'],
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [],
			policy,
			holidays: noHolidays
		});

		expect(historical.workMode).toBe('fixed');
		expect(historical.hasBaseline).toBe(true);
		expect(historical.targetMinutes).toBe(480);
		expect(current.workMode).toBe('autonomous');
		expect(current.hasBaseline).toBe(false);
		expect(current.targetMinutes).toBe(0);
	});

	test('applies one unchanged fixed policy across the whole period', () => {
		const days = [
			'2027-01-04',
			'2027-01-05',
			'2027-01-06',
			'2027-01-07',
			'2027-01-08',
			'2027-01-09',
			'2027-01-10'
		];
		const policy = policyFrom('fixed');

		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days,
			timeZone: 'Asia/Seoul',
			attendance: [
				{
					member_id: member.id,
					kind: 'clock_in',
					occurred_at: '2027-01-05T00:00:00Z'
				},
				{
					member_id: member.id,
					kind: 'clock_out',
					occurred_at: '2027-01-05T09:00:00Z'
				}
			],
			leave: [],
			policy,
			holidays: noHolidays
		});
		const autonomousDay = status.days.find((day) => day.date === '2027-01-05');

		expect(status.remainingMinutes).toBe(1920);
		expect(status.actualMinutes).toBe(480);
		expect(status.fulfilledMinutes).toBe(480);
		expect(status.differenceMinutes).toBe(-1920);
		expect(status.overtimeMinutes).toBe(0);
		expect(autonomousDay?.actualMinutes).toBe(480);
		expect(autonomousDay?.workMode).toBe('fixed');
		expect(autonomousDay?.differenceMinutes).toBe(0);
		expect(autonomousDay?.remainingMinutes).toBe(0);
		expect(autonomousDay?.overtimeMinutes).toBe(0);
	});

	test('uses the editable overnight window and excludes configured breaks', () => {
		const policy = policyFrom('flexible', {
			nightStartTime: '21:00',
			nightEndTime: '05:00',
			breakPeriods: [{ startTime: '22:30', endTime: '23:00' }]
		});
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2027-01-04'],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2027-01-04T12:00:00Z' },
				{ member_id: member.id, kind: 'clock_out', occurred_at: '2027-01-04T15:00:00Z' }
			],
			leave: [],
			policy,
			holidays: noHolidays
		});

		expect(status.actualMinutes).toBe(150);
		expect(status.nightMinutes).toBe(150);
		expect(status.days[0]?.nightMinutes).toBe(150);
	});

	test('uses the working weekdays of the revision in force', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: [
				'2027-01-04',
				'2027-01-05',
				'2027-01-06',
				'2027-01-07',
				'2027-01-08',
				'2027-01-09',
				'2027-01-10'
			],
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [],
			policy: policyFrom('fixed', { workingWeekdays: [6], weeklyTargetMinutes: 480 }),
			holidays: noHolidays
		});

		expect(status.workingCapacitySeconds).toBe(secondsPerDay);
		expect(status.calendarCapacitySeconds).toBe(7 * secondsPerDay);
	});

	test('uses current weekdays and policy-independent holidays instead of a stale projection', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2027-01-09', '2027-01-10'],
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [],
			policy: policyFrom('fixed', { workingWeekdays: [6, 7], weeklyTargetMinutes: 960 }),
			holidays: new Set(['2027-01-10'])
		});

		expect(status.days[0]?.workingDate).toBe(true);
		expect(status.days[0]?.targetMinutes).toBe(480);
		expect(status.days[1]?.workingDate).toBe(false);
		expect(status.days[1]?.targetMinutes).toBe(0);
	});

	test('credits a half-day partial leave with proportional minutes instead of dropping it', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-08-19'],
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [
				{
					member_id: member.id,
					days: 0.5,
					starts_at: '2026-08-19T00:00:00Z',
					ends_at: '2026-08-19T05:00:00Z',
					status: 'approved'
				}
			],
			policy: policyFrom('fixed'),
			holidays: noHolidays
		});

		expect(status.days[0]?.leaveMinutes).toBe(240);
		expect(status.leaveMinutes).toBe(240);
		expect(status.referenceDailyMinutes).toBe(480);
	});

	test('credits a quarter-day partial leave with proportional minutes', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-08-20'],
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [
				{
					member_id: member.id,
					days: 0.25,
					starts_at: '2026-08-20T00:00:00Z',
					ends_at: '2026-08-20T02:00:00Z',
					status: 'approved'
				}
			],
			policy: policyFrom('fixed'),
			holidays: noHolidays
		});

		expect(status.days[0]?.leaveMinutes).toBe(120);
		expect(status.leaveMinutes).toBe(120);
	});

	test('still credits the full daily target per day for a multi-day all-day leave', () => {
		const days = ['2026-08-24', '2026-08-25', '2026-08-26'];
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days,
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [
				{
					member_id: member.id,
					days: 3,
					starts_at: '2026-08-23T15:00:00Z',
					ends_at: '2026-08-26T15:00:00Z',
					status: 'approved'
				}
			],
			policy: policyFrom('fixed'),
			holidays: noHolidays
		});

		expect(status.days[0]?.leaveMinutes).toBe(480);
		expect(status.days[1]?.leaveMinutes).toBe(480);
		expect(status.days[2]?.leaveMinutes).toBe(480);
		expect(status.leaveMinutes).toBe(1440);
	});
	test('reports no overtime on a day whose whole target is covered by leave', () => {
		const fullDayLeave = {
			member_id: member.id,
			days: 1,
			starts_at: '2026-07-30T00:00:00Z',
			ends_at: '2026-07-31T00:00:00Z',
			status: 'approved'
		};
		const policy = policyFrom('fixed');
		const leaveDay = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-07-30'],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-07-30T00:00:00Z' },
				{ member_id: member.id, kind: 'clock_out', occurred_at: '2026-07-30T01:20:00Z' }
			],
			leave: [fullDayLeave],
			policy,
			holidays: noHolidays,
			now: new Date('2026-07-31T00:00:00Z')
		});

		expect(leaveDay.days[0]?.targetMinutes).toBe(0);
		expect(leaveDay.days[0]?.leaveMinutes).toBe(480);
		expect(leaveDay.days[0]?.actualMinutes).toBe(80);
		expect(leaveDay.days[0]?.overtimeMinutes).toBe(0);
		expect(leaveDay.days[0]?.differenceMinutes).toBe(0);
		expect(leaveDay.actualMinutes).toBe(80);
		expect(leaveDay.overtimeMinutes).toBe(0);

		const period = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-07-29', '2026-07-30'],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-07-29T00:00:00Z' },
				{ member_id: member.id, kind: 'clock_out', occurred_at: '2026-07-29T09:00:00Z' },
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-07-30T00:00:00Z' },
				{ member_id: member.id, kind: 'clock_out', occurred_at: '2026-07-30T01:20:00Z' }
			],
			leave: [fullDayLeave],
			policy,
			holidays: noHolidays,
			now: new Date('2026-07-31T00:00:00Z')
		});

		expect(period.targetMinutes).toBe(480);
		expect(period.actualMinutes).toBe(560);
		expect(period.overtimeMinutes).toBe(0);
		expect(period.differenceMinutes).toBe(0);
		expect(period.fulfilledMinutes).toBe(480);
	});

	test('keeps overtime on a day a half-day leave only partly covers', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-07-30'],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-07-30T00:00:00Z' },
				{ member_id: member.id, kind: 'clock_out', occurred_at: '2026-07-30T08:00:00Z' }
			],
			leave: [
				{
					member_id: member.id,
					days: 0.5,
					starts_at: '2026-07-30T00:00:00Z',
					ends_at: '2026-07-30T04:00:00Z',
					status: 'approved'
				}
			],
			policy: policyFrom('fixed'),
			holidays: noHolidays,
			now: new Date('2026-07-31T00:00:00Z')
		});

		expect(status.days[0]?.targetMinutes).toBe(240);
		expect(status.days[0]?.actualMinutes).toBe(420);
		expect(status.days[0]?.overtimeMinutes).toBe(180);
		expect(status.overtimeMinutes).toBe(180);
	});

	test('keeps period overtime when a fully leave-covered week also has weekend work', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: [
				'2026-07-27',
				'2026-07-28',
				'2026-07-29',
				'2026-07-30',
				'2026-07-31',
				'2026-08-01'
			],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-08-01T00:00:00Z' },
				{ member_id: member.id, kind: 'clock_out', occurred_at: '2026-08-01T03:00:00Z' }
			],
			leave: [
				{
					member_id: member.id,
					days: 5,
					starts_at: '2026-07-26T15:00:00Z',
					ends_at: '2026-07-31T15:00:00Z',
					status: 'approved'
				}
			],
			policy: policyFrom('fixed'),
			holidays: noHolidays,
			now: new Date('2026-08-02T00:00:00Z')
		});

		expect(status.targetMinutes).toBe(0);
		expect(status.leaveMinutes).toBe(2400);
		expect(status.actualMinutes).toBe(180);
		expect(status.overtimeMinutes).toBe(180);
		expect(status.days.at(-1)?.leaveMinutes).toBe(0);
		expect(status.days.at(-1)?.overtimeMinutes).toBe(180);
	});
});
