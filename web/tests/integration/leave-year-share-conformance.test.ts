import { describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { leaveDaysInYear } from '../../src/lib/attendance/leave-year-share';
import { projectURL, serviceRoleKey } from './supabase-environment';


const client = createClient(projectURL, serviceRoleKey, {
	auth: { autoRefreshToken: false, persistSession: false }
});

const timeZone = 'Asia/Seoul';

function rounded(value: number): number {
	return Math.round(value * 1_000_000) / 1_000_000;
}

type ShareCase = {
	name: string;
	startsAt: string;
	endsAt: string;
	localStartDate: string;
	localEndDate: string;
	totalDays: number;
	years: number[];
	timeZone?: string;
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
		name: 'new year in a timezone behind the company',
		startsAt: '2026-12-31T00:00:00+09:00',
		endsAt: '2027-01-02T00:00:00+09:00',
		localStartDate: '2026-12-30',
		localEndDate: '2027-01-01',
		totalDays: 2,
		years: [2026, 2027],
		timeZone: 'America/Los_Angeles'
	},
	{
		name: 'a row whose end never moves past its start',
		startsAt: '2026-07-01T00:00:00+09:00',
		endsAt: '2026-07-01T00:00:00+09:00',
		localStartDate: '2026-07-01',
		localEndDate: '2026-06-30',
		totalDays: 1,
		years: [2026]
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

describe('leave_days_in_year agrees with leaveDaysInYear', () => {
for (const shareCase of cases) {
	test(shareCase.name, async () => {
		for (const year of shareCase.years) {
			const answered = await client.rpc('leave_days_in_year', {
				starts_at: shareCase.startsAt,
				ends_at: shareCase.endsAt,
				total_days: shareCase.totalDays,
				time_zone: shareCase.timeZone ?? timeZone,
				target_year: year
			});
			expect(answered.error).toBeNull();

			const inTypeScript = leaveDaysInYear(
				shareCase.totalDays,
				shareCase.localStartDate,
				shareCase.localEndDate,
				year
			);
			expect(rounded(Number(answered.data))).toBe(rounded(inTypeScript));
		}
	});
}
});
