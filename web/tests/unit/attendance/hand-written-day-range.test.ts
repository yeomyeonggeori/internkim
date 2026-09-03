import { describe, expect, test } from 'bun:test';
import { currentAndPreviousMonth } from '../../../src/routes/attendance/hand-written/hand-written-records';

describe('the days the hand-written records are read over', () => {
	test('starts on the first day of the previous month and ends today', () => {
		expect(currentAndPreviousMonth('2026-09-04')).toEqual({
			from: '2026-08-01',
			to: '2026-09-04'
		});
	});

	test('reaches back into the previous year in January', () => {
		expect(currentAndPreviousMonth('2026-01-15')).toEqual({
			from: '2025-12-01',
			to: '2026-01-15'
		});
	});
});
