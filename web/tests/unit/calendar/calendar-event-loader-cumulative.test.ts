import { describe, expect, test } from 'bun:test';
import { createCalendarEventLoader } from '../../../src/routes/calendar/embed/calendar-event-loader';
import type { CalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-persistence';
import type { CalendarModelEvent } from '../../../src/routes/calendar/embed/calendar-event-model';

function calendarEvent(id: string, startISO: string): CalendarEvent {
	return {
		id,
		uid: id,
		title: id,
		description: '',
		location: '',
		startISO,
		endISO: startISO,
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '#2563eb',
		participants: [],
		createdByEmail: 'admin@example.com',
		createdByName: 'admin',
		updatedAt: startISO
	};
}

function createLoader(fetchedEvents: CalendarEvent[]) {
	let visibleEvents: CalendarModelEvent[] = [];
	let calendarEvents: CalendarModelEvent[] = [];
	const loader = createCalendarEventLoader(
		{
			isBrowser: () => true,
			getLocale: () => 'ko',
			errorFallback: () => 'error',
			getCalendarEvents: () => calendarEvents,
			getVisibleEvents: () => visibleEvents,
			applyCalendarEventsChanges: (changes) => {
				calendarEvents = [
					...calendarEvents.filter((event) => !changes.delete.includes(event.id)),
					...changes.add
				];
			},
			triggerCalendarRender: () => {},
			setVisibleEvents: (events) => {
				visibleEvents = events;
			},
			setEventCount: () => {},
			setIsLoading: () => {},
			setErrorMessage: () => {},
			refreshSelectedMonthDateCell: () => {}
		},
		{ fetchEvents: async () => fetchedEvents }
	);
	return { loader, visibleEvents: () => visibleEvents };
}

describe('calendar event loader window', () => {
	test('keeps events loaded outside the newly fetched range', async () => {
		const january = createLoader([calendarEvent('january', '2026-01-10T01:00:00+09:00')]);
		await january.loader.loadEvents(new Date(2026, 0, 1), new Date(2026, 1, 1));
		expect(january.visibleEvents().map((event) => event.id)).toEqual(['january']);
	});

	test('replaces only the events inside the fetched range', async () => {
		const march = createLoader([calendarEvent('march', '2026-03-10T01:00:00+09:00')]);
		await march.loader.loadEvents(new Date(2026, 2, 1), new Date(2026, 3, 1));
		expect(march.visibleEvents().map((event) => event.id)).toEqual(['march']);
		await march.loader.loadEvents(new Date(2026, 2, 1), new Date(2026, 3, 1));
		expect(march.visibleEvents().map((event) => event.id)).toEqual(['march']);
	});
});
