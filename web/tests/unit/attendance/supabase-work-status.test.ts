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

function policyFrom(workCalendar: unknown) {
	const policies = parseSupabaseWorkPolicies([
		{
			member_id: member.id,
			work_hours: weekdayWorkHours,
			minimum_daily_minutes: 480,
			work_mode: 'autonomous',
			work_calendar: workCalendar
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
		const policy = policyFrom([{ date: '2026-08-14', workMode: 'fixed', workingDate: true }]);
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-08-14'],
			timeZone: 'Asia/Seoul',
			attendance: [{ member_id: member.id, kind: 'clock_in', occurred_at: clockIn }],
			leave: [],
			policy,
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
		expect(day.workSegments).toEqual([
			{ startTime: '07:44', endTime: '09:00', provisional: true }
		]);
	});

	test('counts only projected working dates in stage-two capacity for a holiday week', () => {
		const days = [
			'2027-01-04',
			'2027-01-05',
			'2027-01-06',
			'2027-01-07',
			'2027-01-08',
			'2027-01-09',
			'2027-01-10'
		];
		const policy = policyFrom(
			days.map((date, index) => ({
				date,
				workMode: 'fixed',
				workingDate: index < 5 && date !== '2027-01-06'
			}))
		);

		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days,
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [],
			policy
		});

		expect(status.workingCapacitySeconds).toBe(4 * secondsPerDay);
		expect(status.calendarCapacitySeconds).toBe(7 * secondsPerDay);
		expect(status.days.find((day) => day.date === '2027-01-06')?.workingDate).toBe(false);
	});

	test('uses each projected date mode instead of the current compatibility mode', () => {
		const policy = policyFrom([
			{ date: '2027-01-04', workMode: 'fixed', workingDate: true },
			{ date: '2027-01-05', workMode: 'autonomous', workingDate: true }
		]);

		const historical = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2027-01-04'],
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [],
			policy
		});
		const current = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2027-01-05'],
			timeZone: 'Asia/Seoul',
			attendance: [],
			leave: [],
			policy
		});

		expect(historical.workMode).toBe('fixed');
		expect(historical.hasBaseline).toBe(true);
		expect(historical.targetMinutes).toBe(480);
		expect(current.workMode).toBe('autonomous');
		expect(current.hasBaseline).toBe(false);
		expect(current.targetMinutes).toBe(0);
	});

	test('excludes autonomous work from mixed-period baseline comparisons', () => {
		const days = [
			'2027-01-04',
			'2027-01-05',
			'2027-01-06',
			'2027-01-07',
			'2027-01-08',
			'2027-01-09',
			'2027-01-10'
		];
		const policy = policyFrom(
			days.map((date, index) => ({
				date,
				workMode: index === 0 ? 'fixed' : 'autonomous',
				workingDate: index < 5
			}))
		);

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
					occurred_at: '2027-01-05T08:00:00Z'
				}
			],
			leave: [],
			policy
		});
		const autonomousDay = status.days.find((day) => day.date === '2027-01-05');

		expect(status.remainingMinutes).toBe(480);
		expect(status.actualMinutes).toBe(480);
		expect(status.fulfilledMinutes).toBe(0);
		expect(status.differenceMinutes).toBe(-480);
		expect(status.overtimeMinutes).toBe(0);
		expect(autonomousDay?.actualMinutes).toBe(480);
		expect(autonomousDay?.differenceMinutes).toBe(0);
		expect(autonomousDay?.remainingMinutes).toBe(0);
		expect(autonomousDay?.overtimeMinutes).toBe(0);
	});

	test('falls back to the resolved work-hours cycle when no projection exists', () => {
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
			policy: policyFrom(null)
		});

		expect(status.workingCapacitySeconds).toBe(5 * secondsPerDay);
		expect(status.calendarCapacitySeconds).toBe(7 * secondsPerDay);
	});
});
