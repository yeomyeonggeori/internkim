import { expect, test } from 'bun:test';

import {
	CalendarPersistenceError,
	calendarPersistenceErrorFromResponse,
	createCalendarEventDeleteIntent
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

test('rejects invalid delete intent response fields with the localized fallback', async () => {
	const originalFetch = globalThis.fetch;
	const responseDocuments: unknown[] = [
		{ operationID: 'operation-without-execute-at' },
		{ operationID: 7, executeAt: '2026-07-17T05:00:05Z' },
		{ operationID: '   ', executeAt: '2026-07-17T05:00:05Z' },
		{ operationID: 'different-operation', executeAt: '2026-07-17T05:00:05Z' },
		{ operationID: 'operation-4', executeAt: 'not-a-timestamp' }
	];
	let responseIndex = 0;
	const errors: unknown[] = [];
	globalThis.fetch = Object.assign(
		async () => new Response(JSON.stringify(responseDocuments[responseIndex++])),
		{ preconnect: originalFetch.preconnect }
	);

	try {
		for (let attempt = 0; attempt < responseDocuments.length; attempt += 1) {
			try {
				await createCalendarEventDeleteIntent(
					'delete-intent-response-event',
					`operation-${attempt}`,
					'page-client',
					attempt + 1,
					'2026-07-17T05:00:00Z',
					'Could not delete the event.'
				);
			} catch (error: unknown) {
				errors.push(error);
			}
		}
	} finally {
		globalThis.fetch = originalFetch;
	}

	expect(
		errors.map((error) =>
			error instanceof CalendarPersistenceError
				? { code: error.code, message: error.message }
				: null
		)
	).toEqual([
		{ code: 'unknown', message: 'Could not delete the event.' },
		{ code: 'unknown', message: 'Could not delete the event.' },
		{ code: 'unknown', message: 'Could not delete the event.' },
		{ code: 'unknown', message: 'Could not delete the event.' },
		{ code: 'unknown', message: 'Could not delete the event.' }
	]);
});
