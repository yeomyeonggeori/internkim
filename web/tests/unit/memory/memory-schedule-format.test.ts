import { describe, expect, test } from 'bun:test';
import {
	formatScheduleCronExpression,
	formatScheduleDateTime,
	formatScheduleInterval
} from '../../../src/routes/memory/memory-schedule-format';
import { memoryText } from '../../../src/routes/memory/text';

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
		expect(formatScheduleCronExpression('0 9 * * *', 'ko', memoryText.ko)).toBe('매일 오전 9:00');
		expect(formatScheduleCronExpression('0 9 * * 1-5', 'ko', memoryText.ko)).toBe('평일 오전 9:00');
		expect(formatScheduleCronExpression('0 16 * * 5', 'ko', memoryText.ko)).toBe('매주 금요일 오후 4:00');
		expect(formatScheduleCronExpression('0 18 * * 3', 'ko', memoryText.ko)).toBe('매주 수요일 오후 6:00');
	});

	test('formats common cron schedules in English', () => {
		expect(formatScheduleCronExpression('0 9 * * *', 'en', memoryText.en)).toBe('Every day at 9:00 AM');
		expect(formatScheduleCronExpression('0 9 * * 1-5', 'en', memoryText.en)).toBe('Weekdays at 9:00 AM');
		expect(formatScheduleCronExpression('0 16 * * 5', 'en', memoryText.en)).toBe('Every Friday at 4:00 PM');
	});

	test('falls back to the raw cron expression for unsupported patterns', () => {
		expect(formatScheduleCronExpression('*/15 * * * *', 'ko', memoryText.ko)).toBe('*/15 * * * *');
	});

	test('formats interval schedules with localized templates', () => {
		expect(formatScheduleInterval(3600, memoryText.ko)).toBe('1시간마다');
		expect(formatScheduleInterval(7200, memoryText.ko)).toBe('2시간마다');
		expect(formatScheduleInterval(3600, memoryText.en)).toBe('Every 1 hour');
		expect(formatScheduleInterval(7200, memoryText.en)).toBe('Every 2 hours');
		expect(formatScheduleInterval(300, memoryText.en)).toBe('Every 5 minutes');
		expect(formatScheduleInterval(30, memoryText.en)).toBe('Every 30 seconds');
	});
});
