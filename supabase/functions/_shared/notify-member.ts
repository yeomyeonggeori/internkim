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
	if (!(await wantsToBeTold(client, memberID, category))) {
		return { reached: 0, pruned: 0, silent: true };
	}
	if (await hasMuted(client, memberID, conversationID)) {
		return { reached: 0, pruned: 0, silent: true };
	}

	const { reached, pruned } = await pushToMemberDevices(client, memberID, notification, pushKeys, nowInSeconds);
	return { reached, pruned, silent: false };
}

async function wantsToBeTold(
	client: SupabaseClient,
	memberID: string,
	category: NotificationCategory
): Promise<boolean> {
	const { data, error } = await client
		.from('member')
		.select('preferences')
		.eq('id', memberID)
		.maybeSingle<{ preferences: unknown }>();
	if (error) throw new Error(error.message);
	const preferences = data?.preferences;
	const stored = typeof preferences === 'object' && preferences !== null ? Reflect.get(preferences, 'notificationSettings') : undefined;
	return readNotificationSettings(stored).categories[category];
}

async function hasMuted(client: SupabaseClient, memberID: string, conversationID: string): Promise<boolean> {
	if (!conversationID) return false;
	const { data, error } = await client
		.from('notification')
		.select('is_muted')
		.eq('member_id', memberID)
		.eq('conversation_id', conversationID)
		.maybeSingle<{ is_muted: boolean }>();
	if (error) throw new Error(error.message);
	return data?.is_muted === true;
}
