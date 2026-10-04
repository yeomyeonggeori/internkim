import { expect, test } from 'bun:test';
import { createCalendarEventLoader } from '../../../src/routes/calendar/embed/calendar-event-loader';
import type { CalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-persistence';
import type { CalendarHolidayLoadResult } from '../../../src/routes/calendar/embed/calendar-holiday-persistence';
import type { CalendarModelEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import { dayTaskEventFromCalendarHoliday } from '../../../src/routes/calendar/embed/calendar-event-mapping';

function deferred<T>() {
	let resolve: (value: T) => void = () => {};
	let reject: (reason: Error) => void = () => {};
	const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail; });
	return { promise, resolve, reject };
}

const start = new Date('2026-07-01T00:00:00Z');
const end = new Date('2026-08-01T00:00:00Z');
const event: CalendarEvent = {
	id: 'event', uid: 'event', title: 'Meeting', description: '', location: '',
	startISO: '2026-07-16T01:00:00Z', endISO: '2026-07-16T02:00:00Z', timeZone: 'UTC',
	isAllDay: false, color: '', createdByEmail: '', createdByName: '', updatedAt: ''
};
const holidays: CalendarHolidayLoadResult = {
	holidays: [{ id: 'holiday', title: 'Holiday', date: '2026-07-17', source: 'holiday_api', readOnly: true, color: '' }],
	degraded: false
};

function scenario() {
	const pendingEvents = deferred<CalendarEvent[]>();
	const pendingHolidays = deferred<CalendarHolidayLoadResult>();
	const rendered = deferred<void>();
	let requestedEvents = pendingEvents.promise;
	let requestedHolidays = pendingHolidays.promise;
	let visible: CalendarModelEvent[] = [];
	let loading = false;
	let error = '';
	const loader = createCalendarEventLoader({
		isBrowser: () => true, getLocale: () => 'en', errorFallback: () => 'events failed',
		holidayErrorFallback: () => 'holidays failed', getCalendarEvents: () => visible,
		getVisibleEvents: () => visible, applyCalendarEventsChanges: () => {},
		triggerCalendarRender: () => {}, setVisibleEvents: (events) => { visible = events; },
		setEventCount: () => {}, setIsLoading: (value) => { loading = value; },
		setErrorMessage: (value) => { error = value; }, refreshSelectedMonthDateCell: () => {},
		afterRenderEvents: () => rendered.resolve()
	}, {
		fetchEvents: () => requestedEvents,
		fetchHolidays: () => requestedHolidays
	});
	return { loader, pendingEvents, pendingHolidays, rendered: rendered.promise,
		visible: () => visible, loading: () => loading, error: () => error,
		seedHolidays: () => { visible = holidays.holidays.map(dayTaskEventFromCalendarHoliday); },
		edit: () => { visible = visible.map((item) => ({ ...item, title: 'Edited locally' })); },
		next: () => {
			requestedEvents = Promise.resolve([{ ...event, title: 'Newer event' }]);
			requestedHolidays = Promise.resolve({ holidays: [], degraded: false });
		}
	};
}

test('renders events and stops primary loading before slow holidays resolve', async () => {
	const state = scenario();
	const reading = state.loader.loadEvents(start, end);
	state.pendingEvents.resolve([event]);
	await state.rendered;
	expect(state.visible().map((item) => item.title)).toEqual(['Meeting']);
	expect(state.loading()).toBe(false);
	state.pendingHolidays.resolve(holidays);
	await reading;
	expect(state.visible().map((item) => item.title)).toEqual(['Meeting', 'Holiday']);
});

test('merges late holidays without overwriting a local edit', async () => {
	const state = scenario();
	const reading = state.loader.loadEvents(start, end);
	state.pendingEvents.resolve([event]);
	await state.rendered;
	state.edit();
	state.loader.invalidatePendingLoad(true);
	state.pendingHolidays.resolve(holidays);
	await reading;
	expect(state.visible().map((item) => item.title)).toEqual(['Edited locally', 'Holiday']);
});

test('late holiday failure keeps the already displayed events and reports a warning', async () => {
	const state = scenario();
	const reading = state.loader.loadEvents(start, end);
	state.pendingEvents.resolve([event]);
	await state.rendered;
	state.pendingHolidays.reject(new Error('provider unavailable'));
	await reading;
	expect(state.visible().map((item) => item.title)).toEqual(['Meeting']);
	expect(state.error()).toBe('holidays failed');
});

test('retains previous holidays until the refreshed holiday result replaces them', async () => {
	const state = scenario();
	state.seedHolidays();
	const reading = state.loader.loadEvents(start, end);
	state.pendingEvents.resolve([event]);
	await state.rendered;
	expect(state.visible().map((item) => item.title)).toEqual(['Holiday', 'Meeting']);
	state.pendingHolidays.resolve({ holidays: [], degraded: false });
	await reading;
	expect(state.visible().map((item) => item.title)).toEqual(['Meeting']);
});

test('unmount still cancels a holiday read after a local edit invalidated only events', async () => {
	const state = scenario();
	const reading = state.loader.loadEvents(start, end);
	state.pendingEvents.resolve([event]);
	await state.rendered;
	state.edit();
	state.loader.invalidatePendingLoad(true);
	state.loader.invalidatePendingLoad();
	state.pendingHolidays.resolve(holidays);
	await reading;
	expect(state.visible().map((item) => item.title)).toEqual(['Edited locally']);
});

test('a newer range refresh cannot receive an older holiday result or warning', async () => {
	const state = scenario();
	const oldReading = state.loader.loadEvents(start, end);
	state.pendingEvents.resolve([event]);
	await state.rendered;
	state.next();
	await state.loader.loadEvents(start, end);
	state.pendingHolidays.resolve({ ...holidays, degraded: true });
	await oldReading;
	expect(state.visible().map((item) => item.title)).toEqual(['Newer event']);
	expect(state.error()).toBe('');
});

test('event failures finish without waiting for a holiday provider', async () => {
	const state = scenario();
	const reading = state.loader.loadEvents(start, end);
	state.pendingEvents.reject(new Error('event API failed'));
	await reading;
	expect(state.error()).toBe('event API failed');
	expect(state.loading()).toBe(false);
	state.pendingHolidays.resolve(holidays);
	await state.pendingHolidays.promise;
	expect(state.visible()).toEqual([]);
});
