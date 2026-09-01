import { describe, expect, test } from 'bun:test';
import {
	attendanceWriteOutcome,
	attendanceWriteResultFrom,
	combineAttendanceWriteOutcomes
} from '../../../src/lib/attendance/attendance-write';
import { attendanceInstantOfLocalTime } from '../../../src/routes/attendance/shared/attendance-date';
import {
	attendanceWriteConfirmation,
	attendanceWriteIntent,
	attendanceWriteSubmitLabel
} from '../../../src/routes/attendance/team/attendance-write-notice';
import { attendanceText } from '../../../src/routes/attendance/text';

const currentTime = new Date('2026-07-15T10:30:00+09:00');
const threeDays = 4320;
const member = { isAdmin: false, currentUserEmail: 'member@example.com', backdatedAfterMinutes: threeDays };

describe('attendance write outcome', () => {
	test('saves a member record raised inside the three days without telling anybody', () => {
		expect(
			attendanceWriteOutcome(
				member,
				{ subjectEmail: 'member@example.com', anchorTime: new Date('2026-07-14T09:00:00+09:00') },
				currentTime
			)
		).toBe('saved');
	});

	test('tells the administrators once the record is older than three days', () => {
		expect(
			attendanceWriteOutcome(
				member,
				{ subjectEmail: 'member@example.com', anchorTime: new Date('2026-07-01T09:00:00+09:00') },
				currentTime
			)
		).toBe('asked');
	});

	test('blocks a member from writing somebody else record', () => {
		expect(
			attendanceWriteOutcome(
				member,
				{ subjectEmail: 'colleague@example.com', anchorTime: new Date('2026-07-15T09:00:00+09:00') },
				currentTime
			)
		).toBe('blocked');
	});

	test('lets an administrator write any record at any age', () => {
		expect(
			attendanceWriteOutcome(
				{ ...member, isAdmin: true },
				{ subjectEmail: 'colleague@example.com', anchorTime: new Date('2025-01-01T09:00:00+09:00') },
				currentTime
			)
		).toBe('saved');
	});

	test('keeps a device deployment without a central window writing straight through', () => {
		expect(
			attendanceWriteOutcome(
				{ isAdmin: false, currentUserEmail: 'member@example.com' },
				{ subjectEmail: 'colleague@example.com', anchorTime: new Date('2025-01-01T09:00:00+09:00') },
				currentTime
			)
		).toBe('saved');
	});

	test('blocks a write whose anchor time cannot be read', () => {
		expect(
			attendanceWriteOutcome(
				member,
				{ subjectEmail: 'member@example.com', anchorTime: new Date(Number.NaN) },
				currentTime
			)
		).toBe('blocked');
	});
});

describe('combined attendance write outcome', () => {
	test('one blocked record blocks the whole write', () => {
		expect(combineAttendanceWriteOutcomes(['saved', 'asked', 'blocked'])).toBe('blocked');
	});

	test('one backdated record makes the whole write one the administrators hear about', () => {
		expect(combineAttendanceWriteOutcomes(['saved', 'asked'])).toBe('asked');
	});

	test('an empty write is blocked', () => {
		expect(combineAttendanceWriteOutcomes([])).toBe('blocked');
	});
});

describe('attendance write result', () => {
	test('reads every status the record writes as saved', () => {
		for (const status of ['added', 'corrected', 'removed']) {
			expect(attendanceWriteResultFrom({ status })).toEqual({ outcome: 'saved' });
		}
	});

	test('reads a write the record handed to an administrator as an asking', () => {
		expect(attendanceWriteResultFrom({ status: 'asked', eventID: null, backdated: true })).toEqual({
			outcome: 'asked'
		});
	});

	test('refuses an answer with no status at all', () => {
		expect(() => attendanceWriteResultFrom(null)).toThrow();
		expect(() => attendanceWriteResultFrom({ status: 'invented' })).toThrow();
	});
});

describe('attendance write wording', () => {
	const text = attendanceText.ko.records;

	test('tells the person whether the write goes quietly or reaches the administrators', () => {
		expect(attendanceWriteIntent('saved', text)).toBe(text.savesImmediately);
		expect(attendanceWriteIntent('asked', text)).toBe(text.asksAnAdministrator);
		expect(attendanceWriteIntent('blocked', text)).toBe(text.blocked);
	});

	test('names the submit button after what pressing it does', () => {
		expect(attendanceWriteSubmitLabel('saved', text, text.addSubmit)).toBe(text.addSubmit);
		expect(attendanceWriteSubmitLabel('asked', text, text.addSubmit)).toBe(text.askSubmit);
	});

	test('reports what happened once the write came back', () => {
		expect(attendanceWriteConfirmation('saved', text)).toBe(text.saved);
		expect(attendanceWriteConfirmation('asked', text)).toBe(text.askedAnAdministrator);
	});
});

describe('attendance instant of a local time', () => {
	test('reads a local wall clock time through the company time zone', () => {
		expect(attendanceInstantOfLocalTime('2026-07-15', '09:00', 'Asia/Seoul').toISOString()).toBe(
			'2026-07-15T00:00:00.000Z'
		);
	});

	test('follows a daylight saving shift', () => {
		expect(attendanceInstantOfLocalTime('2026-01-15', '09:00', 'America/New_York').toISOString()).toBe(
			'2026-01-15T14:00:00.000Z'
		);
		expect(attendanceInstantOfLocalTime('2026-07-15', '09:00', 'America/New_York').toISOString()).toBe(
			'2026-07-15T13:00:00.000Z'
		);
	});

	test('answers an unreadable date with an unreadable instant', () => {
		expect(Number.isNaN(attendanceInstantOfLocalTime('', '', 'Asia/Seoul').getTime())).toBe(true);
	});
});
