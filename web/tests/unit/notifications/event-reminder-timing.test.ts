import { describe, expect, test } from 'bun:test';
import {
	isDue,
	remindsAt,
	startsIn,
	type DueEvent
} from '../../../../supabase/functions/_shared/event-reminder-timing.ts';

function event(over: Partial<DueEvent> = {}): DueEvent {
	return {
		id: 'event-1',
		title: '주간 회의',
		startsAt: '2026-09-02T05:00:00.000Z',
		isWholeDay: false,
		notifyMinutesBefore: 30,
		...over
	};
}

describe('an event is due for its reminder in exactly one minute', () => {
	test('the lead lands on the minute the reminder belongs to', () => {
		expect(remindsAt(event())?.toISOString()).toBe('2026-09-02T04:30:00.000Z');
	});

	test('that minute is due, and the seconds inside it do not matter', () => {
		expect(isDue(event(), new Date('2026-09-02T04:30:00.000Z'))).toBe(true);
		expect(isDue(event(), new Date('2026-09-02T04:30:59.999Z'))).toBe(true);
	});

	test('the minute before and the minute after are not', () => {
		expect(isDue(event(), new Date('2026-09-02T04:29:59.999Z'))).toBe(false);
		expect(isDue(event(), new Date('2026-09-02T04:31:00.000Z'))).toBe(false);
	});

	test('an event asking for no reminder is never due', () => {
		expect(remindsAt(event({ notifyMinutesBefore: null }))).toBeNull();
		expect(isDue(event({ notifyMinutesBefore: null }), new Date('2026-09-02T04:30:00.000Z'))).toBe(
			false
		);
	});

	test('an event that has already started is not reminded about', () => {
		const started = event({ startsAt: '2026-09-02T04:00:00.000Z', notifyMinutesBefore: 30 });
		expect(isDue(started, new Date('2026-09-02T04:30:00.000Z'))).toBe(false);
	});

	test('a lead of zero or less is not a reminder', () => {
		expect(remindsAt(event({ notifyMinutesBefore: 0 }))).toBeNull();
		expect(remindsAt(event({ notifyMinutesBefore: -30 }))).toBeNull();
	});

	test('a start time nothing can parse is not due', () => {
		expect(remindsAt(event({ startsAt: 'tomorrow morning' }))).toBeNull();
		expect(isDue(event({ startsAt: 'tomorrow morning' }), new Date())).toBe(false);
	});

	test('a whole day of lead still lands on its own minute', () => {
		const tomorrow = event({ notifyMinutesBefore: 1440 });
		expect(isDue(tomorrow, new Date('2026-09-01T05:00:00.000Z'))).toBe(true);
		expect(isDue(tomorrow, new Date('2026-09-01T05:01:00.000Z'))).toBe(false);
	});

	test('how far off the start is, is read from the same pair', () => {
		expect(startsIn(event(), new Date('2026-09-02T04:30:00.000Z'))).toBe(30);
		expect(startsIn(event({ notifyMinutesBefore: 1440 }), new Date('2026-09-01T05:00:00.000Z'))).toBe(
			1440
		);
	});
});
