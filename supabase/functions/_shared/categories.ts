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

export const taskReaches = [...attendanceReaches, 'involved'] as const;

export type TaskReach = (typeof taskReaches)[number];

export const defaultTaskReach: TaskReach = 'team';

export type NotificationSettings = {
	categories: Record<NotificationCategory, boolean>;
	attendanceReach: AttendanceReach;
	taskReach: TaskReach;
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
	const settings: NotificationSettings = {
		categories: { ...notifiesByDefault },
		attendanceReach: 'everyone',
		taskReach: defaultTaskReach
	};
	if (typeof stored !== 'object' || stored === null) return settings;

	const held = stored as Record<string, unknown>;
	for (const category of notificationCategories) {
		if (typeof held[category] === 'boolean') settings.categories[category] = held[category];
	}
	const reach = attendanceReaches.find((known) => known === held.attendanceReach);
	if (reach) settings.attendanceReach = reach;
	const taskReach = taskReaches.find((known) => known === held.taskReach);
	if (taskReach) settings.taskReach = taskReach;
	return settings;
}
