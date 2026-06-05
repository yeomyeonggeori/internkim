import { expect, test } from 'bun:test';
import { dismissCalendarConflictOnServer } from '../../../src/routes/calendar/embed/calendar-conflicts';

test('throws server message when dismiss conflict fails', async () => {
	const originalFetch = globalThis.fetch;

	try {
		const mockFetch: typeof fetch = async () =>
			new Response('dismiss failed', {
				status: 500
			});
		globalThis.fetch = mockFetch;

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
