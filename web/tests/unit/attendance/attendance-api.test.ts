import { describe, expect, test } from 'bun:test';
import { fetchAttendanceSummary } from '../../../src/routes/attendance/attendance-api';

describe('fetchAttendanceSummary', () => {
	test('omits empty summary query parameters', async () => {
		const originalFetch = globalThis.fetch;

		let requestedURL = '';
		try {
			globalThis.fetch = Object.assign(
				async (input: RequestInfo | URL): Promise<Response> => {
					requestedURL = String(input);
					return Response.json({
						month: '2026-05',
						currentUserEmail: 'me@example.com',
						isAdmin: false,
						timeZone: 'Asia/Seoul',
						events: [],
						todayStatus: 'absent',
						locations: [],
						teamViewVisibleToAll: false,
						teamViewBlocked: true
					});
				},
				{ preconnect: originalFetch.preconnect }
			);

			await fetchAttendanceSummary({ month: '', selectedEmail: '' });

			expect(requestedURL).toBe('/attendance/api/summary');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});
