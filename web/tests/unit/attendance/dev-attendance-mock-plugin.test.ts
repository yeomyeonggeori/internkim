import { describe, expect, test } from 'bun:test';
import {
	createDevAttendanceMockResponse,
	createDevAttendanceMockState
} from '../../../dev-attendance-mock-plugin';
import {
	devPopupOverflowAttendanceRows,
	devPopupOverflowDate,
	devPopupOverflowEmail,
	devPopupOverflowMonth
} from '../../../dev-popup-overflow-fixture';
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

	test('includes a four-segment June workday fixture for popup overflow checks', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const response = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/summary',
			searchParams: new URLSearchParams(`month=${devPopupOverflowMonth}`)
		});

		expect(response?.status).toBe(200);
		const body = response?.body as AttendanceSummary | undefined;
		const events = body?.events
			.filter((event) => event.email === devPopupOverflowEmail && event.localDate === devPopupOverflowDate)
			.map((event) => ({
				kind: event.kind,
				localTime: event.localTime,
				locationID: event.locationID
			}));

		expect(events).toEqual(devPopupOverflowAttendanceRows.map((row) => ({
			kind: row.kind,
			localTime: row.localTime,
			locationID: row.locationID
		})));
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
		const kangOtherDates = absences
			.filter((absence) => absence.email === 'kang@example.com' && absence.kind === 'other')
			.map((absence) => absence.date);

		expect(absences.length >= 7).toBe(true);
		expect(absenceKinds).toEqual(new Set(['leave', 'other']));
		expect(parkLeaveDates).toEqual(['2026-05-06', '2026-05-07', '2026-05-08']);
		expect(kangOtherDates).toEqual(['2026-05-22']);
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
		const leeOtherDates = absences
			.filter((absence) => absence.email === 'lee@example.com' && absence.kind === 'other' && absence.rangeID === 'absence-june-lee-other')
			.map((absence) => absence.date);

		expect(overlappingAbsences).toEqual([
			'choi@example.com:other',
			'kim@example.com:leave',
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

	test('deletes a development absence for the current mock user', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const createResponse = await createDevAttendanceMockResponse(state, {
			method: 'POST',
			pathname: '/attendance/api/absences',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				kind: 'leave',
				startDate: '2026-06-10',
				endDate: '2026-06-10',
				reason: 'family'
			})
		});
		const createdAbsence = (createResponse?.body as { absences: AttendanceAbsence[] }).absences[0];
		const deleteResponse = await createDevAttendanceMockResponse(state, {
			method: 'DELETE',
			pathname: `/attendance/api/absences/${createdAbsence.id}`,
			searchParams: new URLSearchParams()
		});
		const summaryResponse = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/summary',
			searchParams: new URLSearchParams('month=2026-06')
		});
		const summary = summaryResponse?.body as AttendanceSummary | undefined;

		expect(deleteResponse?.status).toBe(200);
		expect(summary?.absences.some((absence) => absence.id === createdAbsence.id)).toBe(false);
	});

	test('stores event overrides and projects them into the development summary', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const initialSummaryResponse = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/summary',
			searchParams: new URLSearchParams('month=2026-05')
		});
		const initialSummary = initialSummaryResponse?.body as AttendanceSummary | undefined;
		const event = initialSummary?.events.find(
			(candidate) =>
				candidate.email === 'kim@example.com' &&
				candidate.localDate === '2026-05-19' &&
				candidate.kind === 'clock_in' &&
				candidate.localTime === '08:30'
		);

		if (!event) {
			throw new Error('expected editable fixture event');
		}
		const overrideResponse = await createDevAttendanceMockResponse(state, {
			method: 'PATCH',
			pathname: `/attendance/api/events/${event.id}`,
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				localDate: '2026-05-19',
				localTime: '09:05',
				locationID: 'office',
				reason: 'dev edit'
			})
		});
		const secondOverrideResponse = await createDevAttendanceMockResponse(state, {
			method: 'PATCH',
			pathname: `/attendance/api/events/${event.id}`,
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				localDate: '2026-05-19',
				localTime: '09:15',
				locationID: 'outside',
				reason: 'second dev edit'
			})
		});
		const updatedSummaryResponse = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/attendance/api/summary',
			searchParams: new URLSearchParams('month=2026-05')
		});
		const updatedSummary = updatedSummaryResponse?.body as AttendanceSummary | undefined;
		const updatedEvent = updatedSummary?.events.find((candidate) => candidate.id === event.id);

		expect(overrideResponse?.status).toBe(200);
		expect(secondOverrideResponse?.status).toBe(200);
		expect(updatedEvent).toMatchObject({
			localTime: '09:15',
			locationID: 'outside',
			locationName: '외부',
			originalLocalTime: '09:05',
			originalLocationID: 'office',
			overrideReason: 'second dev edit',
			overriddenBy: 'kim@example.com'
		});
		expect(updatedEvent?.overrideHistory?.map((override) => override.reason)).toEqual([
			'second dev edit',
			'dev edit'
		]);
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
