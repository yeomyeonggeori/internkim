import { describe, expect, test } from 'bun:test';
import {
	shiftsNobodyClosed,
	tellingAbout,
	type AttendanceEventRow
} from '../../../src/lib/server/unclosed-shifts';

const now = new Date('2026-09-03T00:00:00.000Z');

function event(
	id: string,
	memberID: string,
	kind: 'clock_in' | 'clock_out',
	occurredAt: string,
	askedAt: string | null = null
): AttendanceEventRow {
	return { id, member_id: memberID, kind, occurred_at: occurredAt, asked_at: askedAt };
}

describe('shiftsNobodyClosed', () => {
	test('asks about a clock in nothing followed for over a day', () => {
		const rows = [event('one', 'member-one', 'clock_in', '2026-09-01T20:00:00.000Z')];

		expect(shiftsNobodyClosed(rows, now)).toEqual([
			{ eventID: 'one', memberID: 'member-one', occurredAt: '2026-09-01T20:00:00.000Z' }
		]);
	});

	test('leaves alone a shift that is still inside the day it may last', () => {
		const rows = [event('one', 'member-one', 'clock_in', '2026-09-02T20:00:00.000Z')];

		expect(shiftsNobodyClosed(rows, now)).toEqual([]);
	});

	test('leaves alone a shift somebody closed', () => {
		const rows = [
			event('one', 'member-one', 'clock_in', '2026-09-01T20:00:00.000Z'),
			event('two', 'member-one', 'clock_out', '2026-09-02T04:00:00.000Z')
		];

		expect(shiftsNobodyClosed(rows, now)).toEqual([]);
	});

	test('reads the newest event even when the rows arrive out of order', () => {
		const rows = [
			event('two', 'member-one', 'clock_out', '2026-09-02T04:00:00.000Z'),
			event('one', 'member-one', 'clock_in', '2026-09-01T20:00:00.000Z')
		];

		expect(shiftsNobodyClosed(rows, now)).toEqual([]);
	});

	test('asks once, and not again', () => {
		const rows = [
			event('one', 'member-one', 'clock_in', '2026-09-01T20:00:00.000Z', '2026-09-02T21:00:00.000Z')
		];

		expect(shiftsNobodyClosed(rows, now)).toEqual([]);
	});

	test('answers for each person who forgot', () => {
		const rows = [
			event('one', 'member-one', 'clock_in', '2026-09-01T20:00:00.000Z'),
			event('two', 'member-two', 'clock_in', '2026-09-01T18:00:00.000Z'),
			event('three', 'member-three', 'clock_in', '2026-09-02T23:00:00.000Z')
		];

		expect(shiftsNobodyClosed(rows, now).map((shift) => shift.memberID)).toEqual([
			'member-two',
			'member-one'
		]);
	});
});

describe('tellingAbout', () => {
	test('names the hour the company was on when the shift began', () => {
		const telling = tellingAbout(
			{ eventID: 'one', memberID: 'member-one', occurredAt: '2026-09-01T20:00:00.000Z' },
			'Asia/Seoul'
		);

		expect(telling.category).toBe('attendance');
		expect(telling.body).toContain('2026-09-02 05:00');
	});
});
