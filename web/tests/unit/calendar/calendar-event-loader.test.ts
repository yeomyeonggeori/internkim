import { createCalendarModelEvent as createEvent, type CalendarModelEvent as DayFlowEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import { expect, test } from 'bun:test';

import { createCalendarEventLoader } from '../../../src/routes/calendar/embed/calendar-event-loader';
import type { CalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-persistence';

test('does not apply a deferred refresh after a newer local update', async () => {
	let resolveFetch: (events: CalendarEvent[]) => void = () => {};
	let reportFetchStarted: () => void = () => {};
	const fetchStarted = new Promise<void>((resolve) => {
		reportFetchStarted = resolve;
	});
	const pendingEvents = new Promise<CalendarEvent[]>((resolve) => {
		resolveFetch = resolve;
	});
	let calendarEvents = [calendarTestEvent('event-1', 'First update')];
	let visibleEvents: typeof calendarEvents = [];
	let appliedChangeCount = 0;
	const loadingStates: boolean[] = [];
	const loader = createCalendarEventLoader(
		{
			isBrowser: () => true,
			errorFallback: () => 'Could not load events.',
			getCalendarEvents: () => calendarEvents,
			getVisibleEvents: () => visibleEvents,
			applyCalendarEventsChanges: (changes) => {
				appliedChangeCount += 1;
				calendarEvents = [
					...calendarEvents.filter((event) => !changes.delete.includes(event.id)),
					...changes.add
				];
			},
			triggerCalendarRender: () => {},
			setVisibleEvents: (events) => {
				calendarEvents = [...events];
			},
			setEventCount: () => {},
			setIsLoading: (isLoading) => {
				loadingStates.push(isLoading);
			},
			setErrorMessage: () => {},
			refreshSelectedMonthDateCell: () => {}
		},
		{
			fetchEvents: async () => {
				reportFetchStarted();
				return pendingEvents;
			},
			fetchHolidays: async () => []
		}
	);

	const refresh = loader.loadEvents(
		new Date('2026-07-01T00:00:00Z'),
		new Date('2026-08-01T00:00:00Z')
	);
	await fetchStarted;
	calendarEvents = [calendarTestEvent('event-1', 'Second update')];
	loader.invalidatePendingLoad();
	resolveFetch([calendarServerEvent('event-1', 'First update')]);
	await refresh;

	expect(calendarEvents.map((event) => event.title)).toEqual(['Second update']);
	expect(appliedChangeCount).toBe(0);
	expect(loadingStates).toEqual([true, false]);
});

function calendarTestEvent(eventID: string, title: string): DayFlowEvent {
	return createEvent({
		id: eventID,
		title,
		start: new Date('2026-07-16T01:00:00Z'),
		end: new Date('2026-07-16T02:00:00Z'),
		allDay: false,
		calendarId: 'internkim'
	});
}

function calendarServerEvent(eventID: string, title: string): CalendarEvent {
	return {
		id: eventID,
		uid: `${eventID}@internkim`,
		title,
		description: '',
		location: '',
		startISO: '2026-07-16T01:00:00Z',
		endISO: '2026-07-16T02:00:00Z',
		timeZone: 'UTC',
		isAllDay: false,
		color: '#2563eb',
		createdByEmail: 'admin@example.com',
		createdByName: 'Admin',
		updatedAt: '2026-07-16T00:00:00Z'
	};
}
