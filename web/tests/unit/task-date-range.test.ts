import { describe, expect, test } from 'bun:test';
import { taskDateParts, taskDateText } from '../../src/routes/task/task-date-range';

describe('flow task date range', () => {
	test('omits the year when it matches the current year', () => {
		const parts = taskDateParts('2026-06-03', 2026);

		expect(parts).toEqual({ month: '06', day: '03' });
		if (parts) expect(taskDateText(parts)).toBe('06/03');
	});

	test('keeps a year outside the current year', () => {
		const parts = taskDateParts('2025-12-31', 2026);

		expect(parts).toEqual({ year: '2025', month: '12', day: '31' });
		if (parts) expect(taskDateText(parts)).toBe('2025/12/31');
	});

	test('ignores malformed dates', () => {
		expect(taskDateParts('2026/06/03', 2026)).toBe(undefined);
	});
});
