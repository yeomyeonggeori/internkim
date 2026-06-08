import { describe, expect, test } from 'bun:test';
import { formatScheduleCronExpression, formatScheduleDateTime } from '../../../src/routes/memory/memory-schedule-format';

describe('memory schedule formatting', () => {
	test('uses the browser timezone fallback when a schedule timezone is unavailable', () => {
		const value = '2026-06-09T00:00:00Z';
		const expectedText = new Intl.DateTimeFormat('en-US', {
			dateStyle: 'medium',
			timeStyle: 'short',
			timeZone: 'UTC'
		}).format(new Date(value));

		expect(formatScheduleDateTime(value, undefined, 'en-US', 'UTC')).toBe(expectedText);
		expect(formatScheduleDateTime(value, 'Not/A_Zone', 'en-US', 'UTC')).toBe(expectedText);
	});

	test('uses a valid schedule timezone before the fallback timezone', () => {
		const value = '2026-06-09T00:00:00Z';
		const expectedText = new Intl.DateTimeFormat('en-US', {
			dateStyle: 'medium',
			timeStyle: 'short',
			timeZone: 'Asia/Seoul'
		}).format(new Date(value));

		expect(formatScheduleDateTime(value, 'Asia/Seoul', 'en-US', 'UTC')).toBe(expectedText);
	});

	test('formats common cron schedules in Korean', () => {
		expect(formatScheduleCronExpression('0 9 * * *', 'ko')).toBe('매일 오전 9:00');
		expect(formatScheduleCronExpression('0 9 * * 1-5', 'ko')).toBe('평일 오전 9:00');
		expect(formatScheduleCronExpression('0 16 * * 5', 'ko')).toBe('매주 금요일 오후 4:00');
		expect(formatScheduleCronExpression('0 18 * * 3', 'ko')).toBe('매주 수요일 오후 6:00');
	});

	test('formats common cron schedules in English', () => {
		expect(formatScheduleCronExpression('0 9 * * *', 'en')).toBe('Every day at 9:00 AM');
		expect(formatScheduleCronExpression('0 9 * * 1-5', 'en')).toBe('Weekdays at 9:00 AM');
		expect(formatScheduleCronExpression('0 16 * * 5', 'en')).toBe('Every Friday at 4:00 PM');
	});

	test('falls back to the raw cron expression for unsupported patterns', () => {
		expect(formatScheduleCronExpression('*/15 * * * *', 'ko')).toBe('*/15 * * * *');
	});
});
