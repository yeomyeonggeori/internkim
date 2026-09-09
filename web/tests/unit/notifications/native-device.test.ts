import { describe, expect, test } from 'bun:test';
import {
	openPathOfTappedNotification,
	reachabilityOfPermission
} from '../../../src/lib/notifications/native-device';

describe('the path a tapped notification points at', () => {
	test('is the openPath the sender carried', () => {
		expect(openPathOfTappedNotification({ notification: { data: { openPath: '/attendance/' } } })).toBe(
			'/attendance/'
		);
	});

	test('is nothing when the notification carries no data', () => {
		expect(openPathOfTappedNotification({ notification: {} })).toBe('');
		expect(openPathOfTappedNotification({})).toBe('');
		expect(openPathOfTappedNotification(null)).toBe('');
	});

	test('is nothing when the path would leave this app', () => {
		expect(openPathOfTappedNotification({ notification: { data: { openPath: '//example.com/' } } })).toBe('');
		expect(openPathOfTappedNotification({ notification: { data: { openPath: 'https://example.com/' } } })).toBe('');
		expect(openPathOfTappedNotification({ notification: { data: { openPath: 42 } } })).toBe('');
	});
});

describe('what a permission answer says about being reached', () => {
	test('granted is on and denied is blocked', () => {
		expect(reachabilityOfPermission('granted')).toBe('on');
		expect(reachabilityOfPermission('denied')).toBe('blocked');
	});

	test('an answer still to be asked for is off rather than blocked', () => {
		expect(reachabilityOfPermission('prompt')).toBe('off');
		expect(reachabilityOfPermission('prompt-with-rationale')).toBe('off');
	});
});
