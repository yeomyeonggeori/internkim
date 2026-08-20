import { describe, expect, test } from 'bun:test';
import { parseSupabaseWorkPolicies } from '../../../src/lib/attendance/supabase-work-policy';
import { calculateSupabaseEmployeeWorkStatus } from '../../../src/lib/attendance/supabase-work-status';

const member = {
	id: 'member-records',
	name: '박예시',
	email: 'sample2@example.com',
	is_admin: false,
	user_id: 'user-records',
	joined_at: '2026-01-01T00:00:00Z'
};

function fixedPolicy() {
	const policies = parseSupabaseWorkPolicies([
		{
			member_id: member.id,
			work_hours: null,
			minimum_daily_minutes: 480,
			work_mode: 'fixed',
			work_calendar: null,
			work_policy: {
				workMode: 'fixed',
				workingWeekdays: [1, 2, 3, 4, 5],
				dailyTargetMinutes: 480,
				weeklyTargetMinutes: 2400,
				referenceStartTime: '09:00',
				fixedStartTime: '09:00',
				fixedEndTime: '18:00',
				coreTimeEnabled: false,
				coreStartTime: '',
				coreEndTime: '',
				breakPeriods: [{ startTime: '12:00', endTime: '13:00' }],
				nightStartTime: '22:00',
				nightEndTime: '06:00'
			}
		}
	]);
	const policy = policies.get(member.id);
	if (!policy) throw new Error('expected work policy fixture');
	return policy;
}

describe('calculateSupabaseEmployeeWorkStatus event pairing', () => {
	test('a second clock-in before a clock-out marks the already-open date incomplete and needs review', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-08-10'],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-08-10T00:00:00Z' },
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-08-10T01:00:00Z' },
				{ member_id: member.id, kind: 'clock_out', occurred_at: '2026-08-10T09:00:00Z' }
			],
			leave: [],
			policy: fixedPolicy(),
			now: new Date('2026-08-15T00:00:00Z')
		});
		const day = status.days[0];
		if (!day) throw new Error('expected requested day status');

		expect(day.hasIncompleteWorkRecord).toBe(true);
		expect(day.needsReview).toBe(true);
		expect(day.actualMinutes).toBe(420);
	});

	test('a clock-out with no open session marks its own date incomplete and needs review', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-08-11'],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_out', occurred_at: '2026-08-11T09:00:00Z' }
			],
			leave: [],
			policy: fixedPolicy(),
			now: new Date('2026-08-15T00:00:00Z')
		});
		const day = status.days[0];
		if (!day) throw new Error('expected requested day status');

		expect(day.hasIncompleteWorkRecord).toBe(true);
		expect(day.needsReview).toBe(true);
		expect(day.actualMinutes).toBe(0);
		expect(day.provisionalMinutes).toBe(0);
	});

	test('an open clock-in from four days ago yields no provisional minutes and marks its date incomplete', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-08-11'],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-08-11T01:00:00Z' }
			],
			leave: [],
			policy: fixedPolicy(),
			now: new Date('2026-08-15T01:00:00Z')
		});
		const day = status.days[0];
		if (!day) throw new Error('expected requested day status');

		expect(day.provisionalMinutes).toBe(0);
		expect(day.hasIncompleteWorkRecord).toBe(true);
		expect(status.provisionalMinutes).toBe(0);
	});

	test('an open clock-in from earlier today yields provisional minutes and stays complete', () => {
		const status = calculateSupabaseEmployeeWorkStatus({
			member,
			days: ['2026-08-15'],
			timeZone: 'Asia/Seoul',
			attendance: [
				{ member_id: member.id, kind: 'clock_in', occurred_at: '2026-08-15T01:00:00Z' }
			],
			leave: [],
			policy: fixedPolicy(),
			now: new Date('2026-08-15T09:00:00Z')
		});
		const day = status.days[0];
		if (!day) throw new Error('expected requested day status');

		expect(day.provisionalMinutes).toBe(420);
		expect(day.hasIncompleteWorkRecord).toBe(false);
		expect(day.needsReview).toBe(false);
		expect(day.isWorking).toBe(true);
	});
});
