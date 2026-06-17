import { describe, expect, test } from 'bun:test';
import { calendarText } from '../../../src/routes/calendar/text';

describe('calendar text', () => {
	test('separates subscription settings from external account status', () => {
		expect(calendarText.en.subscriptionSettings).toBe('Subscription settings');
		expect(calendarText.en.subscriptionReady).toBe('CalDAV/ICS subscription ready');
		expect(calendarText.en.googleCalendarDisconnected).toBe('Google Calendar not connected');
		expect(calendarText.en.googleCalendarConnectedTemplate).toBe('Google Calendar connected: {email}');
		expect(calendarText.en.googleCalendarConnectAction).toBe('Connect Google Calendar');
		expect(calendarText.en.googleCalendarReconnectAction).toBe('Reconnect Google Calendar');
		expect(calendarText.en.googleCalendarReconnectHint).toBe('Reconnect the account to resume remote calendar sync.');
		expect(calendarText.ko.subscriptionSettings).toBe('구독 설정');
		expect(calendarText.ko.subscriptionReady).toBe('CalDAV/ICS 구독 URL 준비됨');
		expect(calendarText.ko.googleCalendarDisconnected).toBe('Google Calendar 미연결');
		expect(calendarText.ko.googleCalendarConnectedTemplate).toBe('Google Calendar 연결됨: {email}');
		expect(calendarText.ko.googleCalendarConnectAction).toBe('Google Calendar 연결');
		expect(calendarText.ko.googleCalendarReconnectAction).toBe('Google Calendar 다시 연결');
		expect(calendarText.ko.googleCalendarReconnectHint).toBe('외부 캘린더 연동을 다시 시작하려면 계정을 다시 연결하세요.');
	});

	test('localizes event audit labels', () => {
		expect(calendarText.en.eventAuditCreated).toBe('Created');
		expect(calendarText.en.eventAuditUpdated).toBe('Updated');
		expect(calendarText.ko.eventAuditCreated).toBe('등록');
		expect(calendarText.ko.eventAuditUpdated).toBe('수정');
	});
});
