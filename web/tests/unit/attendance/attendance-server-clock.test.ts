import { describe, expect, test } from 'bun:test';
import {
	type AttendanceServerClock,
	attendanceServerTime,
	createAttendanceServerClock
} from '../../../src/routes/attendance/shared/attendance-server-clock';
import {
	timeInTimeZone,
	todayDateInTimeZone
} from '../../../src/routes/attendance/shared/attendance-date';

function requireServerClock(serverTime: string, monotonicTimestampMilliseconds = 1_000): AttendanceServerClock {
	const serverClock = createAttendanceServerClock(serverTime, monotonicTimestampMilliseconds);
	if (!serverClock) throw new Error(`Expected a valid attendance server clock for ${serverTime}`);
	return serverClock;
}

describe('attendance server clock', () => {
	test('calculates elapsed server time without reading the browser wall clock', () => {
		const serverClock = requireServerClock('2026-07-15T15:00:00+09:00');

		expect(attendanceServerTime(serverClock, 61_000).toISOString()).toBe('2026-07-15T06:01:00.000Z');
	});

	test('advances from the synchronized server time using monotonic elapsed time', () => {
		const serverClock = requireServerClock('2026-07-15T15:00:30+09:00');

		expect(attendanceServerTime(serverClock, 1_000).toISOString()).toBe('2026-07-15T06:00:30.000Z');
		expect(attendanceServerTime(serverClock, 31_000).toISOString()).toBe('2026-07-15T06:01:00.000Z');
	});

	test('advances across the workspace date boundary using server time', () => {
		const serverClock = requireServerClock('2026-07-31T23:59:30+09:00');
		const nextServerMinute = attendanceServerTime(serverClock, 31_000);

		expect(todayDateInTimeZone('Asia/Seoul', nextServerMinute)).toBe('2026-08-01');
		expect(timeInTimeZone('Asia/Seoul', nextServerMinute)).toBe('00:00');
	});

	test('distinguishes repeated daylight saving times by their RFC3339 offsets', () => {
		const firstServerClock = requireServerClock('2026-11-01T01:30:00-04:00');
		const secondServerClock = requireServerClock('2026-11-01T01:30:00-05:00');

		expect(attendanceServerTime(firstServerClock, 1_000).toISOString()).toBe('2026-11-01T05:30:00.000Z');
		expect(attendanceServerTime(secondServerClock, 1_000).toISOString()).toBe('2026-11-01T06:30:00.000Z');
	});

	test('accepts Go RFC3339Nano and JavaScript ISO server times', () => {
		expect(createAttendanceServerClock('2026-07-15T06:00:00.123456789Z', 1_000)).not.toBe(null);
		expect(createAttendanceServerClock('2026-07-15T06:00:00.123Z', 1_000)).not.toBe(null);
	});

	test('rejects an invalid server time contract', () => {
		expect(createAttendanceServerClock('invalid', 1_000)).toBe(null);
	});

	test('rejects a numeric server time contract', () => {
		expect(createAttendanceServerClock(2026, 1_000)).toBe(null);
	});

	test('rejects an array server time contract', () => {
		expect(createAttendanceServerClock(['2026-07-15T06:00:00Z'], 1_000)).toBe(null);
	});

	test('rejects a missing server time contract', () => {
		expect(createAttendanceServerClock(undefined, 1_000)).toBe(null);
	});

	test('rejects a date-like server time contract without RFC3339 time fields', () => {
		expect(createAttendanceServerClock('2026-07-15', 1_000)).toBe(null);
	});

	test('rejects an RFC3339-shaped date outside the calendar', () => {
		expect(createAttendanceServerClock('2026-02-30T06:00:00Z', 1_000)).toBe(null);
	});

	test('rejects an RFC3339-shaped time outside the day', () => {
		expect(createAttendanceServerClock('2026-07-15T24:00:00Z', 1_000)).toBe(null);
	});
});
