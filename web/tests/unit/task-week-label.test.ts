import { describe, expect, test } from 'bun:test';
import { taskWeekCodeForDateISO, formatTaskWeekDateRange } from '../../src/routes/task/task-week-label';

describe('formatTaskWeekDateRange', () => {
	test('formats the flow week range as month/day text', () => {
		expect(formatTaskWeekDateRange({ startISO: '2026-06-01', endISO: '2026-06-07' })).toBe('6/1 - 6/7');
	});

	test('returns a placeholder without a loaded week', () => {
		expect(formatTaskWeekDateRange(undefined)).toBe('...');
	});

	test('returns the ISO week code for a selected date', () => {
		expect(taskWeekCodeForDateISO('2026-06-01')).toBe('26W23');
		expect(taskWeekCodeForDateISO('2026-06-07')).toBe('26W23');
		expect(taskWeekCodeForDateISO('2026-06-08')).toBe('26W24');
	});
});
