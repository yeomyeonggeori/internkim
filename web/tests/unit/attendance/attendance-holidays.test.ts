import { describe, expect, test } from 'bun:test';
import { attendanceHolidayDates } from '../../../src/lib/attendance/attendance-holidays';

const august = { from: '2026-08-01', to: '2026-09-01' };

describe('attendanceHolidayDates', () => {
	test('takes both sources and counts a shared date once', async () => {
		const holidays = await attendanceHolidayDates(
			august,
			['2026-08-17', '2026-08-24'],
			async () => ['2026-08-15', '2026-08-17']
		);

		expect([...holidays].sort()).toEqual(['2026-08-15', '2026-08-17', '2026-08-24']);
	});

	test('is the company holidays alone when the country has none', async () => {
		const holidays = await attendanceHolidayDates(august, ['2026-08-24'], async () => []);

		expect([...holidays]).toEqual(['2026-08-24']);
	});

	test('fails rather than reporting a month with no holidays in it', async () => {
		await expect(
			attendanceHolidayDates(august, ['2026-08-24'], async () => {
				throw new Error('Nager.Date request to /v3/PublicHolidays/2026/KR failed');
			})
		).rejects.toThrow('Nager.Date');
	});
});
