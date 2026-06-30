import { createEvent } from '@dayflow/core';
import { expect, test } from 'bun:test';

import { calendarMobileEditorUpdatedEvent } from '../../../src/routes/calendar/embed/calendar-mobile-event-editor-state';

test('keeps edited mobile participants in event metadata', () => {
	const draftEvent = createEvent({
		id: 'mobile-participants',
		title: '기존 일정',
		description: '',
		start: new Date('2026-06-01T01:00:00Z'),
		end: new Date('2026-06-01T02:00:00Z'),
		allDay: false,
		calendarId: 'internkim',
		meta: {
			location: '기존 장소'
		}
	});

	const event = calendarMobileEditorUpdatedEvent({
		draftEvent,
		title: '모바일 참여자 일정',
		description: '회의 메모',
		start: new Date('2026-06-02T01:00:00Z'),
		end: new Date('2026-06-02T02:00:00Z'),
		allDay: false,
		calendarID: 'team',
		location: '회의실',
		participants: [
			{
				personID: 'person-dongha',
				name: '이동하',
				email: 'dongha@example.com',
				image: '/calendar/api/participants/person-dongha/image'
			}
		]
	});

	expect(event.title).toBe('모바일 참여자 일정');
	expect(event.meta?.location).toBe('회의실');
	expect(event.meta?.participants).toEqual([
		{
			personID: 'person-dongha',
			name: '이동하',
			email: 'dongha@example.com',
			image: '/calendar/api/participants/person-dongha/image'
		}
	]);
});
