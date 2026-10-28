import type { SupabaseClient } from './service-client.ts';
import { readNotificationSettings, type NotificationCategory } from './categories.ts';
import { pushToMemberDevices, type Notification } from './push-to-member-devices.ts';
import type { PushKeys } from './push-keys.ts';

export type { Notification };

export type Delivery = {
	reached: number;
	pruned: number;
	silent: boolean;
};

export async function notifyMember(
	client: SupabaseClient,
	memberID: string,
	category: NotificationCategory,
	notification: Notification,
	pushKeys: PushKeys,
	nowInSeconds: number,
	conversationID = ''
): Promise<Delivery> {
	const preferences = await preferencesOf(client, memberID);
	if (!wantsToBeTold(preferences, category) || hasMuted(preferences, conversationID)) {
		return { reached: 0, pruned: 0, silent: true };
	}

	const { reached, pruned } = await pushToMemberDevices(client, memberID, notification, pushKeys, nowInSeconds);
	return { reached, pruned, silent: false };
}

async function preferencesOf(client: SupabaseClient, memberID: string): Promise<object> {
	const { data, error } = await client
		.from('member')
		.select('preferences')
		.eq('id', memberID)
		.maybeSingle<{ preferences: unknown }>();
	if (error) throw new Error(error.message);
	const preferences = data?.preferences;
	return typeof preferences === 'object' && preferences !== null ? preferences : {};
}

function wantsToBeTold(preferences: object, category: NotificationCategory): boolean {
	return readNotificationSettings(Reflect.get(preferences, 'notificationSettings')).categories[category];
}

function hasMuted(preferences: object, conversationID: string): boolean {
	if (!conversationID) return false;
	const muted = Reflect.get(preferences, 'mutedChannels');
	return Array.isArray(muted) && muted.includes(conversationID);
}
