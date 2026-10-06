import { expect, test } from 'bun:test';
import { ToolRefused } from '../../../src/lib/public-api-call';
import { createCalendarEventLoader } from '../../../src/routes/calendar/embed/calendar-event-loader';
import type { CalendarModelEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import type { CalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-persistence';

const event: CalendarEvent = { id: 'sample-event', uid: 'sample-event', title: 'Sample meeting', description: '', location: '', startISO: '2026-10-06T01:00:00Z', endISO: '2026-10-06T02:00:00Z', timeZone: 'UTC', isAllDay: false, color: '', createdByEmail: '', createdByName: '', updatedAt: '' };
const from = new Date('2026-10-01T00:00:00Z');
const to = new Date('2026-11-01T00:00:00Z');

test('first range waits, current range refresh retains content, and failures settle', async () => {
	let visible: CalendarModelEvent[] = [];
	let initial = false;
	let loading = false;
	let error = '';
	let gate = Promise.withResolvers<CalendarEvent[]>();
	const loader = createCalendarEventLoader({
		isBrowser: () => true, getLocale: () => 'en', errorFallback: () => 'failed', holidayErrorFallback: () => 'holidays failed',
		getCalendarEvents: () => visible, getVisibleEvents: () => visible,
		applyCalendarEventsChanges: () => {}, triggerCalendarRender: () => {},
		setVisibleEvents: value => { visible = value; }, setEventCount: () => {},
		setIsLoading: value => { loading = value; }, setIsInitialLoading: value => { initial = value; },
		setErrorMessage: value => { error = value; }, refreshSelectedMonthDateCell: () => {}
	}, { fetchEvents: () => gate.promise, fetchHolidays: async () => ({ holidays: [], degraded: false }) });
	const first = loader.loadEvents(from, to);
	expect(initial).toBe(true);
	expect(loading).toBe(true);
	gate.resolve([event]);
	await first;
	expect(initial).toBe(false);
	expect(visible[0].title).toBe('Sample meeting');
	gate = Promise.withResolvers<CalendarEvent[]>();
	const refresh = loader.refreshCurrentRange();
	expect(initial).toBe(false);
	expect(loading).toBe(true);
	expect(visible[0].title).toBe('Sample meeting');
	gate.reject(new Error('temporarily unavailable'));
	await refresh;
	expect(error).toBe('temporarily unavailable');
	expect(initial).toBe(false);
	expect(loading).toBe(false);
	expect(visible[0].title).toBe('Sample meeting');
	gate = Promise.withResolvers<CalendarEvent[]>();
	const nextRange = loader.loadEvents(to, new Date('2026-12-01T00:00:00Z'));
	expect(initial).toBe(true);
	gate.resolve([]);
	await nextRange;
	expect(initial).toBe(false);
	gate = Promise.withResolvers<CalendarEvent[]>();
	const denied = loader.refreshCurrentRange();
	gate.reject(new ToolRefused('forbidden', 'FORBIDDEN', 403));
	await denied;
	expect(visible).toEqual([]);
	gate = Promise.withResolvers<CalendarEvent[]>();
	const retry = loader.loadEvents(from, to);
	expect(initial).toBe(true);
	gate.resolve([]);
	await retry;
});
