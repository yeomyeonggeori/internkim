import { describe, expect, test } from 'bun:test';
import { currentMonthInTimeZone, todayDateInTimeZone } from '../../../src/routes/attendance/shared/attendance-date';

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
});
