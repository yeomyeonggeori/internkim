export const notificationCategories = [
	'message',
	'task',
	'approval',
	'attendance',
	'leave',
	'calendar',
	'mail'
] as const;

export type NotificationCategory = (typeof notificationCategories)[number];

export type NotificationSettings = {
	categories: Record<NotificationCategory, boolean>;
	calendarAt: string;
};

const notifiesByDefault: Record<NotificationCategory, boolean> = {
	message: true,
	task: true,
	approval: true,
	attendance: true,
	leave: true,
	calendar: true,
	mail: false
};

const morning = '08:00';

export function readNotificationSettings(stored: unknown): NotificationSettings {
	const settings: NotificationSettings = { categories: { ...notifiesByDefault }, calendarAt: morning };
	if (typeof stored !== 'object' || stored === null) return settings;

	const held = stored as Record<string, unknown>;
	for (const category of notificationCategories) {
		if (typeof held[category] === 'boolean') settings.categories[category] = held[category];
	}
	settings.calendarAt = readTimeOfDay(held.calendarAt) || morning;
	return settings;
}

export function writeNotificationSettings(settings: NotificationSettings): Record<string, boolean | string> {
	const stored: Record<string, boolean | string> = { calendarAt: settings.calendarAt };
	for (const category of notificationCategories) {
		stored[category] = settings.categories[category];
	}
	return stored;
}

export function readTimeOfDay(offered: unknown): string {
	if (typeof offered !== 'string') return '';
	const said = /^([01][0-9]|2[0-3]):([0-5][0-9])$/.exec(offered.trim());
	return said ? `${said[1]}:${said[2]}` : '';
}
