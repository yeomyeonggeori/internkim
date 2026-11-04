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

export const attendanceReaches = ['everyone', 'teamAndSupervisors', 'team'] as const;

export type AttendanceReach = (typeof attendanceReaches)[number];

export function isNotificationCategory(value: string): value is NotificationCategory {
	return notificationCategories.some((category) => category === value);
}

export function isAttendanceReach(value: string): value is AttendanceReach {
	return attendanceReaches.some((reach) => reach === value);
}
