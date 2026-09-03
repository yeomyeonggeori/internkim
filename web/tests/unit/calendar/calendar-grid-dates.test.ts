import { describe, expect, test } from 'bun:test';

import {
	calendarGridDateKey,
	calendarGridMonthAnchorWeekStart,
	startOfCalendarGridMonth
} from '../../../src/routes/calendar/grid/calendar-grid-dates';

describe('calendar grid month anchor', () => {
	test('anchors a mid-month date on the week that holds the first of its month', () => {
		expect(calendarGridDateKey(calendarGridMonthAnchorWeekStart(new Date(2026, 5, 16)))).toBe('2026-05-31');
	});

	test('anchors every date in a month on the same week', () => {
		const anchors = [1, 9, 16, 30].map((day) =>
			calendarGridDateKey(calendarGridMonthAnchorWeekStart(new Date(2026, 5, day)))
		);

		expect(new Set(anchors).size).toBe(1);
	});

	test('starts the month at midnight on its first day', () => {
		expect(startOfCalendarGridMonth(new Date(2026, 5, 16, 12, 30))).toEqual(new Date(2026, 5, 1, 0, 0, 0, 0));
	});
});
