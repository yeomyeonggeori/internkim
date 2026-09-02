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

	test('localizes an event version conflict', () => {
		expect(calendarText.ko.calendarEventVersionConflictError).toBe(
			'다른 곳에서 이 일정이 변경되었습니다. 서버의 최신 내용을 다시 불러왔습니다.'
		);
		expect(calendarText.en.calendarEventVersionConflictError).toBe(
			'This event changed elsewhere. The latest server version has been reloaded.'
		);
	});

	test('localizes a delete version conflict', () => {
		expect(calendarText.ko.calendarDeleteVersionConflictError).toBe(
			'다른 곳에서 이 일정이 변경되어 삭제하지 못했습니다. 서버의 최신 내용을 다시 불러왔습니다.'
		);
		expect(calendarText.en.calendarDeleteVersionConflictError).toBe(
			'This event changed elsewhere, so it could not be deleted. The latest server version has been reloaded.'
		);
	});
});
