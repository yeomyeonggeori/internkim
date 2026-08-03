import { describe, expect, test } from 'bun:test';
import {
	dayInTimezone,
	forgottenClockOut,
	instantFromLocalInput,
	isPlausibleClockOut,
	localInputOf,
	nextClockKind,
	type AttendanceEntry,
} from '../../src/lib/clock';

function entry(kind: 'clock_in' | 'clock_out', occurredAt: string): AttendanceEntry {
	return { id: occurredAt, kind, location: null, occurred_at: occurredAt };
}

describe('what to record next', () => {
	test('an open clock-in is followed by a clock-out', () => {
		expect(nextClockKind([entry('clock_in', '2026-08-03T00:00:00Z')])).toBe('clock_out');
	});

	test('with nothing open, or nothing at all, the next record is a clock-in', () => {
		expect(nextClockKind([entry('clock_out', '2026-08-03T09:00:00Z')])).toBe('clock_in');
		expect(nextClockKind([])).toBe('clock_in');
	});
});

describe('a clock-out someone forgot', () => {
	const seoul = 'Asia/Seoul';

	test('an open clock-in from an earlier day has to be closed first', () => {
		// 18:00 KST on the 2nd, asked about at 09:00 KST on the 3rd.
		const open = entry('clock_in', '2026-08-02T09:00:00Z');
		const now = new Date('2026-08-03T00:00:00Z');

		expect(forgottenClockOut([open], seoul, now)).toBe(open);
	});

	test('an open clock-in from today is just an ordinary shift', () => {
		const open = entry('clock_in', '2026-08-03T00:30:00Z');
		const now = new Date('2026-08-03T02:00:00Z');

		expect(forgottenClockOut([open], seoul, now)).toBeNull();
	});

	test('a closed day asks nothing', () => {
		const closed = entry('clock_out', '2026-08-02T09:00:00Z');

		expect(forgottenClockOut([closed], seoul, new Date('2026-08-03T00:00:00Z'))).toBeNull();
	});

	test('the day is read in the member timezone, not the browser one', () => {
		// 23:30 UTC on the 2nd is already the 3rd in Seoul.
		expect(dayInTimezone('2026-08-02T23:30:00Z', seoul)).toBe('2026-08-03');
		expect(dayInTimezone('2026-08-02T23:30:00Z', 'UTC')).toBe('2026-08-02');
	});

	test('the leaving time has to sit between arriving and now', () => {
		const openedAt = '2026-08-02T09:00:00Z';
		const now = new Date('2026-08-03T00:00:00Z');

		expect(isPlausibleClockOut(openedAt, new Date('not a time'), now)).toBe(false);
		expect(isPlausibleClockOut(openedAt, new Date('2026-08-02T12:00:00Z'), now)).toBe(true);
		expect(isPlausibleClockOut(openedAt, new Date('2026-08-02T08:00:00Z'), now)).toBe(false);
		expect(isPlausibleClockOut(openedAt, new Date('2026-08-04T00:00:00Z'), now)).toBe(false);
	});
});

describe('reading a wall clock in the member timezone', () => {
	test('a Seoul wall clock becomes the right instant', () => {
		expect(instantFromLocalInput('2026-08-02T18:00', 'Asia/Seoul').toISOString()).toBe(
			'2026-08-02T09:00:00.000Z',
		);
	});

	test('the same wall clock in New York is a different instant', () => {
		expect(instantFromLocalInput('2026-08-02T18:00', 'America/New_York').toISOString()).toBe(
			'2026-08-02T22:00:00.000Z',
		);
	});

	test('an instant renders back to the wall clock it came from', () => {
		const instant = instantFromLocalInput('2026-08-02T18:00', 'Asia/Seoul');

		expect(localInputOf(instant, 'Asia/Seoul')).toBe('2026-08-02T18:00');
	});
});
