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
const member = { isAdmin: false, currentUserEmail: 'member@example.com', correctionWindowMinutes: threeDays };

describe('attendance write outcome', () => {
	test('saves a member record raised inside the correction window', () => {
		expect(
			attendanceWriteOutcome(
				member,
				{ subjectEmail: 'member@example.com', anchorTime: new Date('2026-07-14T09:00:00+09:00') },
				currentTime
			)
		).toBe('saved');
	});

	test('asks for approval once the record is older than the window', () => {
		expect(
			attendanceWriteOutcome(
				member,
				{ subjectEmail: 'member@example.com', anchorTime: new Date('2026-07-01T09:00:00+09:00') },
				currentTime
			)
		).toBe('requested');
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
		expect(combineAttendanceWriteOutcomes(['saved', 'requested', 'blocked'])).toBe('blocked');
	});

	test('one out-of-window record turns the whole write into a request', () => {
		expect(combineAttendanceWriteOutcomes(['saved', 'requested'])).toBe('requested');
	});

	test('an empty write is blocked', () => {
		expect(combineAttendanceWriteOutcomes([])).toBe('blocked');
	});
});

describe('attendance write result', () => {
	test('reads every status the record applies straight away as saved', () => {
		for (const status of ['added', 'corrected', 'removed']) {
			expect(attendanceWriteResultFrom({ status })).toEqual({ outcome: 'saved' });
		}
	});

	test('carries the approval id when the record raised a request', () => {
		expect(attendanceWriteResultFrom({ status: 'approval_requested', approvalID: 'approval-1' })).toEqual({
			outcome: 'requested',
			approvalID: 'approval-1'
		});
	});

	test('refuses a request that names no approval', () => {
		expect(() => attendanceWriteResultFrom({ status: 'approval_requested' })).toThrow();
	});

	test('refuses an answer with no status at all', () => {
		expect(() => attendanceWriteResultFrom(null)).toThrow();
		expect(() => attendanceWriteResultFrom({ status: 'invented' })).toThrow();
	});
});

describe('attendance write wording', () => {
	const text = attendanceText.ko.records;

	test('tells the person whether the write saves or asks', () => {
		expect(attendanceWriteIntent('saved', text)).toBe(text.savesImmediately);
		expect(attendanceWriteIntent('requested', text)).toBe(text.requestsApproval);
		expect(attendanceWriteIntent('blocked', text)).toBe(text.blocked);
	});

	test('names the submit button after what pressing it does', () => {
		expect(attendanceWriteSubmitLabel('saved', text, text.addSubmit)).toBe(text.addSubmit);
		expect(attendanceWriteSubmitLabel('requested', text, text.addSubmit)).toBe(text.requestSubmit);
	});

	test('reports what happened once the write came back', () => {
		expect(attendanceWriteConfirmation('saved', text)).toBe(text.saved);
		expect(attendanceWriteConfirmation('requested', text)).toBe(text.requested);
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
