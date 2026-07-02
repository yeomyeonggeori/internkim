import { describe, expect, test } from 'bun:test';
import { createDevCalendarMockResponse } from '../../../dev-calendar-mock-plugin';
import {
	devPopupOverflowCalendarEvents,
	devPopupOverflowDate,
	devPopupOverflowMonth
} from '../../../dev-popup-overflow-fixture';

describe('dev calendar mock plugin', () => {
	test('returns four June popup overflow events for 김철수', async () => {
		const response = await createDevCalendarMockResponse({
			method: 'GET',
			pathname: '/calendar/api/events',
			searchParams: new URLSearchParams({
				startISO: `${devPopupOverflowMonth}-01T00:00:00.000Z`,
				endISO: `${devPopupOverflowMonth}-30T23:59:59.999Z`
			})
		});

		const events = response?.body.events
			.filter((event) => event.startISO.startsWith(devPopupOverflowDate))
			.map((event) => event.title);

		expect(response?.status).toBe(200);
		expect(events).toEqual(devPopupOverflowCalendarEvents.map((event) => event.title));
	});

	test('uses the configured development user as the calendar participant', async () => {
		const response = await createDevCalendarMockResponse({
			method: 'GET',
			pathname: '/calendar/api/events',
			searchParams: new URLSearchParams({
				startISO: `${devPopupOverflowDate}T00:00:00.000Z`,
				endISO: `${devPopupOverflowDate}T23:59:59.999Z`
			})
		}, 'lee@example.com');

		expect(response?.body.events[0]?.participants?.[0]).toEqual({
			personID: 'lee',
			name: '이영희',
			email: 'lee@example.com'
		});
	});
});
