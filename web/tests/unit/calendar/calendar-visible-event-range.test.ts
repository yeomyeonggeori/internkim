import { expect, test } from 'bun:test';
import { calendarDateRangeForColumns, calendarRangeHasEvents } from '../../../src/routes/calendar/grid/calendar-visible-event-range';

const date = new Date(2026, 9, 6, 12);
const nextMonth = { start: new Date(2026, 10, 10, 10), end: new Date(2026, 10, 10, 11) };
test('cached events outside the displayed day or week do not prevent filtered-empty feedback', () => {
	for (const days of [[date], Array.from({ length: 7 }, (_, index) => new Date(2026, 9, 4 + index))]) {
		const range = calendarDateRangeForColumns(days, 0, days.length);
		expect(calendarRangeHasEvents([nextMonth], range)).toBe(false);
		expect(calendarRangeHasEvents([{ start: new Date(2026, 9, 6, 10), end: new Date(2026, 9, 6, 11) }], range)).toBe(true);
	}
});
test('month filtering follows the actually visible weeks and does not guess before geometry is known', () => {
	expect(calendarRangeHasEvents([], null)).toBe(true);
	const range = { start: new Date(2026, 8, 27), end: new Date(2026, 10, 8) };
	expect(calendarRangeHasEvents([nextMonth], range)).toBe(false);
	expect(calendarRangeHasEvents([{ start: new Date(2026, 8, 26), end: new Date(2026, 8, 28) }], range)).toBe(true);
	expect(calendarRangeHasEvents([{ start: new Date(2026, 8, 26), end: new Date(2026, 8, 27) }], range)).toBe(false);
});
test('a week scrolled by one day includes its trailing Sunday and excludes the preceding one', () => {
	const days = Array.from({ length: 21 }, (_, index) => new Date(2026, 8, 27 + index));
	const range = calendarDateRangeForColumns(days, 8, 7);
	expect(calendarRangeHasEvents([{ start: new Date(2026, 9, 11, 10), end: new Date(2026, 9, 11, 11) }], range)).toBe(true);
	expect(calendarRangeHasEvents([{ start: new Date(2026, 9, 4, 10), end: new Date(2026, 9, 4, 11) }], range)).toBe(false);
});
