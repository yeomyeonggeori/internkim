import type { SupabaseClient } from '@supabase/supabase-js';
import { readNotificationSettings, type NotificationCategory } from '$lib/notifications/categories';
import { pushToMemberDevices, type Notification } from './push-to-member-devices';
import type { VapidKeys } from './web-push-vapid';

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
	vapid: VapidKeys,
	nowInSeconds: number
): Promise<Delivery> {
	if (!(await wantsToBeTold(client, memberID, category))) {
		return { reached: 0, pruned: 0, silent: true };
	}

	const { reached, pruned } = await pushToMemberDevices(client, memberID, notification, vapid, nowInSeconds);
	return { reached, pruned, silent: false };
}

async function wantsToBeTold(
	client: SupabaseClient,
	memberID: string,
	category: NotificationCategory
): Promise<boolean> {
	const { data, error } = await client
		.from('member')
		.select('notification_settings')
		.eq('id', memberID)
		.maybeSingle<{ notification_settings: unknown }>();
	if (error) throw new Error(error.message);
	return readNotificationSettings(data?.notification_settings)[category];
}
