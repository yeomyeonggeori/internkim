import { describe, expect, test } from 'bun:test';
import { notificationCategories, readNotificationSettings } from '../../src/lib/notifications/categories';

describe('readNotificationSettings', () => {
	test('an account that has never chosen gets the defaults', () => {
		expect(readNotificationSettings({})).toEqual({
			message: true,
			task: true,
			approval: true,
			attendance: false
		});
	});

	test('a choice is kept and the rest stay at their default', () => {
		expect(readNotificationSettings({ message: false })).toEqual({
			message: false,
			task: true,
			approval: true,
			attendance: false
		});
	});

	test('anything that is not a choice is not one', () => {
		expect(readNotificationSettings({ message: 'false' }).message).toBe(true);
		expect(readNotificationSettings({ message: 0 }).message).toBe(true);
		expect(readNotificationSettings(null).message).toBe(true);
		expect(readNotificationSettings('message').message).toBe(true);
	});

	test('turning the web off for messages leaves the other categories alone', () => {
		const chosen = readNotificationSettings({ message: false });
		expect(chosen.message).toBe(false);
		expect(chosen.task).toBe(true);
		expect(chosen.approval).toBe(true);
	});

	test('a category nobody defined is not carried forward', () => {
		expect(Object.keys(readNotificationSettings({ payroll: true })).sort()).toEqual(
			[...notificationCategories].sort()
		);
	});
});
