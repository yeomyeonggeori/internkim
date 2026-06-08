import { expect, test } from 'bun:test';
import { dismissCalendarConflictOnServer } from '../../../src/routes/calendar/embed/calendar-conflicts';
import { createMockFetch } from '../test-fetch';

test('throws server message when dismiss conflict fails', async () => {
	const originalFetch = globalThis.fetch;

	try {
		globalThis.fetch = createMockFetch(async () =>
			new Response('dismiss failed', {
				status: 500
			}));

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
