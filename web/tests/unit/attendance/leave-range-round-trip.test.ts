import { describe, expect, test } from 'bun:test';
import {
	leaveDisplayRange,
	leaveTimestampRange
} from '../../../src/lib/attendance/supabase-leave-range';
import { leaveDaysInYear } from '../../../src/lib/attendance/leave-year-share';
import type { EmployeeLeavePreviewRequest } from '../../../src/routes/attendance/leave/employee-leave-types';

const timeZones = ['Asia/Seoul', 'UTC', 'America/Los_Angeles', 'Pacific/Auckland'];

function roundTrip(request: EmployeeLeavePreviewRequest, days: number, timeZone: string) {
	const range = leaveTimestampRange(request, timeZone);
	return leaveDisplayRange(range.startsAt, range.endsAt, days, timeZone);
}

describe('a leave written from its dates reads back as the same dates', () => {
	for (const timeZone of timeZones) {
		test(`three full days in ${timeZone}`, () => {
			const shown = roundTrip(
				{ leaveTypeID: 'leave', unit: 'fullDay', startDate: '2026-03-02', endDate: '2026-03-04' },
				3,
				timeZone
			);

			expect(shown.startDate).toBe('2026-03-02');
			expect(shown.endDate).toBe('2026-03-04');
		});

		test(`one full day in ${timeZone}`, () => {
			const shown = roundTrip(
				{ leaveTypeID: 'leave', unit: 'fullDay', startDate: '2026-03-02' },
				1,
				timeZone
			);

			expect(shown.startDate).toBe('2026-03-02');
			expect(shown.endDate).toBe('2026-03-02');
		});

		test(`a half day in ${timeZone}`, () => {
			const shown = roundTrip(
				{
					leaveTypeID: 'leave',
					unit: 'halfDay',
					startDate: '2026-03-02',
					partialPeriod: 'morning'
				},
				0.5,
				timeZone
			);

			expect(shown.startDate).toBe('2026-03-02');
			expect(shown.endDate).toBeUndefined();
		});
	}

	test('a span across new year keeps every day it was given', () => {
		const shown = roundTrip(
			{ leaveTypeID: 'leave', unit: 'fullDay', startDate: '2026-12-30', endDate: '2027-01-02' },
			4,
			'Asia/Seoul'
		);

		expect(shown.startDate).toBe('2026-12-30');
		expect(shown.endDate).toBe('2027-01-02');
		expect(leaveDaysInYear(4, shown.startDate, shown.endDate ?? shown.startDate, 2026)).toBe(2);
		expect(leaveDaysInYear(4, shown.startDate, shown.endDate ?? shown.startDate, 2027)).toBe(2);
	});
});
