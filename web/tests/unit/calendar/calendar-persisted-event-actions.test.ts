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
		await actions.writeEvent('/calendar/api/events/versioned-write', 'PUT', event);
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
		expectedUpdatedAt: '2026-07-17T03:04:05Z'
	});
	expect(JSON.parse(requestBodies[1] ?? '')).toMatchObject({
		expectedUpdatedAt: '2026-07-17T04:05:06Z'
	});
	const postDocument: unknown = JSON.parse(requestBodies[2] ?? '');
	if (!postDocument || typeof postDocument !== 'object') {
		throw new Error('calendar POST request body is not an object');
	}
	expect('expectedUpdatedAt' in postDocument).toBe(false);
});

test('includes the expected persisted version in normal and keepalive DELETE payloads', async () => {
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
		actions.deleteEventOnPageHide('delete-version', '2026-07-17T06:00:00Z');
		await Promise.resolve();
	} finally {
		globalThis.fetch = originalFetch;
	}

	expect(requests).toEqual([
		{
			method: 'DELETE',
			body: JSON.stringify({ expectedUpdatedAt: '2026-07-17T05:00:00Z' }),
			keepalive: undefined
		},
		{
			method: 'DELETE',
			body: JSON.stringify({ expectedUpdatedAt: '2026-07-17T06:00:00Z' }),
			keepalive: true
		}
	]);
});
