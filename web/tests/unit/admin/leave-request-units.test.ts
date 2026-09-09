import { describe, expect, test } from 'bun:test';
import { leaveUnitsWith } from '../../../src/routes/admin/attendance-leave-policy-model';
import {
	daysInLeaveYearStartMonth,
	leaveYearStartWithin,
	sameLeaveYearStart
} from '../../../src/routes/admin/leave-year-start-model';

describe('the units a leave can be asked in', () => {
	test('offering a smaller unit offers the larger ones with it', () => {
		expect(leaveUnitsWith([], 'quarterDay', true)).toEqual(['fullDay', 'halfDay', 'quarterDay']);
		expect(leaveUnitsWith([], 'halfDay', true)).toEqual(['fullDay', 'halfDay']);
		expect(leaveUnitsWith([], 'fullDay', true)).toEqual(['fullDay']);
	});

	test('withdrawing a larger unit withdraws the smaller ones under it', () => {
		const all = ['fullDay', 'halfDay', 'quarterDay'] as const;
		expect(leaveUnitsWith([...all], 'fullDay', false)).toEqual([]);
		expect(leaveUnitsWith([...all], 'halfDay', false)).toEqual(['fullDay']);
		expect(leaveUnitsWith([...all], 'quarterDay', false)).toEqual(['fullDay', 'halfDay']);
	});

	test('withdrawing keeps only what was held', () => {
		expect(leaveUnitsWith(['halfDay'], 'quarterDay', false)).toEqual(['halfDay']);
	});
});

describe('the day a leave year starts on', () => {
	test('a month offers the days it has', () => {
		expect(daysInLeaveYearStartMonth(1)).toBe(31);
		expect(daysInLeaveYearStartMonth(4)).toBe(30);
	});

	test('February stops at the 28th, the way the record refuses the 29th', () => {
		expect(daysInLeaveYearStartMonth(2)).toBe(28);
		expect(leaveYearStartWithin(2, 29)).toEqual({ month: 2, day: 28 });
	});

	test('a day the chosen month does not have comes back as its last', () => {
		expect(leaveYearStartWithin(4, 31)).toEqual({ month: 4, day: 30 });
	});

	test('a month or day outside the calendar is brought inside it', () => {
		expect(leaveYearStartWithin(0, 0)).toEqual({ month: 1, day: 1 });
		expect(leaveYearStartWithin(13, 99)).toEqual({ month: 12, day: 31 });
		expect(leaveYearStartWithin(Number.NaN, Number.NaN)).toEqual({ month: 1, day: 1 });
	});

	test('two starts are the same when both halves are', () => {
		expect(sameLeaveYearStart({ month: 3, day: 1 }, { month: 3, day: 1 })).toBe(true);
		expect(sameLeaveYearStart({ month: 3, day: 1 }, { month: 3, day: 2 })).toBe(false);
	});
});
