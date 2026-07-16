import { describe, expect, test } from 'bun:test';
import {
	createDevCalendarMockResponse,
	createDevCalendarMockState
} from '../../../dev-calendar-mock-plugin';
import {
	devPopupOverflowCalendarEvents,
	devPopupOverflowDate,
	devPopupOverflowMonth
} from '../../../dev-popup-overflow-fixture';
import { todayDateInTimeZone } from '../../../src/routes/attendance/shared/attendance-date';

describe('dev calendar mock plugin', () => {
	test('provides an authenticated session for the configured development user', async () => {
		const state = createDevCalendarMockState('calendar-review@example.com');
		const response = await createDevCalendarMockResponse(state, {
			method: 'GET',
			pathname: '/auth/session',
			searchParams: new URLSearchParams()
		});

		expect(response).toEqual({
			status: 200,
			body: {
				authenticated: true,
				email: 'calendar-review@example.com',
				isAdmin: true
			}
		});
	});

	test('persists locale changes in the development calendar session', async () => {
		const state = createDevCalendarMockState('calendar-review@example.com');
		await createDevCalendarMockResponse(state, {
			method: 'PUT',
			pathname: '/admin/api/locale',
			searchParams: new URLSearchParams(),
			body: JSON.stringify({ locale: 'en' })
		});

		const response = await createDevCalendarMockResponse(state, {
			method: 'GET',
			pathname: '/admin/api/locale',
			searchParams: new URLSearchParams()
		});

		expect(response).toEqual({ status: 200, body: { locale: 'en' } });
	});

	test('provides the calendar shell background responses', async () => {
		const state = createDevCalendarMockState('calendar-review@example.com');
		const requests = [
			{ method: 'GET', pathname: '/admin/api/session' },
			{ method: 'GET', pathname: '/calendar/api/sync' },
			{ method: 'GET', pathname: '/calendar/api/account-status' },
			{ method: 'GET', pathname: '/calendar/api/participants' },
			{ method: 'POST', pathname: '/calendar/api/remote-sync' },
			{ method: 'GET', pathname: '/calendar/api/conflicts' }
		];

		for (const request of requests) {
			const response = await createDevCalendarMockResponse(state, {
				...request,
				searchParams: new URLSearchParams()
			});
			expect(response?.status).toBe(200);
		}
	});

	test('returns four June popup overflow events for 김철수', async () => {
		const response = await createDevCalendarMockResponse(createDevCalendarMockState('kim@example.com'), {
			method: 'GET',
			pathname: '/calendar/api/events',
			searchParams: new URLSearchParams({
				startISO: `${devPopupOverflowMonth}-01T00:00:00.000Z`,
				endISO: `${devPopupOverflowMonth}-30T23:59:59.999Z`
			})
		});

		const events = (response?.body.events ?? [])
			.filter((event) => event.startISO.startsWith(devPopupOverflowDate))
			.map((event) => event.title);

		expect(response?.status).toBe(200);
		expect(events).toEqual(devPopupOverflowCalendarEvents.map((event) => event.title));
	});

	test('uses the configured development user as the calendar participant', async () => {
		const response = await createDevCalendarMockResponse(createDevCalendarMockState('lee@example.com'), {
			method: 'GET',
			pathname: '/calendar/api/events',
			searchParams: new URLSearchParams({
				startISO: `${devPopupOverflowDate}T00:00:00.000Z`,
				endISO: `${devPopupOverflowDate}T23:59:59.999Z`
			})
		});

		expect(response?.body.events?.[0]?.participants?.[0]).toEqual({
			personID: 'lee',
			name: '이영희',
			email: 'lee@example.com'
		});
	});

	test('provides attendance detail preview events for today', async () => {
		const todayDate = todayDateInTimeZone('Asia/Seoul', new Date());
		const response = await createDevCalendarMockResponse(createDevCalendarMockState('kim@example.com'), {
			method: 'GET',
			pathname: '/calendar/api/events',
			searchParams: new URLSearchParams({
				startISO: `${todayDate}T00:00:00+09:00`,
				endISO: `${todayDate}T23:59:59+09:00`
			})
		});

		expect(response?.body.events?.map((event) => event.title)).toEqual([
			'오늘의 우선순위 정렬',
			'근태 상세 화면 UI 리뷰',
			'팀 진행 상황 공유'
		]);
	});
});
