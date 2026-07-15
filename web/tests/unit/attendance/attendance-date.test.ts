import { describe, expect, test } from 'bun:test';
import {
	addDays,
	currentMonthInTimeZone,
	eachDayOfWeek,
	fallbackFutureAttendanceLocalTime,
	isFutureAttendanceLocalTime,
	todayDateInTimeZone
} from '../../../src/routes/attendance/shared/attendance-date';

describe('attendance date helpers', () => {
	test('uses workspace timezone date at UTC day boundary', () => {
		const utcEveningBeforeSeoulDay = new Date('2026-05-19T15:30:00.000Z');
		const today = todayDateInTimeZone('Asia/Seoul', utcEveningBeforeSeoulDay);
		const month = currentMonthInTimeZone('Asia/Seoul', utcEveningBeforeSeoulDay);

		expect(today).toBe('2026-05-20');
		expect(month).toBe('2026-05');
	});

	test('falls back to browser timezone for local timezone label', () => {
		const date = new Date('2026-05-19T15:30:00.000Z');

		expect(() => todayDateInTimeZone('Local', date)).not.toThrow();
	});

	test('moves across month and year boundaries with UTC date keys', () => {
		expect(addDays('2026-06-30', 1)).toBe('2026-07-01');
		expect(addDays('2026-12-31', 1)).toBe('2027-01-01');
		expect(addDays('2026-01-01', -1)).toBe('2025-12-31');
	});

	test('builds Monday to Sunday week dates', () => {
		expect(eachDayOfWeek('2026-06-17')).toEqual([
			'2026-06-15',
			'2026-06-16',
			'2026-06-17',
			'2026-06-18',
			'2026-06-19',
			'2026-06-20',
			'2026-06-21',
		]);
	});

	test('allows past dates and the current workspace minute', () => {
		const now = new Date('2026-07-14T06:00:30.000Z');

		expect(isFutureAttendanceLocalTime('2026-07-13', '23:59', 'Asia/Seoul', now)).toBe(false);
		expect(isFutureAttendanceLocalTime('2026-07-14', '14:59', 'Asia/Seoul', now)).toBe(false);
		expect(isFutureAttendanceLocalTime('2026-07-14', '15:00', 'Asia/Seoul', now)).toBe(false);
	});

	test('rejects a future workspace minute or date', () => {
		const now = new Date('2026-07-14T06:00:30.000Z');

		expect(isFutureAttendanceLocalTime('2026-07-14', '15:01', 'Asia/Seoul', now)).toBe(true);
		expect(isFutureAttendanceLocalTime('2026-07-15', '00:00', 'Asia/Seoul', now)).toBe(true);
	});

	test('uses the workspace date when UTC is still on the previous day', () => {
		const utcEvening = new Date('2026-07-14T15:30:00.000Z');

		expect(isFutureAttendanceLocalTime('2026-07-14', '23:59', 'Asia/Seoul', utcEvening)).toBe(false);
		expect(isFutureAttendanceLocalTime('2026-07-15', '00:31', 'Asia/Seoul', utcEvening)).toBe(true);
	});

	test('falls back only future minutes on the current workspace date', () => {
		const now = new Date('2026-07-14T06:00:30.000Z');

		expect(fallbackFutureAttendanceLocalTime('2026-07-14', '15:30', 'Asia/Seoul', now)).toBe('15:00');
		expect(fallbackFutureAttendanceLocalTime('2026-07-14', '14:30', 'Asia/Seoul', now)).toBe('14:30');
		expect(fallbackFutureAttendanceLocalTime('2026-07-13', '23:59', 'Asia/Seoul', now)).toBe('23:59');
		expect(fallbackFutureAttendanceLocalTime('2026-07-15', '00:00', 'Asia/Seoul', now)).toBe('00:00');
	});
});
