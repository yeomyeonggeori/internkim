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
	yearStart?: { month: number; day: number };
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
	},
	{
		name: 'a February leave for a company whose year starts in March',
		startsAt: '2026-02-10T00:00:00+09:00',
		endsAt: '2026-02-12T00:00:00+09:00',
		localStartDate: '2026-02-10',
		localEndDate: '2026-02-11',
		totalDays: 2,
		years: [2025, 2026],
		yearStart: { month: 3, day: 1 }
	},
	{
		name: 'a leave over the last day of February for a March start',
		startsAt: '2026-02-27T00:00:00+09:00',
		endsAt: '2026-03-03T00:00:00+09:00',
		localStartDate: '2026-02-27',
		localEndDate: '2026-03-02',
		totalDays: 4,
		years: [2025, 2026],
		yearStart: { month: 3, day: 1 }
	},
	{
		name: 'a leave over new year for a March start',
		startsAt: '2025-12-30T00:00:00+09:00',
		endsAt: '2026-01-02T00:00:00+09:00',
		localStartDate: '2025-12-30',
		localEndDate: '2026-01-01',
		totalDays: 3,
		years: [2025, 2026],
		yearStart: { month: 3, day: 1 }
	},
	{
		name: 'a year that opens mid-month, over its own boundary',
		startsAt: '2026-04-13T00:00:00+09:00',
		endsAt: '2026-04-17T00:00:00+09:00',
		localStartDate: '2026-04-13',
		localEndDate: '2026-04-16',
		totalDays: 4,
		years: [2025, 2026],
		yearStart: { month: 4, day: 15 }
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
				target_year: year,
				year_start_month: shareCase.yearStart?.month ?? 1,
				year_start_day: shareCase.yearStart?.day ?? 1
			});
			expect(answered.error).toBeNull();

			const inTypeScript = leaveDaysInYear(
				shareCase.totalDays,
				shareCase.localStartDate,
				shareCase.localEndDate,
				year,
				shareCase.yearStart?.month ?? 1,
				shareCase.yearStart?.day ?? 1
			);
			expect(rounded(Number(answered.data))).toBe(rounded(inTypeScript));
		}
	});
}
});
