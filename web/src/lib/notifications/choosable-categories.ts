import { notificationCategories, type NotificationCategory } from './categories';

const onlyAdministratorsAreTold: readonly NotificationCategory[] = ['leave'];

export function categoriesChoosableBy(isAdmin: boolean): NotificationCategory[] {
	if (isAdmin) return [...notificationCategories];
	return notificationCategories.filter((category) => !onlyAdministratorsAreTold.includes(category));
}
