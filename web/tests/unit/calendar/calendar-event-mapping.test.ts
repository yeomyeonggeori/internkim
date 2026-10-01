import { createCalendarModelEvent as createEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import { expect, test } from 'bun:test';

import {
	calendarEventPayloadFromDayTaskEvent,
	dayTaskEventFromCalendarEvent,
	eventEndDate,
	eventStartDate
} from '../../../src/routes/calendar/embed/calendar-event-mapping';

function calendarDatePartsOf(date: Date): { year: number; month: number; day: number } {
	return { year: date.getFullYear(), month: date.getMonth() + 1, day: date.getDate() };
}

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
	const event = dayTaskEventFromCalendarEvent({
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
			{ personID: 'person-sample', name: '이샘플', email: 'sample@example.com', image: '/calendar/api/participants/person-sample/image' },
			{ personID: 'person-pyobon', name: '김예시', email: 'pyobon@example.com' }
		],
		createdByEmail: 'admin@example.com',
		createdByName: 'Admin',
		updatedAt: '2026-06-18T04:00:00Z'
	});

	expect(event.meta?.participants).toEqual([
		{ personID: 'person-sample', name: '이샘플', email: 'sample@example.com', image: '/calendar/api/participants/person-sample/image' },
		{ personID: 'person-pyobon', name: '김예시', email: 'pyobon@example.com' }
	]);

	const payload = calendarEventPayloadFromDayTaskEvent(event, '#2563eb');

	expect(payload.participants).toEqual([
		{ personID: 'person-sample', name: '이샘플', email: 'sample@example.com' },
		{ personID: 'person-pyobon', name: '김예시', email: 'pyobon@example.com' }
	]);
});

test('maps leave source and read-only state into event meta', () => {
	const event = dayTaskEventFromCalendarEvent({
		id: 'leave:leave-1',
		uid: 'leave:leave-1',
		title: '이샘플 · 휴가',
		description: '',
		location: '',
		startISO: '2026-08-03T00:00:00.000Z',
		endISO: '2026-08-04T00:00:00.000Z',
		timeZone: 'Asia/Seoul',
		isAllDay: true,
		color: '',
		createdByEmail: '',
		createdByName: '',
		updatedAt: '2026-08-02T15:00:00.000Z',
		readOnly: true,
		source: 'leave'
	});

	expect(event.start).toEqual(new Date(2026, 7, 3));
	expect(event.end).toEqual(new Date(2026, 7, 3));
	expect(event.meta?.readOnly).toBe(true);
	expect(event.meta?.source).toBe('leave');
});

test('draws a two-day leave across both of its days', () => {
	const event = dayTaskEventFromCalendarEvent({
		id: 'leave:leave-two-days',
		uid: 'leave:leave-two-days',
		title: '이샘플 · 휴가',
		description: '',
		location: '',
		startISO: '2026-08-02T15:00:00.000Z',
		endISO: '2026-08-04T15:00:00.000Z',
		timeZone: 'Asia/Seoul',
		isAllDay: true,
		color: '',
		createdByEmail: '',
		createdByName: '',
		updatedAt: '2026-08-02T15:00:00.000Z',
		readOnly: true,
		source: 'leave'
	});

	expect(calendarDatePartsOf(event.start)).toEqual({ year: 2026, month: 8, day: 3 });
	expect(calendarDatePartsOf(event.end)).toEqual({ year: 2026, month: 8, day: 4 });
});

test('writes a whole-day event as the calendar day the viewer picked, not an instant of their own', () => {
	const originalTimeZone = process.env.TZ;
	try {
		process.env.TZ = 'America/Los_Angeles';
		const wholeDay = createEvent({
			id: 'whole-day-draft',
			title: '워크숍',
			start: new Date(2026, 8, 2),
			end: new Date(2026, 8, 4),
			allDay: true
		});

		const payload = calendarEventPayloadFromDayTaskEvent(wholeDay, '#2563eb');

		expect(payload.startsAt).toBe('2026-09-02');
		expect(payload.endsAt).toBe('2026-09-04');
	} finally {
		process.env.TZ = originalTimeZone;
	}
});

test('places a whole-day event on the company day for a Los Angeles and a Seoul viewer alike', () => {
	const originalTimeZone = process.env.TZ;
	const wholeDayEvent = {
		id: 'whole-day-event',
		uid: 'whole-day-event',
		title: '워크숍',
		description: '',
		location: '',
		startISO: '2026-09-01T15:00:00.000Z',
		endISO: '2026-09-02T15:00:00.000Z',
		timeZone: 'Asia/Seoul',
		isAllDay: true,
		color: '',
		createdByEmail: '',
		createdByName: '',
		updatedAt: '2026-09-01T15:00:00.000Z'
	};

	try {
		process.env.TZ = 'America/Los_Angeles';
		const fromLosAngeles = dayTaskEventFromCalendarEvent(wholeDayEvent);
		expect(calendarDatePartsOf(fromLosAngeles.start)).toEqual({ year: 2026, month: 9, day: 2 });
		expect(calendarDatePartsOf(fromLosAngeles.end)).toEqual({ year: 2026, month: 9, day: 2 });

		process.env.TZ = 'Asia/Seoul';
		const fromSeoul = dayTaskEventFromCalendarEvent(wholeDayEvent);
		expect(calendarDatePartsOf(fromSeoul.start)).toEqual({ year: 2026, month: 9, day: 2 });
		expect(calendarDatePartsOf(fromSeoul.end)).toEqual({ year: 2026, month: 9, day: 2 });
	} finally {
		process.env.TZ = originalTimeZone;
	}
});
