import { describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { leaveDaysInYear } from '../../src/lib/attendance/leave-year-share';

// The rule that splits a leave across calendar years is written twice: once in
// SQL for member_leave_remaining, once in TypeScript for the summaries the
// browser computes from rows it already holds. Neither can call the other
// without a round trip per row, so this reads the SQL one and fails when the
// two answers drift apart.

const projectURL = process.env.SUPABASE_URL ?? '';
const serviceRoleKey = process.env.SUPABASE_SECRET_KEY ?? process.env.SUPABASE_SERVICE_ROLE_KEY ?? '';
const canReachSupabase = Boolean(projectURL && serviceRoleKey);
const client = canReachSupabase
	? createClient(projectURL, serviceRoleKey, {
			auth: { autoRefreshToken: false, persistSession: false }
		})
	: null;

const timeZone = 'Asia/Seoul';

type ShareCase = {
	name: string;
	startsAt: string;
	endsAt: string;
	localStartDate: string;
	localEndDate: string;
	totalDays: number;
	years: number[];
};

const cases: ShareCase[] = [
	{
		name: 'a single full day',
		startsAt: '2026-07-01T00:00:00+09:00',
		endsAt: '2026-07-02T00:00:00+09:00',
		localStartDate: '2026-07-01',
		localEndDate: '2026-07-01',
		totalDays: 1,
		years: [2025, 2026, 2027]
	},
	{
		name: 'a half day',
		startsAt: '2026-07-01T09:00:00+09:00',
		endsAt: '2026-07-01T14:00:00+09:00',
		localStartDate: '2026-07-01',
		localEndDate: '2026-07-01',
		totalDays: 0.5,
		years: [2026, 2027]
	},
	{
		name: 'four days across new year',
		startsAt: '2026-12-30T00:00:00+09:00',
		endsAt: '2027-01-03T00:00:00+09:00',
		localStartDate: '2026-12-30',
		localEndDate: '2027-01-02',
		totalDays: 4,
		years: [2025, 2026, 2027, 2028]
	},
	{
		name: 'one day either side of new year',
		startsAt: '2026-12-31T00:00:00+09:00',
		endsAt: '2027-01-02T00:00:00+09:00',
		localStartDate: '2026-12-31',
		localEndDate: '2027-01-01',
		totalDays: 2,
		years: [2026, 2027]
	},
	{
		name: 'a span covering a whole year',
		startsAt: '2025-12-30T00:00:00+09:00',
		endsAt: '2027-01-03T00:00:00+09:00',
		localStartDate: '2025-12-30',
		localEndDate: '2027-01-02',
		totalDays: 369,
		years: [2025, 2026, 2027]
	}
];

describe.skipIf(!canReachSupabase)('leave_days_in_year agrees with leaveDaysInYear', () => {
	for (const shareCase of cases) {
		test(shareCase.name, async () => {
			for (const year of shareCase.years) {
				const answered = await client!.rpc('leave_days_in_year', {
					starts_at: shareCase.startsAt,
					ends_at: shareCase.endsAt,
					total_days: shareCase.totalDays,
					time_zone: timeZone,
					target_year: year
				});
				expect(answered.error).toBeNull();

				const inTypeScript = leaveDaysInYear(
					shareCase.totalDays,
					shareCase.localStartDate,
					shareCase.localEndDate,
					year
				);
				expect(Number(answered.data)).toBeCloseTo(inTypeScript, 6);
			}
		});
	}
});
