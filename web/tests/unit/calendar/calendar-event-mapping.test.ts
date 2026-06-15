// 캘린더 이벤트 매핑의 날짜 변환 회귀를 검증합니다.
import { createEvent } from '@dayflow/core';
import { expect, test } from 'bun:test';

import {
	eventEndDate,
	eventStartDate
} from '../../../src/routes/calendar/embed/calendar-event-mapping';

test('maps all-day event display dates to local midnight', () => {
	const event = createEvent({
		id: 'all-day-display-date',
		title: 'All day display date',
		start: new Date(2026, 5, 8),
		end: new Date(2026, 5, 10),
		allDay: true,
		calendarId: 'internkim'
	});

	expect(eventStartDate(event)).toEqual(new Date(2026, 5, 8));
	expect(eventEndDate(event)).toEqual(new Date(2026, 5, 10));
});
