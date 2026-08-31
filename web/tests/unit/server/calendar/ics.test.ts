import { describe, expect, test } from 'bun:test';
import {
	calendarFeedOf,
	calendarFieldsOf,
	dayOf,
	escapedText,
	foldedLine,
	locationNameOf,
	momentOf,
	type CalendarFeedEvent
} from '$lib/server/calendar/ics';

function event(overrides: Partial<CalendarFeedEvent> = {}): CalendarFeedEvent {
	return {
		id: '00000000-0000-0000-0000-000000000001',
		title: '주간 회의',
		note: null,
		location: null,
		starts_at: '2026-09-01T01:00:00.000Z',
		ends_at: '2026-09-01T02:00:00.000Z',
		is_whole_day: false,
		updated_at: '2026-08-31T00:00:00.000Z',
		calendar: null,
		...overrides
	};
}

function linesOf(feed: string): string[] {
	return feed.split('\r\n');
}

describe('a feed', () => {
	test('opens and closes as one calendar, and names the company', () => {
		const feed = calendarFeedOf([], '예시회사', 'Asia/Seoul');
		const lines = linesOf(feed);

		expect(lines[0]).toBe('BEGIN:VCALENDAR');
		expect(feed).toContain('X-WR-CALNAME:예시회사');
		expect(feed).toContain('X-WR-TIMEZONE:Asia/Seoul');
		expect(feed.endsWith('END:VCALENDAR\r\n')).toBe(true);
	});

	test('breaks its lines the way the format counts them', () => {
		const feed = calendarFeedOf([event({ title: '가'.repeat(60) })], 'c', 'UTC');

		for (const line of linesOf(feed)) {
			expect(new TextEncoder().encode(line).byteLength <= 75).toBe(true);
		}
		expect(feed).toContain('\r\n ');
	});
});

describe('an event at a time', () => {
	test('carries the moments it runs between', () => {
		const feed = calendarFeedOf([event()], 'c', 'Asia/Seoul');

		expect(feed).toContain('DTSTART:20260901T010000Z');
		expect(feed).toContain('DTEND:20260901T020000Z');
		expect(feed).toContain('UID:00000000-0000-0000-0000-000000000001');
		expect(feed).toContain('DTSTAMP:20260831T000000Z');
	});

	test('leaves out what it does not have', () => {
		const feed = calendarFeedOf([event({ note: '   ', location: null })], 'c', 'UTC');

		expect(feed).not.toContain('DESCRIPTION:');
		expect(feed).not.toContain('LOCATION:');
	});

	test('carries a note and a place when it has them', () => {
		const feed = calendarFeedOf([event({ note: '작년 것 참고', location: { name: '회의실' } })], 'c', 'UTC');

		expect(feed).toContain('DESCRIPTION:작년 것 참고');
		expect(feed).toContain('LOCATION:회의실');
	});
});

describe('a whole-day event', () => {
	test('is dated where the company is, and ends the day after the last one it holds', () => {
		const feed = calendarFeedOf(
			[
				event({
					is_whole_day: true,
					starts_at: '2026-08-31T15:00:00.000Z',
					ends_at: '2026-09-01T14:59:59.999Z'
				})
			],
			'c',
			'Asia/Seoul'
		);

		expect(feed).toContain('DTSTART;VALUE=DATE:20260901');
		expect(feed).toContain('DTEND;VALUE=DATE:20260902');
	});

	test('covering several days ends the day after the last of them', () => {
		const feed = calendarFeedOf(
			[
				event({
					is_whole_day: true,
					starts_at: '2026-08-31T15:00:00.000Z',
					ends_at: '2026-09-03T14:59:59.999Z'
				})
			],
			'c',
			'Asia/Seoul'
		);

		expect(feed).toContain('DTSTART;VALUE=DATE:20260901');
		expect(feed).toContain('DTEND;VALUE=DATE:20260904');
	});
});

describe('the text a calendar app reads back', () => {
	test('keeps a comma, a semicolon, a backslash and a newline from ending the line', () => {
		expect(escapedText('a,b;c\\d\ne')).toBe('a\\,b\\;c\\\\d\\ne');
	});

	test('is folded only when it is too long', () => {
		expect(foldedLine('short')).toBe('short');
		expect(foldedLine('x'.repeat(80))).toContain('\r\n ');
	});
});

describe('what the task row cannot hold', () => {
	test('is read from the calendar the event carries', () => {
		const feed = calendarFeedOf(
			[event({ calendar: { color: '#2563eb', timeZone: 'Asia/Seoul', mirrors: [] } })],
			'c',
			'UTC'
		);

		expect(feed).toContain('COLOR:#2563eb');
	});

	test('dates a whole day where the event was made, not where the company is', () => {
		const inSeoul = event({
			is_whole_day: true,
			starts_at: '2026-08-31T15:00:00.000Z',
			ends_at: '2026-09-01T14:59:59.999Z',
			calendar: { timeZone: 'Asia/Seoul', mirrors: [] }
		});

		expect(calendarFeedOf([inSeoul], 'c', 'UTC')).toContain('DTSTART;VALUE=DATE:20260901');
		expect(calendarFeedOf([{ ...inSeoul, calendar: null }], 'c', 'UTC')).toContain(
			'DTSTART;VALUE=DATE:20260831'
		);
	});

	test('reads nothing out of a calendar that carries nothing', () => {
		expect(calendarFieldsOf(null)).toEqual({ timeZone: undefined, color: undefined });
		expect(calendarFieldsOf({ mirrors: [] })).toEqual({ timeZone: undefined, color: undefined });
		expect(calendarFieldsOf({ color: '  ' })).toEqual({ timeZone: undefined, color: undefined });
	});
});

describe('the pieces a row carries', () => {
	test('read a place written either way', () => {
		expect(locationNameOf('회의실')).toBe('회의실');
		expect(locationNameOf({ name: '회의실' })).toBe('회의실');
		expect(locationNameOf(null)).toBe('');
		expect(locationNameOf({ other: 1 })).toBe('');
	});

	test('read a moment and a day', () => {
		expect(momentOf('2026-09-01T01:00:00.000Z')).toBe('20260901T010000Z');
		expect(dayOf('2026-08-31T15:00:00.000Z', 'Asia/Seoul')).toBe('20260901');
		expect(dayOf('2026-08-31T15:00:00.000Z', 'UTC')).toBe('20260831');
	});
});
