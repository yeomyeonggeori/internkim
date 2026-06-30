import { createEvent } from '@dayflow/core';
import { expect, test } from 'bun:test';

import {
	calendarMobileEditorStartDateTimeChanges,
	calendarMobileEditorUpdatedEvent
} from '../../../src/routes/calendar/embed/calendar-mobile-event-editor-state';

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
				personID: 'person-gamyeong',
				name: '이샘플',
				email: 'gamyeong@example.com',
				image: '/calendar/api/participants/person-gamyeong/image'
			}
		]
	});

	expect(event.title).toBe('모바일 참여자 일정');
	expect(event.meta?.location).toBe('회의실');
	expect(event.meta?.participants).toEqual([
		{
			personID: 'person-gamyeong',
			name: '이샘플',
			email: 'gamyeong@example.com',
			image: '/calendar/api/participants/person-gamyeong/image'
		}
	]);
});

test('keeps the existing duration when a mobile editor start moves after the current end', () => {
	const changes = calendarMobileEditorStartDateTimeChanges(
		{
			startDateKey: '2026-06-18',
			endDateKey: '2026-06-18',
			startTime: '10:00',
			endTime: '12:00',
			allDay: false
		},
		{ startTime: '14:00' }
	);

	expect(changes).toEqual({
		startDateKey: '2026-06-18',
		endDateKey: '2026-06-18',
		startTime: '14:00',
		endTime: '16:00',
		allDay: false
	});
});
