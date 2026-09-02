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

export function readNotificationSettings(stored: unknown): NotificationSettings {
	const settings: NotificationSettings = { categories: { ...notifiesByDefault } };
	if (typeof stored !== 'object' || stored === null) return settings;

	const held = stored as Record<string, unknown>;
	for (const category of notificationCategories) {
		if (typeof held[category] === 'boolean') settings.categories[category] = held[category];
	}
	return settings;
}

export function writeNotificationSettings(settings: NotificationSettings): Record<string, boolean> {
	const stored: Record<string, boolean> = {};
	for (const category of notificationCategories) {
		stored[category] = settings.categories[category];
	}
	return stored;
}
