import { describe, expect, test } from 'bun:test';
import {
	createDevAttendanceMockResponse,
	createDevAttendanceMockState
} from '../../../dev-attendance-mock-plugin';
import { todayDateInTimeZone } from '../../../src/routes/attendance/shared/attendance-date';
import type { AttendanceAbsence, AttendanceSummary } from '../../../src/routes/attendance/attendance-context.svelte';

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

	test('includes a multiple-location current-day scenario for the development user', async () => {
		const today = todayDateInTimeZone('Asia/Seoul', new Date());
		const state = createDevAttendanceMockState('kim@example.com');
		const response = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/summary',
			searchParams: new URLSearchParams(`month=${today.slice(0, 7)}`)
		});

		expect(response?.status).toBe(200);
		const body = response?.body as AttendanceSummary | undefined;
		const events = body?.events
			.filter((event) => event.email === 'kim@example.com' && event.localDate === today)
			.map((event) => ({
				kind: event.kind,
				localTime: event.localTime,
				locationID: event.locationID
			}));

		expect(events).toEqual([
			{ kind: 'clock_in', localTime: '08:30', locationID: 'remote' },
			{ kind: 'clock_out', localTime: '10:20', locationID: 'remote' },
			{ kind: 'clock_in', localTime: '10:45', locationID: 'office' },
			{ kind: 'clock_out', localTime: '12:20', locationID: 'office' },
			{ kind: 'clock_in', localTime: '12:45', locationID: 'outside' }
		]);
	});

	test('includes a completed three-location workday fixture', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const response = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/summary',
			searchParams: new URLSearchParams('month=2026-05')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as AttendanceSummary | undefined;
		const events = body?.events
			.filter((event) => event.email === 'kim@example.com' && event.localDate === '2026-05-19')
			.map((event) => ({
				kind: event.kind,
				localTime: event.localTime,
				locationID: event.locationID
			}));

		expect(events).toEqual([
			{ kind: 'clock_in', localTime: '08:30', locationID: 'remote' },
			{ kind: 'clock_out', localTime: '10:20', locationID: 'remote' },
			{ kind: 'clock_in', localTime: '10:45', locationID: 'office' },
			{ kind: 'clock_out', localTime: '12:20', locationID: 'office' },
			{ kind: 'clock_in', localTime: '13:00', locationID: 'outside' },
			{ kind: 'clock_out', localTime: '17:30', locationID: 'outside' }
		]);
	});

	test('includes varied short and multi-day absence fixture records', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const response = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/summary',
			searchParams: new URLSearchParams('month=2026-05')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as AttendanceSummary | undefined;
		const absences = body?.absences ?? [];
		const absenceKinds = new Set(absences.map((absence) => absence.kind));
		const parkLeaveDates = absences
			.filter((absence) => absence.email === 'park@example.com' && absence.kind === 'leave')
			.map((absence) => absence.date);
		const choiBusinessTripDates = absences
			.filter((absence) => absence.email === 'choi@example.com' && absence.kind === 'business_trip')
			.map((absence) => absence.date);

		expect(absences.length >= 10).toBe(true);
		expect(absenceKinds).toEqual(new Set(['business_trip', 'leave', 'other']));
		expect(parkLeaveDates).toEqual(['2026-05-06', '2026-05-07', '2026-05-08']);
		expect(choiBusinessTripDates).toEqual(['2026-05-12', '2026-05-13']);
		expect(absences.some((absence) => absence.email === 'jung@example.com' && absence.kind === 'other' && absence.date === '2026-05-18')).toBe(true);
		expect(absences.some((absence) => absence.email === 'lee@example.com' && absence.kind === 'leave' && absence.date === '2026-05-27')).toBe(true);
	});

	test('includes overlapping June absence fixture records', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const response = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/summary',
			searchParams: new URLSearchParams('month=2026-06')
		});

		expect(response?.status).toBe(200);
		const body = response?.body as AttendanceSummary | undefined;
		const absences = body?.absences ?? [];
		const overlappingAbsences = absences
			.filter((absence) => absence.date === '2026-06-10')
			.map((absence) => `${absence.email}:${absence.kind}`)
			.sort();
		const parkBusinessTripDates = absences
			.filter((absence) => absence.email === 'park@example.com' && absence.kind === 'business_trip')
			.map((absence) => absence.date);
		const leeOtherDates = absences
			.filter((absence) => absence.email === 'lee@example.com' && absence.kind === 'other' && absence.rangeID === 'absence-june-lee-other')
			.map((absence) => absence.date);

		expect(overlappingAbsences).toEqual([
			'choi@example.com:other',
			'kim@example.com:leave',
			'lee@example.com:business_trip',
		]);
		expect(parkBusinessTripDates).toEqual([
			'2026-06-18',
			'2026-06-19',
			'2026-06-22',
			'2026-06-23',
			'2026-06-24',
			'2026-06-25',
			'2026-06-26',
			'2026-06-29',
			'2026-06-30',
		]);
		expect(leeOtherDates).toEqual(['2026-06-24', '2026-06-25', '2026-06-26']);
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
