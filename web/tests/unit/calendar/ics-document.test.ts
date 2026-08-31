import { describe, expect, test } from 'bun:test';
import {
	icsDocumentOf,
	type CalendarFeed,
	type CalendarFeedEvent
} from '../../../src/lib/server/calendar-feed/ics-document';

const uidDomain = 'internkim.example';

function aFeed(events: CalendarFeedEvent[], timezone = 'Asia/Seoul'): CalendarFeed {
	return { company: '샘플회사', timezone, events };
}

function anEvent(held: Partial<CalendarFeedEvent> = {}): CalendarFeedEvent {
	return {
		id: '00000000-0000-0000-0000-0000000000c1',
		title: 'Standup',
		note: null,
		location: null,
		startsAt: '2027-03-02T01:00:00+00:00',
		endsAt: '2027-03-02T02:00:00+00:00',
		isWholeDay: false,
		updatedAt: '2027-03-01T09:30:00+00:00',
		...held
	};
}

function linesOf(document: string): string[] {
	return document.split('\r\n');
}

describe('the calendar feed document', () => {
	test('a calendar with nothing in it is still a calendar', () => {
		const document = icsDocumentOf(aFeed([]), uidDomain);
		expect(linesOf(document)[0]).toBe('BEGIN:VCALENDAR');
		expect(document.endsWith('END:VCALENDAR\r\n')).toBe(true);
		expect(document).toContain('X-WR-TIMEZONE:Asia/Seoul');
	});

	test('every line ends the way a calendar client expects', () => {
		const document = icsDocumentOf(aFeed([anEvent()]), uidDomain);
		expect(document.includes('\n')).toBe(true);
		expect(document.split('\n').every((line) => line === '' || line.endsWith('\r'))).toBe(true);
	});

	test('a timed event carries the instant it happens at, in UTC', () => {
		const lines = linesOf(icsDocumentOf(aFeed([anEvent()]), uidDomain));
		expect(lines).toContain('DTSTART:20270302T010000Z');
		expect(lines).toContain('DTEND:20270302T020000Z');
		expect(lines).toContain('DTSTAMP:20270301T093000Z');
		expect(lines).toContain('UID:00000000-0000-0000-0000-0000000000c1@internkim.example');
		expect(lines).toContain('SUMMARY:Standup');
	});

	test('a whole day is a date in the company timezone, and its end is the day after', () => {
		const wholeDay = anEvent({
			isWholeDay: true,
			startsAt: '2027-03-01T15:00:00+00:00',
			endsAt: '2027-03-02T15:00:00+00:00'
		});
		const lines = linesOf(icsDocumentOf(aFeed([wholeDay]), uidDomain));
		expect(lines).toContain('DTSTART;VALUE=DATE:20270302');
		expect(lines).toContain('DTEND;VALUE=DATE:20270303');
	});

	test('a whole day is read in the company timezone rather than in UTC', () => {
		const wholeDay = anEvent({
			isWholeDay: true,
			startsAt: '2027-03-01T15:00:00+00:00',
			endsAt: '2027-03-02T15:00:00+00:00'
		});
		const lines = linesOf(icsDocumentOf(aFeed([wholeDay], 'UTC'), uidDomain));
		expect(lines).toContain('DTSTART;VALUE=DATE:20270301');
		expect(lines).toContain('DTEND;VALUE=DATE:20270302');
	});

	test('a note and a place are carried only when there is one', () => {
		const bare = linesOf(icsDocumentOf(aFeed([anEvent()]), uidDomain));
		expect(bare.some((line) => line.startsWith('DESCRIPTION:'))).toBe(false);
		expect(bare.some((line) => line.startsWith('LOCATION:'))).toBe(false);

		const filled = linesOf(
			icsDocumentOf(aFeed([anEvent({ note: 'bring the deck', location: 'Room 3' })]), uidDomain)
		);
		expect(filled).toContain('DESCRIPTION:bring the deck');
		expect(filled).toContain('LOCATION:Room 3');
	});

	test('a comma, a semicolon and a newline are escaped rather than ending the value', () => {
		const lines = linesOf(
			icsDocumentOf(aFeed([anEvent({ title: 'Plan A, B; C\nand D' })]), uidDomain)
		);
		expect(lines).toContain('SUMMARY:Plan A\\, B\\; C\\nand D');
	});

	test('a backslash survives as a backslash', () => {
		const lines = linesOf(icsDocumentOf(aFeed([anEvent({ title: 'before\\after' })]), uidDomain));
		expect(lines).toContain('SUMMARY:before\\\\after');
	});

	test('a long line is folded, and folding never cuts a character in half', () => {
		const title = '분기 계획 회의와 그 준비를 위한 사전 점검 자리입니다 그리고 이어지는 회고까지';
		const document = icsDocumentOf(aFeed([anEvent({ title })]), uidDomain);
		const lines = linesOf(document);

		const folded = lines.filter((line) => line.startsWith(' '));
		expect(folded.length > 0).toBe(true);
		expect(lines.filter((line) => new TextEncoder().encode(line).length > 75)).toEqual([]);
		expect(unfolded(lines)).toContain(`SUMMARY:${title}`);
	});
});

function unfolded(lines: string[]): string {
	return lines
		.reduce<string[]>((joined, line) => {
			if (line.startsWith(' ') && joined.length > 0) {
				joined[joined.length - 1] += line.slice(1);
				return joined;
			}
			return [...joined, line];
		}, [])
		.join('\n');
}
