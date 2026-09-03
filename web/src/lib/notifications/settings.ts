import { invokeTool } from '$lib/public-api-call';
import { isNotificationCategory, type NotificationCategory } from './categories';

export type NotificationChoice = {
	category: NotificationCategory;
	isOn: boolean;
	isChoosable: boolean;
};

export type NotificationSettings = {
	categories: NotificationChoice[];
	mutedConversationIDs: string[];
};

type AnsweredSettings = {
	categories: { category: string; isOn: boolean; isChoosable: boolean }[];
	mutedConversationIDs: string[];
};

function settingsOf(answered: AnsweredSettings): NotificationSettings {
	return {
		categories: answered.categories.flatMap((choice) =>
			isNotificationCategory(choice.category) ? [{ ...choice, category: choice.category }] : []
		),
		mutedConversationIDs: answered.mutedConversationIDs
	};
}

export async function myNotificationSettings(): Promise<NotificationSettings> {
	return settingsOf(await invokeTool<AnsweredSettings>('notification_settings_get', {}));
}

export async function chooseNotificationCategory(
	category: NotificationCategory,
	wanted: boolean
): Promise<NotificationSettings> {
	const chosen = wanted ? { turnOn: [category] } : { turnOff: [category] };
	return settingsOf(await invokeTool<AnsweredSettings>('notification_settings_set', chosen));
}
