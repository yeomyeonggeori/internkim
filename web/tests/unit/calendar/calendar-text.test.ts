import { describe, expect, test } from 'bun:test';
import { calendarText } from '../../../src/routes/calendar/text';

describe('calendar text', () => {
	test('separates subscription settings from external account status', () => {
		expect(calendarText.en.subscriptionSettings).toBe('Subscription settings');
		expect(calendarText.en.subscriptionReady).toBe('CalDAV/ICS subscription ready');
		expect(calendarText.en.googleCalendarDisconnected).toBe('Google Calendar not connected');
		expect(calendarText.en.googleCalendarConnectedTemplate).toBe('Google Calendar connected: {email}');
		expect(calendarText.ko.subscriptionSettings).toBe('구독 설정');
		expect(calendarText.ko.subscriptionReady).toBe('CalDAV/ICS 구독 URL 준비됨');
		expect(calendarText.ko.googleCalendarDisconnected).toBe('Google Calendar 미연결');
		expect(calendarText.ko.googleCalendarConnectedTemplate).toBe('Google Calendar 연결됨: {email}');
	});
});
