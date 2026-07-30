// 캘린더 이벤트 매핑의 날짜 변환 회귀를 검증합니다.
import { createCalendarModelEvent as createEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import { expect, test } from 'bun:test';

import {
	calendarEventPayloadFromDayFlowEvent,
	dayFlowEventFromCalendarEvent,
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

test('maps calendar participants through event meta and payload', () => {
	const event = dayFlowEventFromCalendarEvent({
		id: 'participants-event',
		uid: 'participants-event@internkim',
		title: 'Participants event',
		description: 'Bring agenda',
		location: 'Studio',
		startISO: '2026-06-18T03:00:00Z',
		endISO: '2026-06-18T04:00:00Z',
		timeZone: 'Asia/Seoul',
		isAllDay: false,
		color: '#2563eb',
		participants: [
			{ personID: 'person-dongha', name: '이샘플', email: 'dongha@example.com', image: '/calendar/api/participants/person-dongha/image' },
			{ personID: 'person-yeomyeong', name: '김여명', email: 'yeomyeong@example.com' }
		],
		createdByEmail: 'admin@example.com',
		createdByName: 'Admin',
		updatedAt: '2026-06-18T04:00:00Z'
	});

	expect(event.meta?.participants).toEqual([
		{ personID: 'person-dongha', name: '이샘플', email: 'dongha@example.com', image: '/calendar/api/participants/person-dongha/image' },
		{ personID: 'person-yeomyeong', name: '김여명', email: 'yeomyeong@example.com' }
	]);

	const payload = calendarEventPayloadFromDayFlowEvent(event, '#2563eb', 'Asia/Seoul');

	expect(payload.participants).toEqual([
		{ personID: 'person-dongha', name: '이샘플', email: 'dongha@example.com' },
		{ personID: 'person-yeomyeong', name: '김여명', email: 'yeomyeong@example.com' }
	]);
});
