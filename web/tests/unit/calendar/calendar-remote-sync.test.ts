import { expect, test } from 'bun:test';

import { syncRemoteCalendarAndRefreshConflicts } from '../../../src/routes/calendar/embed/calendar-remote-sync';

test('loads conflicts after remote sync and calendar refresh complete', async () => {
	const originalFetch = globalThis.fetch;
	const calls: string[] = [];
	try {
		globalThis.fetch = Object.assign(
			async (input: RequestInfo | URL): Promise<Response> => {
				calls.push(`fetch:${String(input)}`);
				return new Response('{}', { status: 200 });
			},
			{ preconnect: originalFetch.preconnect }
		);

		await syncRemoteCalendarAndRefreshConflicts(
			'Remote sync failed',
			async () => {
				calls.push('refresh');
			},
			async () => {
				calls.push('conflicts');
			}
		);
	} finally {
		globalThis.fetch = originalFetch;
	}

	expect(calls).toEqual(['fetch:/calendar/api/remote-sync', 'refresh', 'conflicts']);
});

test('loads conflicts after remote sync fails', async () => {
	const originalFetch = globalThis.fetch;
	const calls: string[] = [];
	try {
		globalThis.fetch = Object.assign(
			async (input: RequestInfo | URL): Promise<Response> => {
				calls.push(`fetch:${String(input)}`);
				return new Response('Unauthorized', { status: 401 });
			},
			{ preconnect: originalFetch.preconnect }
		);

		let caughtError: unknown;
		try {
			await syncRemoteCalendarAndRefreshConflicts(
				'Remote sync failed',
				async () => {
					calls.push('refresh');
				},
				async () => {
					calls.push('conflicts');
				}
			);
		} catch (error) {
			caughtError = error;
		}

		expect(caughtError instanceof Error).toBe(true);
		expect(caughtError instanceof Error ? caughtError.message : '').toBe('Unauthorized');
	} finally {
		globalThis.fetch = originalFetch;
	}

	expect(calls).toEqual(['fetch:/calendar/api/remote-sync', 'conflicts']);
});
