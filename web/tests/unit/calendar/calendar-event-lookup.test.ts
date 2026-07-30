import { createCalendarModelEvent as createEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import { expect, test } from 'bun:test';

import { findCalendarEventByID } from '../../../src/routes/calendar/embed/calendar-event-lookup';

test('finds events from the DayFlow app store before the fallback array', () => {
	const appEvent = createEvent({
		id: 'target-event',
		title: 'App Store Event',
		start: new Date(2026, 5, 16, 9),
		end: new Date(2026, 5, 16, 10),
		calendarId: 'internkim'
	});
	const fallbackEvent = createEvent({
		id: 'target-event',
		title: 'Fallback Event',
		start: new Date(2026, 5, 16, 11),
		end: new Date(2026, 5, 16, 12),
		calendarId: 'internkim'
	});

	expect(findCalendarEventByID([appEvent], [fallbackEvent], 'target-event')).toBe(appEvent);
});

test('falls back to calendar events when the DayFlow app store has not caught up', () => {
	const fallbackEvent = createEvent({
		id: 'fallback-only-event',
		title: 'Fallback Only Event',
		start: new Date(2026, 5, 16, 13),
		end: new Date(2026, 5, 16, 14),
		calendarId: 'internkim'
	});

	expect(findCalendarEventByID([], [fallbackEvent], 'fallback-only-event')).toBe(fallbackEvent);
});
