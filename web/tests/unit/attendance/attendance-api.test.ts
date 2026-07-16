import { describe, expect, test } from 'bun:test';
import {
	createAttendanceAbsence,
	deleteAttendanceAbsence,
	fetchAttendanceSummary
} from '../../../src/routes/attendance/attendance-api';
import { createMockFetch } from '../test-fetch';

describe('fetchAttendanceSummary', () => {
	test('requests a private response without using the browser cache', async () => {
		const originalFetch = globalThis.fetch;

		let requestOptions: RequestInit | undefined;
		try {
			globalThis.fetch = createMockFetch(async (_input, init) => {
				requestOptions = init;
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
			});

			await fetchAttendanceSummary({ month: '2026-05' });

			expect(requestOptions).toEqual({ credentials: 'include', cache: 'no-store' });
		} finally {
			globalThis.fetch = originalFetch;
		}
	});

	test('omits empty summary query parameters', async () => {
		const originalFetch = globalThis.fetch;

		let requestedURL = '';
		try {
			globalThis.fetch = createMockFetch(async (input) => {
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
			});

			await fetchAttendanceSummary({ month: '' });

			expect(requestedURL).toBe('/attendance/api/summary');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});

describe('createAttendanceAbsence', () => {
	test('posts own absence without an email override', async () => {
		const originalFetch = globalThis.fetch;

		let requestedURL = '';
		let requestBody: Record<string, unknown> = {};
		try {
			globalThis.fetch = createMockFetch(async (input, init) => {
				requestedURL = String(input);
				requestBody = JSON.parse(String(init?.body ?? '{}')) as Record<string, unknown>;
				return Response.json({
					absences: [
						{
							id: 'absence-1',
							email: 'me@example.com',
							kind: 'leave',
							labelKey: 'leave',
							date: '2026-06-10',
							createdAt: '2026-06-01T09:00:00Z'
						}
					]
				});
			});

			await createAttendanceAbsence({
				kind: 'leave',
				startDate: '2026-06-10',
				endDate: '2026-06-11',
				reason: 'family'
			});

			expect(requestedURL).toBe('/attendance/api/absences');
			expect(requestBody).toEqual({
				kind: 'leave',
				startDate: '2026-06-10',
				endDate: '2026-06-11',
				reason: 'family'
			});
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});

describe('deleteAttendanceAbsence', () => {
	test('deletes the selected absence by id', async () => {
		const originalFetch = globalThis.fetch;

		let requestedURL = '';
		let requestMethod = '';
		try {
			globalThis.fetch = createMockFetch(async (input, init) => {
				requestedURL = String(input);
				requestMethod = init?.method ?? '';
				return Response.json({ ok: true });
			});

			await deleteAttendanceAbsence('absence 1');

			expect(requestedURL).toBe('/attendance/api/absences/absence%201');
			expect(requestMethod).toBe('DELETE');
		} finally {
			globalThis.fetch = originalFetch;
		}
	});
});
