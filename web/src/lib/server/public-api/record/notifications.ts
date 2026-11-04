import {
	attendanceReaches,
	notificationCategories,
	type AttendanceReach,
	type NotificationCategory
} from '../catalog/notifications';

export type NotificationChoices = Record<NotificationCategory, boolean>;

const notifiesByDefault: NotificationChoices = {
	message: true,
	task: true,
	approval: true,
	attendance: true,
	leave: true,
	calendar: true,
	mail: false
};

const onlyAdministratorsAreTold: readonly NotificationCategory[] = ['leave'];

export function categoriesChoosableBy(isAdmin: boolean): NotificationCategory[] {
	if (isAdmin) return [...notificationCategories];
	return notificationCategories.filter((category) => !onlyAdministratorsAreTold.includes(category));
}

export function isNotificationCategory(value: string): value is NotificationCategory {
	return notificationCategories.some((category) => category === value);
}

export function choicesFromStored(stored: unknown): NotificationChoices {
	const choices: NotificationChoices = { ...notifiesByDefault };
	if (typeof stored !== 'object' || stored === null) return choices;

	const held = stored as Record<string, unknown>;
	for (const category of notificationCategories) {
		if (typeof held[category] === 'boolean') choices[category] = held[category];
	}
	return choices;
}

export function isAttendanceReach(value: string): value is AttendanceReach {
	return attendanceReaches.some((reach) => reach === value);
}

export function attendanceReachFromStored(stored: unknown): AttendanceReach {
	if (typeof stored !== 'object' || stored === null) return 'everyone';
	const reach = Reflect.get(stored, 'attendanceReach');
	return typeof reach === 'string' && isAttendanceReach(reach) ? reach : 'everyone';
}

export function storedFromChoices(
	choices: NotificationChoices,
	attendanceReach: AttendanceReach
): Record<string, boolean | string> {
	const stored: Record<string, boolean | string> = { attendanceReach };
	for (const category of notificationCategories) {
		stored[category] = choices[category];
	}
	return stored;
}
