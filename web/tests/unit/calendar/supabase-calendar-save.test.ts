import { describe, expect, test } from 'bun:test';
import { calendarEventWritten } from '../../../src/lib/calendar/supabase-calendar';
import { sizeOfEvent } from '../../../src/lib/server/public-api/record/event-tools';
import type { CalendarEventPayload } from '../../../src/routes/calendar/embed/calendar-event-persistence';

function eventWith(fields: Partial<CalendarEventPayload> = {}): CalendarEventPayload {
	return {
		eventID: 'event-1',
		title: '팀 회의',
		description: '주간 진행 공유',
		location: '회의실 A',
		startISO: '2026-08-20T10:00:00.000Z',
		endISO: '2026-08-20T11:00:00.000Z',
		timeZone: 'Asia/Seoul',
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

	test('carries an empty note and location rather than dropping them', () => {
		const written = calendarEventWritten(eventWith({ description: '', location: '' }));
		expect(written.note).toBe('');
		expect(written.location).toBe('');
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
