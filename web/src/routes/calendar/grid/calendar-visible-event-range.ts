import { addCalendarGridDays, startOfCalendarGridDay } from './calendar-grid-dates';

export type CalendarVisibleDateRange = { start: Date; end: Date };

export function calendarDateRangeForColumns(days: Date[], firstIndex: number, columnCount: number): CalendarVisibleDateRange | null {
	const first = days[firstIndex];
	const last = days[Math.min(days.length - 1, firstIndex + columnCount - 1)];
	return first && last ? { start: startOfCalendarGridDay(first), end: addCalendarGridDays(startOfCalendarGridDay(last), 1) } : null;
}

export function calendarRangeHasEvents(events: { start: Date; end: Date }[], range: CalendarVisibleDateRange | null): boolean {
	// The month scroller reports its actual visible weeks after mounting or moving.
	// Until that geometry is known, do not claim that the visible result is empty.
	return range === null || events.some(event => event.start < range.end && event.end > range.start);
}
