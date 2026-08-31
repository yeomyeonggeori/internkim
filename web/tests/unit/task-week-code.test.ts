import { describe, expect, test } from 'bun:test';

import {
	taskWeekCodeForDateISO,
	taskWeekCodeForMonday,
	taskWeekDateISO,
	taskWeekForCode,
	taskWeekMondayForCode,
	taskWeekOfDate
} from '../../src/lib/task/task-week-code';

function mondayISOForCode(weekCode: string): string {
	const monday = taskWeekMondayForCode(weekCode);
	return monday ? taskWeekDateISO(monday) : '';
}

describe('task week code', () => {
	test('names the week a day falls in', () => {
		expect(taskWeekCodeForDateISO('2026-06-01')).toBe('26W23');
		expect(taskWeekCodeForDateISO('2026-06-07')).toBe('26W23');
		expect(taskWeekCodeForDateISO('2026-06-08')).toBe('26W24');
	});

	test('rejects a day that is not a date', () => {
		expect(taskWeekCodeForDateISO('')).toBe('');
		expect(taskWeekCodeForDateISO('2026-02-30')).toBe('');
	});

	test('resolves a code back to the Monday it names', () => {
		expect(mondayISOForCode('26W23')).toBe('2026-06-01');
		expect(mondayISOForCode('26W24')).toBe('2026-06-08');
	});

	test('still reads the Monday-date codes the central plane used to write', () => {
		expect(mondayISOForCode('2026-06-08')).toBe('2026-06-08');
		expect(mondayISOForCode('2026-06-10')).toBe('2026-06-08');
	});

	test('refuses a code that names no week', () => {
		expect(taskWeekMondayForCode('')).toBeNull();
		expect(taskWeekMondayForCode('26W00')).toBeNull();
		expect(taskWeekMondayForCode('26W54')).toBeNull();
		expect(taskWeekMondayForCode('not-a-week')).toBeNull();
	});

	test('round-trips every week of 2026, whose ISO year holds 53 of them', () => {
		for (let week = 1; week <= 53; week += 1) {
			const code = `26W${String(week).padStart(2, '0')}`;
			const monday = taskWeekMondayForCode(code);
			expect(monday).not.toBeNull();
			if (!monday) continue;
			expect(taskWeekCodeForMonday(monday)).toBe(code);
		}
	});
});

describe('task week built for the board', () => {
	const wednesdayOfWeek24 = new Date('2026-06-10T22:30:00Z');

	test('describes the week a moment falls in with codes the picker also emits', () => {
		expect(taskWeekOfDate(wednesdayOfWeek24)).toEqual({
			code: '26W24',
			startISO: '2026-06-08',
			endISO: '2026-06-14',
			previous: '26W23',
			next: '26W25',
			isCurrent: true
		});
	});

	test('builds the week a picked code names', () => {
		expect(taskWeekForCode('26W23', wednesdayOfWeek24)).toEqual({
			code: '26W23',
			startISO: '2026-06-01',
			endISO: '2026-06-07',
			previous: '26W22',
			next: '26W24',
			isCurrent: false
		});
	});

	test('marks the week holding today as the current one', () => {
		expect(taskWeekForCode('26W24', wednesdayOfWeek24).isCurrent).toBe(true);
	});

	test('builds the same week from a Monday-date code as from an ISO week code', () => {
		expect(taskWeekForCode('2026-06-01', wednesdayOfWeek24)).toEqual(taskWeekForCode('26W23', wednesdayOfWeek24));
	});

	test('falls back to today when the code names no week', () => {
		expect(taskWeekForCode('nonsense', wednesdayOfWeek24)).toEqual(taskWeekOfDate(wednesdayOfWeek24));
	});
});
