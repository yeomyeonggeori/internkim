import { expect, test } from 'bun:test';
import { dismissCalendarConflictOnServer } from '../../../src/routes/calendar/embed/calendar-conflicts';

test('throws server message when dismiss conflict fails', async () => {
	const originalFetch = globalThis.fetch;

	try {
		globalThis.fetch = Object.assign(
			async (): Promise<Response> =>
				new Response('dismiss failed', {
					status: 500
				}),
			{ preconnect: originalFetch.preconnect }
		);

		let caughtError: unknown;
		try {
			await dismissCalendarConflictOnServer(12);
		} catch (error) {
			caughtError = error;
		}

		expect(caughtError instanceof Error).toBe(true);
		expect(caughtError instanceof Error ? caughtError.message : '').toBe('dismiss failed');
	} finally {
		globalThis.fetch = originalFetch;
	}
});
