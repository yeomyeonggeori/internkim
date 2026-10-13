import { describe, expect, test } from 'bun:test';

import { calendarGridWeek } from '../../../src/routes/calendar/grid/calendar-grid-dates';
import { calendarGridWeekLayout, type CalendarGridEvent } from '../../../src/routes/calendar/grid/calendar-grid-layout';
import { calendarMonthDayRows, isCalendarMonthSpanVisible } from '../../../src/routes/calendar/grid/calendar-month-day-rows';

function event(overrides: Partial<CalendarGridEvent> & Pick<CalendarGridEvent, 'id' | 'start' | 'end'>): CalendarGridEvent {
	return { title: overrides.id, isAllDay: false, color: '#000000', participants: [], ...overrides };
}

function allDay(id: string, startDay: number, endDay: number): CalendarGridEvent {
	return event({ id, start: new Date(2026, 9, startDay), end: new Date(2026, 9, endDay), isAllDay: true });
}

function meeting(id: string, day: number, hour: number): CalendarGridEvent {
	return event({ id, start: new Date(2026, 9, day, hour), end: new Date(2026, 9, day, hour + 1) });
}

const week = calendarGridWeek(new Date(2026, 9, 4));
const wednesdayIndex = 3;

describe('calendar month day rows', () => {
	test('shows every event when they fit in three rows', () => {
		const layout = calendarGridWeekLayout(week, [allDay('holiday', 7, 8), meeting('standup', 7, 9), meeting('review', 7, 14)]);

		const rows = calendarMonthDayRows(layout, wednesdayIndex);

		expect(rows.spanLaneCount).toBe(1);
		expect(rows.visibleTimedEntries.map((entry) => entry.event.id)).toEqual(['standup', 'review']);
		expect(rows.hiddenCount).toBe(0);
	});

	test('moves a third multi-day lane into the more count instead of overflowing the cell', () => {
		const layout = calendarGridWeekLayout(week, [
			allDay('holiday', 7, 8),
			allDay('trip', 5, 9),
			allDay('booth', 6, 10),
			meeting('standup', 7, 9)
		]);

		const rows = calendarMonthDayRows(layout, wednesdayIndex);

		expect(layout.spans.filter(isCalendarMonthSpanVisible).map((span) => span.event.id)).toEqual(['trip', 'booth']);
		expect(rows.spanLaneCount).toBe(2);
		expect(rows.visibleTimedEntries).toEqual([]);
		expect(rows.hiddenCount).toBe(2);
	});

	test('keeps two rows of events above the more button when timed events overflow', () => {
		const layout = calendarGridWeekLayout(week, [meeting('first', 7, 9), meeting('second', 7, 10), meeting('third', 7, 11), meeting('fourth', 7, 12)]);

		const rows = calendarMonthDayRows(layout, wednesdayIndex);

		expect(rows.visibleTimedEntries.map((entry) => entry.event.id)).toEqual(['first', 'second']);
		expect(rows.hiddenCount).toBe(2);
	});
});
