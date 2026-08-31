import { describe, expect, test } from 'bun:test';
import { formatTaskWeekCodeRange, formatTaskWeekDateRange } from '../../src/routes/task/task-week-label';

describe('formatTaskWeekDateRange', () => {
	test('formats the flow week range as month/day text', () => {
		expect(formatTaskWeekDateRange({ startISO: '2026-06-01', endISO: '2026-06-07' })).toBe('6/1 - 6/7');
	});

	test('returns a placeholder without a loaded week', () => {
		expect(formatTaskWeekDateRange(undefined)).toBe('...');
	});
});

describe('formatTaskWeekCodeRange', () => {
	test('reads a week code as the date range it covers', () => {
		expect(formatTaskWeekCodeRange('26W23')).toBe('6/1 - 6/7');
		expect(formatTaskWeekCodeRange('26W36')).toBe('8/31 - 9/6');
	});

	test('reads the Monday-date codes the central plane used to write', () => {
		expect(formatTaskWeekCodeRange('2026-06-01')).toBe('6/1 - 6/7');
	});

	test('stays empty for work that belongs to no week', () => {
		expect(formatTaskWeekCodeRange('')).toBe('');
	});

	test('shows a code it cannot read rather than dropping the week', () => {
		expect(formatTaskWeekCodeRange('2026-W29')).toBe('2026-W29');
		expect(formatTaskWeekCodeRange('26w28')).toBe('26w28');
		expect(formatTaskWeekCodeRange('not-a-week')).toBe('not-a-week');
	});
});
