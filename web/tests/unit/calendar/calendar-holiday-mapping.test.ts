import { expect, test } from 'bun:test';

import { dayTaskEventFromCalendarHoliday } from '../../../src/routes/calendar/embed/calendar-event-mapping';

test('maps a holiday to the read-only holiday calendar', () => {
	const event = dayTaskEventFromCalendarHoliday({
		id: 'holiday:holiday_api:KR:2026:2026-08-15:광복절',
		title: '광복절',
		date: '2026-08-15',
		source: 'holiday_api',
		countryCode: 'KR',
		readOnly: true,
		color: '#dc2626'
	});

	expect(event.calendarId).toBe('holidays');
	expect(event.allDay).toBe(true);
	expect(event.meta?.readOnly).toBe(true);
	expect(event.meta?.source).toBe('holiday_api');
	expect(event.meta?.countryCode).toBe('KR');
});
