import { notificationCategories, type NotificationCategory } from '../catalog/notifications';

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

export function storedFromChoices(choices: NotificationChoices): Record<string, boolean> {
	const stored: Record<string, boolean> = {};
	for (const category of notificationCategories) {
		stored[category] = choices[category];
	}
	return stored;
}
