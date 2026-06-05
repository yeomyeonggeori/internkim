import { describe, expect, test } from 'bun:test';
import { fetchAttendanceSummary } from '../../../src/routes/attendance/attendance-api';

describe('fetchAttendanceSummary', () => {
	test('omits empty summary query parameters', async () => {
		const originalFetch = globalThis.fetch;

		let requestedURL = '';
		try {
			const mockFetch: typeof fetch = async (input) => {
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
			};
			globalThis.fetch = mockFetch;

			await fetchAttendanceSummary({ month: '', selectedEmail: '' });

			expect(requestedURL).toBe('/attendance/api/summary');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});
