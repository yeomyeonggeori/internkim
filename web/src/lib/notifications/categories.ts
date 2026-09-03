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

export function isNotificationCategory(value: string): value is NotificationCategory {
	return notificationCategories.some((category) => category === value);
}
