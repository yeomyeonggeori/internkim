import { describe, expect, test } from 'bun:test';
import { calendarText } from '../../../src/routes/calendar/text';

describe('calendar text', () => {
	test('names the subscription settings', () => {
		expect(calendarText.en.subscriptionSettings).toBe('Subscription settings');
		expect(calendarText.en.subscriptionReady).toBe('Subscription URL ready');
		expect(calendarText.en.settings).toBe('Settings');
		expect(calendarText.ko.subscriptionSettings).toBe('구독 설정');
		expect(calendarText.ko.subscriptionReady).toBe('구독 URL 준비됨');
		expect(calendarText.ko.settings).toBe('설정');
	});
});
