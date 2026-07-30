import { describe, expect, test } from 'bun:test';
import {
	createDevAttendanceMockResponse,
	createDevAttendanceMockState
} from '../../../dev-attendance-mock-plugin';
import { buildAttendanceSummaryFixture } from '../../../dev-attendance-summary-fixture';
import {
	devPopupOverflowAttendanceRows,
	devPopupOverflowDate,
	devPopupOverflowEmail,
	devPopupOverflowMonth
} from '../../../dev-popup-overflow-fixture';
import { todayDateInTimeZone } from '../../../src/routes/attendance/shared/attendance-date';
import type { AttendanceAbsence, AttendanceSummary } from '../../../src/routes/attendance/attendance-context.svelte';

class HostPreviousDate extends Date {
	override getDate(): number {
		return 30;
	}
}

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

	test('returns and updates the default attendance leave policy', async () => {
		const state = createDevAttendanceMockState('kim@example.com');
		const initialResponse = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/admin/api/attendance-leave-policy',
			searchParams: new URLSearchParams()
		});

		expect(initialResponse?.status).toBe(200);
		expect(initialResponse?.body).toMatchObject({
			version: 2
		});

		const initialPolicy = initialResponse?.body as {
			leaveTypes: Array<Record<string, unknown>>;
			[key: string]: unknown;
		};
		expect(initialPolicy.leaveTypes.length).toBe(16);
		expect(initialPolicy.leaveTypes[9]).toMatchObject({ id: 'reward', name: '포상휴가' });
		expect(initialPolicy.leaveTypes[13]).toMatchObject({
			id: 'parental-leave',
			name: '육아휴직'
		});
		const updateResponse = await createDevAttendanceMockResponse(state, {
			method: 'PUT',
			pathname: '/admin/api/attendance-leave-policy',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({
				...initialPolicy,
				leaveTypes: [
					...initialPolicy.leaveTypes,
					{
						...initialPolicy.leaveTypes[15],
						id: '',
						systemKind: '',
						name: '회사 특별 휴가',
						isSystem: false,
						sortOrder: 16
					}
				]
			})
		});
		const reloadedResponse = await createDevAttendanceMockResponse(state, {
			method: 'GET',
			pathname: '/admin/api/attendance-leave-policy',
			searchParams: new URLSearchParams()
		});

		expect(updateResponse?.status).toBe(200);
		const updatedPolicy = updateResponse?.body as {
			leaveTypes: Array<Record<string, unknown>>;
		};
		expect(updatedPolicy.leaveTypes.length).toBe(17);
		expect(updatedPolicy.leaveTypes[16]).toMatchObject({
			id: 'custom-1',
			name: '회사 특별 휴가',
			isSystem: false
		});
		expect(reloadedResponse?.body).toEqual(updateResponse?.body);
	});

	test('keeps the Seoul fixture day when the host time zone is still on the previous date', () => {
		const currentTime = new HostPreviousDate('2026-07-01T00:30:00+09:00');
		const summary = buildAttendanceSummaryFixture('2026-07', currentTime);
		const personalLeave = summary.absences.find((absence) => absence.id === 'absence-personal-leave');

		expect(summary.events.every((event) => event.localDate <= '2026-07-01')).toBe(true);
		expect(personalLeave?.date).toBe('2026-07-02');
	});

	test('includes work, partial leave, and resumed work in the July fixture', () => {
		const summary = buildAttendanceSummaryFixture(
			'2026-07',
			new Date('2026-07-29T10:00:00+09:00')
		);
		const partialLeave = summary.absences.find(
			(absence) => absence.id === 'absence-lee-partial-leave'
		);
		const events = summary.events
			.filter(
				(event) => event.email === 'lee@example.com' && event.localDate === '2026-07-17'
			)
			.map((event) => ({ kind: event.kind, localTime: event.localTime }));

		expect(partialLeave).toMatchObject({
			kind: 'leave',
			startTime: '13:00',
			endTime: '15:00'
		});
		expect(events).toEqual([
			{ kind: 'clock_in', localTime: '09:00' },
			{ kind: 'clock_out', localTime: '12:00' },
			{ kind: 'clock_in', localTime: '15:00' },
			{ kind: 'clock_out', localTime: '18:00' }
		]);
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
