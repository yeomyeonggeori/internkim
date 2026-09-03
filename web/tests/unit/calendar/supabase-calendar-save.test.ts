import { describe, expect, test } from 'bun:test';
import { calendarEventWritten } from '../../../src/lib/calendar/supabase-calendar';
import { createCalendarModelEvent } from '../../../src/routes/calendar/embed/calendar-event-model';
import { calendarEventPayloadFromDayTaskEvent } from '../../../src/routes/calendar/embed/calendar-event-mapping';
import { eventEndInstant, sizeOfEvent } from '../../../src/lib/server/public-api/record/event-tools';
import { instantWritten } from '../../../src/lib/server/public-api/record/days';
import type { CalendarEventPayload } from '../../../src/routes/calendar/embed/calendar-event-persistence';

function eventWith(fields: Partial<CalendarEventPayload> = {}): CalendarEventPayload {
	return {
		eventID: 'event-1',
		title: '팀 회의',
		description: '주간 진행 공유',
		location: '회의실 A',
		startsAt: '2026-08-20T10:00:00.000Z',
		endsAt: '2026-08-20T11:00:00.000Z',
		isAllDay: false,
		color: '',
		reminderMinutesBefore: null,
		participants: [
			{ personID: 'member-1', name: '첫 번째' },
			{ personID: 'member-2', name: '두 번째' }
		],
		...fields
	};
}

describe('what the calendar sends the record', () => {
	test('names every attendee by the exact person ID the screen already holds', () => {
		expect(calendarEventWritten(eventWith())).toEqual({
			title: '팀 회의',
			note: '주간 진행 공유',
			location: '회의실 A',
			startsAt: '2026-08-20T10:00:00.000Z',
			endsAt: '2026-08-20T11:00:00.000Z',
			isWholeDay: false,
			notifyMinutesBefore: 0,
			participantPersonHints: ['member-1', 'member-2']
		});
	});

	test('sends the whole attendee set, so clearing one removes it', () => {
		expect(calendarEventWritten(eventWith({ participants: [] })).participantPersonHints).toEqual([]);
	});

	test('names a reminder in minutes, and sends zero when the event has none', () => {
		expect(calendarEventWritten(eventWith({ reminderMinutesBefore: 30 })).notifyMinutesBefore).toBe(30);
		expect(calendarEventWritten(eventWith({ reminderMinutesBefore: null })).notifyMinutesBefore).toBe(0);
	});

	test('names a whole day by the day the viewer picked, wherever the viewer is', () => {
		const originalTimeZone = process.env.TZ;
		try {
			process.env.TZ = 'America/Los_Angeles';
			const wholeDay = createCalendarModelEvent({
				id: 'whole-day-draft',
				title: '워크숍',
				start: new Date(2026, 8, 2),
				end: new Date(2026, 8, 2),
				allDay: true
			});
			const written = calendarEventWritten(calendarEventPayloadFromDayTaskEvent(wholeDay, ''));
			expect(written).toMatchObject({ isWholeDay: true, startsAt: '2026-09-02', endsAt: '2026-09-02' });
		} finally {
			process.env.TZ = originalTimeZone;
		}
	});

	test('carries an empty note and location rather than dropping them', () => {
		const written = calendarEventWritten(eventWith({ description: '', location: '' }));
		expect(written.note).toBe('');
		expect(written.location).toBe('');
	});
});

describe('what the record makes of a whole day it is sent', () => {
	test('covers the company day the viewer picked, from its midnight to the next', () => {
		expect(instantWritten('Asia/Seoul', '2026-09-02')).toBe('2026-09-01T15:00:00.000Z');
		expect(eventEndInstant('Asia/Seoul', '2026-09-02', true)).toBe('2026-09-02T15:00:00.000Z');
	});

	test('leaves a timed end where the caller wrote it', () => {
		expect(eventEndInstant('Asia/Seoul', '2026-09-02T11:00:00+09:00', false)).toBe('2026-09-02T02:00:00.000Z');
	});
});

describe('the size an event takes', () => {
	test('follows its length in hours', () => {
		expect(sizeOfEvent('2026-08-20T10:00:00Z', '2026-08-20T11:00:00Z', false)).toBe('XS');
		expect(sizeOfEvent('2026-08-20T10:00:00Z', '2026-08-20T14:00:00Z', false)).toBe('M');
	});

	test('counts whole days when the event takes them', () => {
		expect(sizeOfEvent('2026-08-20T00:00:00Z', '2026-08-21T00:00:00Z', true)).toBe('M');
		expect(sizeOfEvent('2026-08-20T00:00:00Z', '2026-08-23T00:00:00Z', true)).toBe('XL');
	});
});
