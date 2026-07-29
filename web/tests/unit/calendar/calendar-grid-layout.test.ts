import { describe, expect, test } from 'bun:test';

import { calendarGridWeek, calendarGridWeeks, calendarGridDominantMonth, startOfCalendarGridWeek } from '../../../src/routes/calendar/grid/calendar-grid-dates';
import {
	calendarGridTimedBlocks,
	calendarGridWeekLayout,
	type CalendarGridEvent
} from '../../../src/routes/calendar/grid/calendar-grid-layout';

function event(overrides: Partial<CalendarGridEvent> & Pick<CalendarGridEvent, 'id' | 'start' | 'end'>): CalendarGridEvent {
	return { title: overrides.id, isAllDay: false, color: '#000000', ...overrides };
}

describe('calendar grid weeks', () => {
	test('starts weeks on sunday around the anchor date', () => {
		const weeks = calendarGridWeeks(new Date(2026, 6, 29), 1, 1);

		expect(weeks).toHaveLength(3);
		expect(weeks[1].startDateKey).toBe('2026-07-26');
		expect(weeks[0].startDateKey).toBe('2026-07-19');
		expect(weeks[2].startDateKey).toBe('2026-08-02');
	});

	test('names a week by the month covering most of its days', () => {
		const week = calendarGridWeek(startOfCalendarGridWeek(new Date(2026, 6, 29)));

		expect(calendarGridDominantMonth(week).getMonth()).toBe(6);
	});
});

describe('calendar grid week layout', () => {
	test('stacks overlapping multi-day events into separate lanes', () => {
		const week = calendarGridWeek(new Date(2026, 6, 26));
		const layout = calendarGridWeekLayout(week, [
			event({ id: 'trip', start: new Date(2026, 6, 27), end: new Date(2026, 6, 30), isAllDay: true }),
			event({ id: 'workshop', start: new Date(2026, 6, 28), end: new Date(2026, 6, 31), isAllDay: true })
		]);

		expect(layout.spans.map((span) => [span.event.id, span.startColumn, span.columnCount, span.lane])).toEqual([
			['trip', 1, 3, 0],
			['workshop', 2, 3, 1]
		]);
	});

	test('reuses a lane once the earlier event has ended', () => {
		const week = calendarGridWeek(new Date(2026, 6, 26));
		const layout = calendarGridWeekLayout(week, [
			event({ id: 'first', start: new Date(2026, 6, 26), end: new Date(2026, 6, 28), isAllDay: true }),
			event({ id: 'second', start: new Date(2026, 6, 29), end: new Date(2026, 6, 31), isAllDay: true })
		]);

		expect(layout.spans.map((span) => span.lane)).toEqual([0, 0]);
	});

	test('marks events that continue past the week edges', () => {
		const week = calendarGridWeek(new Date(2026, 6, 26));
		const layout = calendarGridWeekLayout(week, [
			event({ id: 'long', start: new Date(2026, 6, 20), end: new Date(2026, 7, 5), isAllDay: true })
		]);

		expect(layout.spans[0]).toMatchObject({ startColumn: 0, columnCount: 7, continuesBefore: true, continuesAfter: true });
	});

	test('keeps single day timed events out of the span lanes', () => {
		const week = calendarGridWeek(new Date(2026, 6, 26));
		const layout = calendarGridWeekLayout(week, [
			event({ id: 'standup', start: new Date(2026, 6, 29, 9), end: new Date(2026, 6, 29, 9, 30) })
		]);

		expect(layout.spans).toHaveLength(0);
		expect(layout.timedEntries).toEqual([{ event: expect.objectContaining({ id: 'standup' }), dayIndex: 3 }]);
	});
});

describe('calendar grid timed blocks', () => {
	test('splits overlapping events into side by side columns', () => {
		const blocks = calendarGridTimedBlocks(new Date(2026, 6, 29), [
			event({ id: 'review', start: new Date(2026, 6, 29, 9), end: new Date(2026, 6, 29, 10) }),
			event({ id: 'sync', start: new Date(2026, 6, 29, 9, 30), end: new Date(2026, 6, 29, 10, 30) })
		]);

		expect(blocks.map((block) => [block.event.id, block.column, block.columnCount])).toEqual([
			['review', 0, 2],
			['sync', 1, 2]
		]);
	});

	test('gives back to back events the full width', () => {
		const blocks = calendarGridTimedBlocks(new Date(2026, 6, 29), [
			event({ id: 'first', start: new Date(2026, 6, 29, 9), end: new Date(2026, 6, 29, 10) }),
			event({ id: 'second', start: new Date(2026, 6, 29, 10), end: new Date(2026, 6, 29, 11) })
		]);

		expect(blocks.every((block) => block.columnCount === 1)).toBe(true);
	});

	test('clamps events that reach past the day to the day bounds', () => {
		const blocks = calendarGridTimedBlocks(new Date(2026, 6, 29), [
			event({ id: 'overnight', start: new Date(2026, 6, 28, 22), end: new Date(2026, 6, 29, 2) })
		]);

		expect(blocks[0]).toMatchObject({ startMinutes: 0, endMinutes: 120 });
	});
});
