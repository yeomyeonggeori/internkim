import type { SupabaseClient } from '@supabase/supabase-js';
import { readNotificationSettings, type NotificationCategory } from '$lib/notifications/categories';
import { sendWebPush, type PushTarget } from './web-push';
import type { VapidKeys } from './web-push-vapid';

export type Notification = {
	title: string;
	body: string;
	openPath: string;
	tag: string;
};

export type Delivery = {
	reached: number;
	pruned: number;
	silent: boolean;
};

type DeviceRow = { kind: string; address: string; keys: unknown };

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

	const devices = await webPushDevices(client, memberID);
	let reached = 0;
	let pruned = 0;
	for (const device of devices) {
		const outcome = await sendWebPush(device, notification, vapid, nowInSeconds);
		if (outcome === 'delivered') reached += 1;
		if (outcome === 'gone') {
			await forget(client, device.address);
			pruned += 1;
		}
	}
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

async function webPushDevices(client: SupabaseClient, memberID: string): Promise<PushTarget[]> {
	const { data, error } = await client
		.from('push_device')
		.select('kind, address, keys')
		.eq('member_id', memberID)
		.eq('kind', 'web-push')
		.returns<DeviceRow[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map(asTarget).filter((target): target is PushTarget => target !== null);
}

function asTarget(row: DeviceRow): PushTarget | null {
	if (typeof row.keys !== 'object' || row.keys === null) return null;
	const { p256dh, auth } = row.keys as { p256dh?: unknown; auth?: unknown };
	if (typeof p256dh !== 'string' || typeof auth !== 'string') return null;
	return { address: row.address, keys: { p256dh, auth } };
}

async function forget(client: SupabaseClient, address: string): Promise<void> {
	const { error } = await client.from('push_device').delete().eq('address', address);
	if (error) throw new Error(error.message);
}
