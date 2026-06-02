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

		await expect(dismissCalendarConflictOnServer(12)).rejects.toThrow('dismiss failed');
	} finally {
		globalThis.fetch = originalFetch;
	}
});
