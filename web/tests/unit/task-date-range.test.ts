import { describe, expect, test } from 'bun:test';
import { taskDateParts, taskDateRangeParts, taskDateText } from '../../src/routes/task/task-date-range';

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

describe('the dates a task card shows', () => {
	test('a task starting and ending on one day shows that day once', () => {
		expect(taskDateRangeParts('2026-09-07', '2026-09-07', 2026)).toEqual([{ month: '09', day: '07' }]);
	});

	test('a task spanning days shows both', () => {
		expect(taskDateRangeParts('2026-09-07', '2026-09-09', 2026)).toEqual([
			{ month: '09', day: '07' },
			{ month: '09', day: '09' }
		]);
	});

	test('a task with one date shows just that', () => {
		expect(taskDateRangeParts(undefined, '2026-09-09', 2026)).toEqual([{ month: '09', day: '09' }]);
		expect(taskDateRangeParts('2026-09-07', undefined, 2026)).toEqual([{ month: '09', day: '07' }]);
	});
});

