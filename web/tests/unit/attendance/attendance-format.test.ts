import { describe, expect, test } from 'bun:test';
import { formatHoursMinutes, formatLocalTime } from '../../../src/routes/attendance/shared/attendance-format';

describe('formatHoursMinutes', () => {
	test('formats English duration units by default', () => {
		expect(formatHoursMinutes(0)).toBe('');
		expect(formatHoursMinutes(30)).toBe('00h 30m');
		expect(formatHoursMinutes(360)).toBe('06h 00m');
		expect(formatHoursMinutes(390)).toBe('06h 30m');
	});

	test('formats Korean duration units with attendance text units', () => {
		const units = { hourUnit: '시간', minuteUnit: '분' };

		expect(formatHoursMinutes(0, units)).toBe('');
		expect(formatHoursMinutes(30, units)).toBe('00시간 30분');
		expect(formatHoursMinutes(360, units)).toBe('06시간 00분');
		expect(formatHoursMinutes(390, units)).toBe('06시간 30분');
	});
});

describe('formatLocalTime', () => {
	test('keeps only hours and minutes for attendance display', () => {
		expect(formatLocalTime('00:01:40')).toBe('00:01');
		expect(formatLocalTime('02:41')).toBe('02:41');
		expect(formatLocalTime('24:00:00')).toBe('24:00');
	});

	test('returns non-matching input unchanged', () => {
		expect(formatLocalTime('not-a-time')).toBe('not-a-time');
	});
});
