import { describe, expect, test } from 'bun:test';
import {
	isDue,
	remindsAt,
	spellOutMinutes,
	startsIn,
	statusesStillAhead,
	whoStillListens,
	type DueEvent
} from '../../../../supabase/functions/_shared/event-reminder-rules.ts';
import { centralTaskStatuses } from '../../../../supabase/functions/_shared/central-task-status.ts';

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

	test('the lead a person asked for is the lead they are told, all through its minute', () => {
		expect(startsIn(event(), new Date('2026-09-02T04:30:00.000Z'))).toBe(30);
		expect(startsIn(event(), new Date('2026-09-02T04:30:59.999Z'))).toBe(30);
		expect(startsIn(event({ notifyMinutesBefore: 1440 }), new Date('2026-09-01T05:00:00.000Z'))).toBe(
			1440
		);
	});
});

describe('the lead is said back in the units it was set in', () => {
	test('under an hour is minutes', () => {
		expect(spellOutMinutes(1)).toBe('1분');
		expect(spellOutMinutes(30)).toBe('30분');
		expect(spellOutMinutes(59)).toBe('59분');
	});

	test('a whole number of hours or days drops the remainder', () => {
		expect(spellOutMinutes(60)).toBe('1시간');
		expect(spellOutMinutes(120)).toBe('2시간');
		expect(spellOutMinutes(1440)).toBe('1일');
		expect(spellOutMinutes(2880)).toBe('2일');
	});

	test('a lead that does not divide keeps what is left rather than rounding it away', () => {
		expect(spellOutMinutes(90)).toBe('1시간 30분');
		expect(spellOutMinutes(1439)).toBe('23시간 59분');
		expect(spellOutMinutes(1500)).toBe('1일 1시간');
	});

	test('minutes left over inside a day are not spelled out past the hour', () => {
		expect(spellOutMinutes(1501)).toBe('1일 1시간');
	});
});

describe('a reminder reaches the people still in the company', () => {
	test('an active participant listens', () => {
		expect(whoStillListens([{ member: { id: 'one', status: 'active' } }])).toEqual(['one']);
	});

	test('somebody who left or never joined does not', () => {
		expect(
			whoStillListens([
				{ member: { id: 'one', status: 'active' } },
				{ member: { id: 'two', status: 'removed' } },
				{ member: { id: 'three', status: 'invited' } }
			])
		).toEqual(['one']);
	});

	test('a participant row whose member is gone is skipped rather than thrown on', () => {
		expect(whoStillListens([{ member: null }, { member: { id: 'one', status: 'active' } }])).toEqual([
			'one'
		]);
	});

	test('the same person listed twice is told once', () => {
		expect(
			whoStillListens([
				{ member: { id: 'one', status: 'active' } },
				{ member: { id: 'one', status: 'active' } }
			])
		).toEqual(['one']);
	});

	test('no participants means nobody to tell', () => {
		expect(whoStillListens([])).toEqual([]);
	});
});

describe('only an event still ahead of itself announces', () => {
	test('every status it announces for is one the plane actually has', () => {
		for (const status of statusesStillAhead) {
			expect(centralTaskStatuses).toContain(status);
		}
	});

	test('the settled ones and the unanswered invitation are left out', () => {
		expect([...centralTaskStatuses].filter((status) => !statusesStillAhead.includes(status)).sort())
			.toEqual(['completed', 'paused', 'rejected', 'requested', 'stopped']);
	});
});
