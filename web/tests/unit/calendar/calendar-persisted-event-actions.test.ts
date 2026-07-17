import { expect, test } from 'bun:test';

import { dayFlowEventFromCalendarEvent } from '../../../src/routes/calendar/embed/calendar-event-mapping';
import { createCalendarPersistedEventActions } from '../../../src/routes/calendar/embed/calendar-persisted-event-actions';
import { CalendarProgrammaticUpdateState } from '../../../src/routes/calendar/embed/calendar-programmatic-updates';
import { calendarServerEvent } from './calendar-event-persistence-scenario';

test('includes the current server version only in PUT payloads', async () => {
	const originalFetch = globalThis.fetch;
	const requestBodies: string[] = [];
	const serverEvent = {
		...calendarServerEvent('versioned-write', 'Versioned event'),
		updatedAt: '2026-07-17T03:04:05Z'
	};
	const event = dayFlowEventFromCalendarEvent(serverEvent);
	globalThis.fetch = Object.assign(
		async (_input: RequestInfo | URL, init?: RequestInit) => {
			if (typeof init?.body !== 'string') throw new Error('calendar write request body is missing');
			requestBodies.push(init.body);
			return new Response(JSON.stringify(serverEvent), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			});
		},
		{ preconnect: originalFetch.preconnect }
	);
	const actions = createCalendarPersistedEventActions(
		{
			getCalendarEvents: () => [event],
			updateCalendarEvent: async () => {},
			setVisibleEvents: () => {},
			text: {
				deleteError: 'Could not delete the event.',
				saveError: 'Could not save the event.'
			}
		},
		new CalendarProgrammaticUpdateState()
	);

	try {
		await actions.writeEvent(
			'/calendar/api/events/versioned-write',
			'PUT',
			event,
			undefined,
			'page-client',
			7
		);
		await actions.writeEvent(
			'/calendar/api/events/versioned-write',
			'PUT',
			event,
			'2026-07-17T04:05:06Z'
		);
		await actions.writeEvent('/calendar/api/events', 'POST', event);
	} finally {
		globalThis.fetch = originalFetch;
	}

	expect(JSON.parse(requestBodies[0] ?? '')).toMatchObject({
		expectedUpdatedAt: '2026-07-17T03:04:05Z',
		mutationClientID: 'page-client',
		mutationSequence: 7
	});
	expect(JSON.parse(requestBodies[1] ?? '')).toMatchObject({
		expectedUpdatedAt: '2026-07-17T04:05:06Z'
	});
	const postDocument: unknown = JSON.parse(requestBodies[2] ?? '');
	if (!postDocument || typeof postDocument !== 'object') {
		throw new Error('calendar POST request body is not an object');
	}
	expect('expectedUpdatedAt' in postDocument).toBe(false);
	expect('mutationClientID' in postDocument).toBe(false);
	expect('mutationSequence' in postDocument).toBe(false);
});

test('includes the expected persisted version in direct DELETE payloads', async () => {
	const originalFetch = globalThis.fetch;
	const requests: Array<{ method: string | undefined; body: BodyInit | null | undefined; keepalive: boolean | undefined }> = [];
	globalThis.fetch = Object.assign(
		async (_input: RequestInfo | URL, init?: RequestInit) => {
			requests.push({ method: init?.method, body: init?.body, keepalive: init?.keepalive });
			return new Response(null, { status: 204 });
		},
		{ preconnect: originalFetch.preconnect }
	);
	const event = dayFlowEventFromCalendarEvent(calendarServerEvent('delete-version', 'Delete version'));
	const actions = createCalendarPersistedEventActions(
		{
			getCalendarEvents: () => [event],
			updateCalendarEvent: async () => {},
			setVisibleEvents: () => {},
			text: {
				deleteError: 'Could not delete the event.',
				saveError: 'Could not save the event.'
			}
		},
		new CalendarProgrammaticUpdateState()
	);

	try {
		await actions.deleteEvent('delete-version', '2026-07-17T05:00:00Z');
	} finally {
		globalThis.fetch = originalFetch;
	}

	expect(requests).toEqual([
		{
			method: 'DELETE',
			body: JSON.stringify({ expectedUpdatedAt: '2026-07-17T05:00:00Z' }),
			keepalive: undefined
		}
	]);
});

test('creates and cancels a delete intent with keepalive and the same operation ID', async () => {
	const originalFetch = globalThis.fetch;
	const operationID = 'delete-operation';
	const executeAt = '2026-07-17T05:00:05Z';
	const requests: Array<{
		url: string;
		method: string | undefined;
		body: BodyInit | null | undefined;
		keepalive: boolean | undefined;
	}> = [];
	globalThis.fetch = Object.assign(
		async (input: RequestInfo | URL, init?: RequestInit) => {
			requests.push({ url: input.toString(), method: init?.method, body: init?.body, keepalive: init?.keepalive });
			if (init?.method === 'PUT') {
				return new Response(JSON.stringify({ operationID, executeAt }), {
					status: 202,
					headers: { 'Content-Type': 'application/json' }
				});
			}
			return new Response(null, { status: 204 });
		},
		{ preconnect: originalFetch.preconnect }
	);
	const event = dayFlowEventFromCalendarEvent(calendarServerEvent('delete-intent-event', 'Delete intent'));
	const actions = createCalendarPersistedEventActions(
		{
			getCalendarEvents: () => [event],
			updateCalendarEvent: async () => {},
			setVisibleEvents: () => {},
			text: {
				deleteError: 'Could not delete the event.',
				saveError: 'Could not save the event.'
			}
		},
		new CalendarProgrammaticUpdateState()
	);
	let response: { operationID: string; executeAt: string };

	try {
		response = await actions.createDeleteIntent(
			'delete-intent-event',
			operationID,
			'page-client',
			8,
			'2026-07-17T05:00:00Z'
		);
		await actions.cancelDeleteIntent('delete-intent-event', operationID, 'page-client', 9);
	} finally {
		globalThis.fetch = originalFetch;
	}

	expect(response).toEqual({ operationID, executeAt });
	expect(requests).toEqual([
		{
			url: '/calendar/api/events/delete-intent-event/delete-intents/delete-operation',
			method: 'PUT',
			body: JSON.stringify({
				clientID: 'page-client',
				sequence: 8,
				expectedUpdatedAt: '2026-07-17T05:00:00Z'
			}),
			keepalive: true
		},
		{
			url: '/calendar/api/events/delete-intent-event/delete-intents/delete-operation',
			method: 'DELETE',
			body: JSON.stringify({ clientID: 'page-client', sequence: 9 }),
			keepalive: true
		}
	]);
});
