import { describe, expect, test } from 'bun:test';
import {
	notificationCategories,
	readNotificationSettings,
	writeNotificationSettings
} from '../../src/lib/notifications/categories';

describe('what a member is told about', () => {
	test('a member who has chosen nothing hears everything except mail', () => {
		const settings = readNotificationSettings(null);
		expect(settings.categories.message).toBe(true);
		expect(settings.categories.task).toBe(true);
		expect(settings.categories.approval).toBe(true);
		expect(settings.categories.attendance).toBe(true);
		expect(settings.categories.leave).toBe(true);
		expect(settings.categories.calendar).toBe(true);
		expect(settings.categories.mail).toBe(false);
	});

	test('a stored choice wins over the default', () => {
		const settings = readNotificationSettings({ message: false, mail: true });
		expect(settings.categories.message).toBe(false);
		expect(settings.categories.mail).toBe(true);
		expect(settings.categories.task).toBe(true);
	});

	test('a value that is not a boolean is not a choice', () => {
		const settings = readNotificationSettings({ message: 'no', task: 1, approval: null });
		expect(settings.categories.message).toBe(true);
		expect(settings.categories.task).toBe(true);
		expect(settings.categories.approval).toBe(true);
	});

	test('what is written back is what reading it gives again', () => {
		const chosen = readNotificationSettings({ message: false, mail: true });
		expect(readNotificationSettings(writeNotificationSettings(chosen))).toEqual(chosen);
	});

	test('every category is written, so a new one does not read as a refusal', () => {
		const stored = writeNotificationSettings(readNotificationSettings(null));
		for (const category of notificationCategories) {
			expect(typeof stored[category]).toBe('boolean');
		}
	});
});
