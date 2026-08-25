import { describe, expect, test } from 'bun:test';
import {
	notificationCategories,
	readNotificationSettings,
	readTimeOfDay,
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
		// Mail arrives all day whether or not it is worth a phone.
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

	test('the schedule goes out in the morning until someone says otherwise', () => {
		expect(readNotificationSettings(null).calendarAt).toBe('08:00');
		expect(readNotificationSettings({ calendarAt: '10:30' }).calendarAt).toBe('10:30');
		expect(readNotificationSettings({ calendarAt: '25:00' }).calendarAt).toBe('08:00');
		expect(readNotificationSettings({ calendarAt: 'morning' }).calendarAt).toBe('08:00');
	});

	test('what is written back is what reading it gives again', () => {
		const chosen = readNotificationSettings({ message: false, mail: true, calendarAt: '09:15' });
		expect(readNotificationSettings(writeNotificationSettings(chosen))).toEqual(chosen);
	});

	test('every category is written, so a new one does not read as a refusal', () => {
		const stored = writeNotificationSettings(readNotificationSettings(null));
		for (const category of notificationCategories) {
			expect(typeof stored[category]).toBe('boolean');
		}
	});
});

describe('a time of day', () => {
	test('is two digits, a colon, and a real hour', () => {
		expect(readTimeOfDay('00:00')).toBe('00:00');
		expect(readTimeOfDay('23:59')).toBe('23:59');
		expect(readTimeOfDay(' 08:00 ')).toBe('08:00');
	});

	test('is nothing when it is not one', () => {
		for (const said of ['8:00', '24:00', '08:60', '0800', '', null, 12]) {
			expect(readTimeOfDay(said)).toBe('');
		}
	});
});
