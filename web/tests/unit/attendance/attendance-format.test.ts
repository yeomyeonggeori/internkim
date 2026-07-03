// 근태 시간 포맷의 언어별 단위 표시를 검증한다.
import { describe, expect, test } from 'bun:test';
import { formatHoursMinutes } from '../../../src/routes/attendance/shared/attendance-format';

describe('formatHoursMinutes', () => {
	test('formats English duration units by default', () => {
		expect(formatHoursMinutes(0)).toBe('0h');
		expect(formatHoursMinutes(30)).toBe('30m');
		expect(formatHoursMinutes(360)).toBe('6h');
		expect(formatHoursMinutes(390)).toBe('6h 30m');
	});

	test('formats Korean duration units with attendance text units', () => {
		const units = { hourUnit: '시간', minuteUnit: '분' };

		expect(formatHoursMinutes(0, units)).toBe('0시간');
		expect(formatHoursMinutes(30, units)).toBe('30분');
		expect(formatHoursMinutes(360, units)).toBe('6시간');
		expect(formatHoursMinutes(390, units)).toBe('6시간 30분');
	});
});
