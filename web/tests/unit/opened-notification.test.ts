import { describe, expect, test } from 'bun:test';
import { openedNotificationMessage, pathOfOpenedNotification } from '../../src/lib/notifications/opened-notification';

describe('pathOfOpenedNotification', () => {
	test('a path the worker sent is followed', () => {
		expect(pathOfOpenedNotification({ type: openedNotificationMessage, openPath: '/calendar/' })).toBe('/calendar/');
	});

	test('another message the page receives is not a place to go', () => {
		expect(pathOfOpenedNotification({ type: 'something-else', openPath: '/calendar/' })).toBe('');
		expect(pathOfOpenedNotification(null)).toBe('');
		expect(pathOfOpenedNotification('/calendar/')).toBe('');
	});

	test('nothing can send the page to another host', () => {
		expect(pathOfOpenedNotification({ type: openedNotificationMessage, openPath: 'https://example.test/steal' })).toBe('');
		expect(pathOfOpenedNotification({ type: openedNotificationMessage, openPath: '//example.test/steal' })).toBe('');
		expect(pathOfOpenedNotification({ type: openedNotificationMessage, openPath: 'javascript:alert(1)' })).toBe('');
	});
});
