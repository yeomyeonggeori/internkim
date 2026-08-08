export const notificationCategories = ['message', 'task', 'approval', 'attendance'] as const;

export type NotificationCategory = (typeof notificationCategories)[number];

export type NotificationSettings = Record<NotificationCategory, boolean>;

const notifiesByDefault: NotificationSettings = {
	message: true,
	task: true,
	approval: true,
	attendance: false
};

export function readNotificationSettings(stored: unknown): NotificationSettings {
	const settings = { ...notifiesByDefault };
	if (typeof stored !== 'object' || stored === null) return settings;

	const held = stored as Record<string, unknown>;
	for (const category of notificationCategories) {
		if (typeof held[category] === 'boolean') settings[category] = held[category];
	}
	return settings;
}
