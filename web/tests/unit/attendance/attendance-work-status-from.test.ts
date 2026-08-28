import { describe, expect, test } from 'bun:test';
import { parseSupabaseWorkPolicies } from '../../../src/lib/attendance/supabase-work-policy';
import {
	attendanceWorkStatusFrom,
	coveredDaysOf,
	type SupabaseWorkStatusInputs
} from '../../../src/lib/attendance/supabase-work-status';

const timeZone = 'Asia/Seoul';
const now = new Date('2026-09-07T00:00:00Z');

const member = {
	id: 'member-one',
	name: '이샘플',
	email: 'sample@example.com',
	is_admin: false,
	user_id: 'user-one',
	joined_at: '2026-01-01T00:00:00Z'
};

function policy() {
	const policies = parseSupabaseWorkPolicies([
		{
			member_id: member.id,
			work_hours: [[[], [], [], [], [], null, null]],
			minimum_daily_minutes: 480,
			work_mode: 'flexible',
			work_policy: {
				version: 1,
				revisions: [
					{
						effectiveDate: '1970-01-01',
						workMode: 'flexible',
						workingWeekdays: [1, 2, 3, 4, 5],
						dailyTargetMinutes: 480,
						weeklyTargetMinutes: 2400,
						referenceStartTime: '09:00',
						fixedStartTime: '',
						fixedEndTime: '',
						coreTimeEnabled: false,
						coreStartTime: '',
						coreEndTime: '',
						breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
						nightStartTime: '22:00',
						nightEndTime: '06:00'
					}
				]
			}
		}
	]);
	const found = policies.get(member.id);
	if (!found) throw new Error('expected a work policy fixture');
	return found;
}

const august = { period: 'month', anchor: '2026-08-05' } as const;
const oneDay = { period: 'day', anchor: '2026-08-20' } as const;
const crossingWeek = { period: 'week', anchor: '2026-08-31' } as const;

function inputsFor(requests: { period: 'day' | 'week' | 'month'; anchor: string }[]) {
	const inputs: SupabaseWorkStatusInputs = {
		timeZone,
		members: [member],
		me: member,
		attendance: [
			{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-08-20T00:00:00Z' },
			{ member_id: member.id, kind: 'clock_out', occurred_at: '2026-08-20T09:00:00Z' },
			{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-09-02T00:00:00Z' },
			{ member_id: member.id, kind: 'clock_out', occurred_at: '2026-09-02T09:00:00Z' }
		],
		leave: [],
		policiesByMember: new Map([[member.id, policy()]]),
		holidays: new Set<string>(),
		coveredDays: coveredDaysOf(requests, timeZone, now),
		now
	};
	return inputs;
}

describe('attendanceWorkStatusFrom', () => {
	test('one set of rows answers two periods without either taking the other totals', () => {
		const inputs = inputsFor([august, oneDay]);
		const month = attendanceWorkStatusFrom(inputs, august);
		const day = attendanceWorkStatusFrom(inputs, oneDay);

		expect(month.periodStart).toBe('2026-08-01');
		expect(month.periodEnd).toBe('2026-08-31');
		expect(day.periodStart).toBe('2026-08-20');
		expect(day.periodEnd).toBe('2026-08-20');

		expect(day.personal?.targetMinutes).toBe(480);
		expect(month.personal?.targetMinutes).toBe(21 * 480);
		expect(day.personal?.actualMinutes).toBe(480);
		expect(month.personal?.actualMinutes).toBe(480);
		expect(day.personal?.calendarCapacitySeconds).toBe(24 * 60 * 60);
		expect(month.personal?.calendarCapacitySeconds).toBe(31 * 24 * 60 * 60);
	});

	test('the order the periods are asked in does not change either answer', () => {
		const inputs = inputsFor([august, oneDay]);
		const dayFirst = attendanceWorkStatusFrom(inputs, oneDay);
		attendanceWorkStatusFrom(inputs, august);
		const dayAgain = attendanceWorkStatusFrom(inputs, oneDay);

		expect(dayAgain).toEqual(dayFirst);
	});

	test('a week running past the month reads the days on the far side of the boundary', () => {
		const inputs = inputsFor([august, crossingWeek]);
		const week = attendanceWorkStatusFrom(inputs, crossingWeek);

		expect(week.periodStart).toBe('2026-08-31');
		expect(week.periodEnd).toBe('2026-09-06');
		expect(week.personal?.actualMinutes).toBe(480);
		expect(week.personal?.days.map((day) => day.date)).toContain('2026-09-02');
	});

	test('the month beside a crossing week keeps its own bounds', () => {
		const inputs = inputsFor([august, crossingWeek]);
		const month = attendanceWorkStatusFrom(inputs, august);

		expect(month.periodStart).toBe('2026-08-01');
		expect(month.periodEnd).toBe('2026-08-31');
		expect(month.personal?.actualMinutes).toBe(480);
	});

	test('the period a payload reports is the one it was asked for', () => {
		const inputs = inputsFor([august, oneDay]);

		expect(attendanceWorkStatusFrom(inputs, august).period).toBe('month');
		expect(attendanceWorkStatusFrom(inputs, oneDay).period).toBe('day');
		expect(attendanceWorkStatusFrom(inputs, oneDay).anchor).toBe('2026-08-20');
	});
});
