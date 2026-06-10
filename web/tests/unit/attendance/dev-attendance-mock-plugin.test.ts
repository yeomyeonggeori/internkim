import { describe, expect, test } from 'bun:test';
import {
	createDevAttendanceMockResponse,
	createDevAttendanceMockState
} from '../../../dev-attendance-mock-plugin';
import type { AttendanceAbsence } from '../../../src/routes/attendance/attendance-context.svelte';

describe('dev attendance mock plugin', () => {
	test('returns an authenticated development session', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const response = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/auth/session',
			searchParams: new URLSearchParams()
		});

		expect(response).toEqual({
			status: 200,
			body: { authenticated: true, email: 'kim@example.com', isAdmin: true }
		});
	});

	test('returns attendance summary fixture data', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const response = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/summary',
			searchParams: new URLSearchParams('month=2026-05')
		});

		expect(response?.status).toBe(200);
		expect(response?.body).toMatchObject({
			month: '2026-05',
			currentUserEmail: 'kim@example.com',
			isAdmin: true
		});
		const body = response?.body;
		expect(hasKey(body, 'events')).toBe(true);
		expect(hasKey(body, 'absences')).toBe(true);
	});

	test('registers a development absence for the current mock user', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const response = await createDevAttendanceMockResponse(state, {
			method: 'POST',
			pathname: '/attendance/api/absences',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				kind: 'leave',
				startDate: '2026-05-12',
				endDate: '2026-05-12',
				reason: 'local test',
				email: 'other@example.com'
			})
		});

		expect(response?.status).toBe(200);
		const body = response?.body as { absences: AttendanceAbsence[] } | undefined;
		expect(body?.absences.length).toBe(1);
		expect(body?.absences[0]).toMatchObject({
			email: 'kim@example.com',
			kind: 'leave',
			date: '2026-05-12',
			reason: 'local test'
		});
	});

	test('skips weekend dates in development absence ranges', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const response = await createDevAttendanceMockResponse(state, {
			method: 'POST',
			pathname: '/attendance/api/absences',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				kind: 'leave',
				startDate: '2026-05-15',
				endDate: '2026-05-18'
			})
		});

		expect(response?.status).toBe(200);
		const body = response?.body as { absences: AttendanceAbsence[] } | undefined;
		expect(body?.absences.map((absence) => absence.date)).toEqual(['2026-05-15', '2026-05-18']);
	});
});

function hasKey(value: unknown, key: string): boolean {
	return !!value && typeof value === 'object' && key in value;
}
