import { expect, test } from 'bun:test';

import {
	syncRemoteCalendar,
	syncRemoteCalendarAndRefreshConflicts
} from '../../../src/routes/calendar/embed/calendar-remote-sync';
import { createMockFetch } from '../test-fetch';

test('coalesces concurrent remote sync requests and resets after completion', async () => {
	const originalFetch = globalThis.fetch;
	const calls: string[] = [];
	let completeRemoteSync = (response: Response) => {
		void response;
	};
	try {
		globalThis.fetch = createMockFetch((input) => {
			calls.push(`fetch:${String(input)}`);
			return new Promise<Response>((resolve) => {
				completeRemoteSync = resolve;
			});
		});

		const firstSync = syncRemoteCalendar('Remote sync failed');
		const secondSync = syncRemoteCalendar('Remote sync failed');
		expect(calls).toEqual(['fetch:/calendar/api/remote-sync']);

		completeRemoteSync(new Response('{}', { status: 200 }));
		await Promise.all([firstSync, secondSync]);

		const thirdSync = syncRemoteCalendar('Remote sync failed');
		expect(calls).toEqual(['fetch:/calendar/api/remote-sync', 'fetch:/calendar/api/remote-sync']);
		completeRemoteSync(new Response('{}', { status: 200 }));
		await thirdSync;
	} finally {
		globalThis.fetch = originalFetch;
	}
});

test('loads conflicts after remote sync and calendar refresh complete', async () => {
	const originalFetch = globalThis.fetch;
	const calls: string[] = [];
	try {
		globalThis.fetch = createMockFetch(async (input) => {
			calls.push(`fetch:${String(input)}`);
			return new Response('{}', { status: 200 });
		});

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
		globalThis.fetch = createMockFetch(async (input) => {
			calls.push(`fetch:${String(input)}`);
			return new Response('Unauthorized', { status: 401 });
		});

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
