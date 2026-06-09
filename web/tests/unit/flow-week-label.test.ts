import { describe, expect, test } from 'bun:test';
import { flowWeekCodeForDateISO, formatFlowWeekDateRange } from '../../src/routes/flow/flow-week-label';

describe('formatFlowWeekDateRange', () => {
	test('formats the flow week range as month/day text', () => {
		expect(formatFlowWeekDateRange({ startISO: '2026-06-01', endISO: '2026-06-07' })).toBe('6/1 - 6/7');
	});

	test('returns a placeholder without a loaded week', () => {
		expect(formatFlowWeekDateRange(undefined)).toBe('...');
	});

	test('returns the ISO week code for a selected date', () => {
		expect(flowWeekCodeForDateISO('2026-06-01')).toBe('26W23');
		expect(flowWeekCodeForDateISO('2026-06-07')).toBe('26W23');
		expect(flowWeekCodeForDateISO('2026-06-08')).toBe('26W24');
	});
});
