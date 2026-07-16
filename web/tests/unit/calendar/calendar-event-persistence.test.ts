import { expect, test } from 'bun:test';

import {
	CalendarPersistenceError,
	calendarPersistenceErrorFromResponse
} from '../../../src/routes/calendar/embed/calendar-event-persistence';

test('decodes the target unavailable response into the closed error code', async () => {
	const errorValue = await calendarPersistenceErrorFromResponse(
		new Response(JSON.stringify({ code: 'calendar_target_unavailable' }), {
			status: 409,
			headers: { 'Content-Type': 'application/json' }
		}),
		'Could not save the event.'
	);

	expect(errorValue instanceof CalendarPersistenceError).toBe(true);
	expect(errorValue.code).toBe('calendar_target_unavailable');
	expect(errorValue.message).toBe('Could not save the event.');
});

test('decodes an event version conflict into the closed error code', async () => {
	const errorValue = await calendarPersistenceErrorFromResponse(
		new Response(JSON.stringify({ code: 'calendar_event_version_conflict' }), {
			status: 409,
			headers: { 'Content-Type': 'application/json' }
		}),
		'Could not save the event.'
	);

	expect(errorValue.code).toBe('calendar_event_version_conflict');
	expect(errorValue.message).toBe('Could not save the event.');
});

test('does not admit unknown server codes into the typed error union', async () => {
	const errorValue = await calendarPersistenceErrorFromResponse(
		new Response(JSON.stringify({ code: 'account_google-test' }), {
			status: 409,
			headers: { 'Content-Type': 'application/json' }
		}),
		'Could not save the event.'
	);

	expect(errorValue.code).toBe('unknown');
	expect(errorValue.message).toBe('Could not save the event.');
});

test('uses the localized fallback instead of exposing a plain server error', async () => {
	const errorValue = await calendarPersistenceErrorFromResponse(
		new Response('database is locked', {
			status: 500,
			headers: { 'Content-Type': 'text/plain' }
		}),
		'Could not save the event.'
	);

	expect(errorValue.code).toBe('unknown');
	expect(errorValue.message).toBe('Could not save the event.');
});
