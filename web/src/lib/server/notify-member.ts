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
	nowInSeconds: number,
	conversationID = ''
): Promise<Delivery> {
	if (!(await wantsToBeTold(client, memberID, category))) {
		return { reached: 0, pruned: 0, silent: true };
	}
	if (await hasMuted(client, memberID, conversationID)) {
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

// A muted conversation is a row; silence is the exception, so a member who has
// muted nothing costs one lookup that finds nothing.
async function hasMuted(client: SupabaseClient, memberID: string, conversationID: string): Promise<boolean> {
	if (!conversationID) return false;
	const { data, error } = await client
		.from('muted_conversation')
		.select('conversation_id')
		.eq('member_id', memberID)
		.eq('conversation_id', conversationID)
		.maybeSingle<{ conversation_id: string }>();
	if (error) throw new Error(error.message);
	return data !== null;
}
