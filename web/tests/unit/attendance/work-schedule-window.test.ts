import { describe, expect, test } from 'bun:test';
import {
	scheduleEndMinute,
	scheduleEndTime
} from '../../../src/lib/attendance/work-schedule-window';

const lunchBreak = [{ startTime: '12:00', endTime: '13:00' }];
const adjacentBreaks = [
	{ startTime: '12:00', endTime: '13:00' },
	{ startTime: '13:00', endTime: '14:00' }
];
const twoBreaks = [
	{ startTime: '12:00', endTime: '13:00' },
	{ startTime: '15:00', endTime: '15:30' }
];

describe('scheduleEndTime', () => {
	const cases = [
		{ name: 'a full day ends after the break it spans', startTime: '09:00', requiredMinutes: 480, breakPeriods: lunchBreak, expected: '18:00' },
		{ name: 'a half day ends an hour past the break', startTime: '09:00', requiredMinutes: 240, breakPeriods: lunchBreak, expected: '14:00' },
		{ name: 'a quarter day ends before the break', startTime: '09:00', requiredMinutes: 120, breakPeriods: lunchBreak, expected: '11:00' },
		{ name: 'a quarter day overlapping the break resumes after it', startTime: '11:30', requiredMinutes: 120, breakPeriods: lunchBreak, expected: '14:30' },
		{ name: 'a start inside the break counts from the end of it', startTime: '12:30', requiredMinutes: 120, breakPeriods: lunchBreak, expected: '15:00' },
		{ name: 'adjacent breaks are stepped over together', startTime: '12:30', requiredMinutes: 120, breakPeriods: adjacentBreaks, expected: '16:00' },
		{ name: 'a day with two breaks lays both on top of the target', startTime: '09:00', requiredMinutes: 480, breakPeriods: twoBreaks, expected: '18:30' },
		{ name: 'a day without breaks is the target added to the start', startTime: '09:00', requiredMinutes: 480, breakPeriods: [], expected: '17:00' }
	];

	for (const testCase of cases) {
		test(testCase.name, () => {
			expect(
				scheduleEndTime(testCase.startTime, testCase.requiredMinutes, testCase.breakPeriods)
			).toBe(testCase.expected);
		});
	}

	test('a target that runs past midnight is refused', () => {
		expect(() => scheduleEndTime('20:00', 480, lunchBreak)).toThrow('crosses the day boundary');
	});

	test('a target that lands exactly on midnight is refused', () => {
		expect(() => scheduleEndTime('23:00', 60, [])).toThrow('crosses the day boundary');
	});

	test('a target consumed entirely before midnight is allowed', () => {
		expect(scheduleEndTime('23:00', 59, [])).toBe('23:59');
	});
});

describe('scheduleEndMinute', () => {
	test('asking for no minutes returns the start untouched', () => {
		expect(scheduleEndMinute(9 * 60, 0, lunchBreak)).toBe(9 * 60);
	});

	test('asking for no minutes from inside a break does not step over it', () => {
		expect(scheduleEndMinute(12 * 60 + 30, 0, lunchBreak)).toBe(12 * 60 + 30);
	});

	test('a malformed break is refused rather than read as midnight', () => {
		expect(() => scheduleEndMinute(9 * 60, 60, [{ startTime: '9:00', endTime: '10:00' }])).toThrow(
			'breakPeriods is invalid'
		);
	});
});
