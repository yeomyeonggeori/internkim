import type { SupabaseClient } from '@supabase/supabase-js';
import { sendWebPush, type PushTarget } from './web-push';
import type { VapidKeys } from './web-push-vapid';

export type Notification = {
	title: string;
	body: string;
	openPath: string;
	tag: string;
	icon?: string;
};

export type Reached = {
	reached: number;
	pruned: number;
};

type DeviceRow = { kind: string; address: string; keys: unknown };

export async function pushToMemberDevices(
	client: SupabaseClient,
	memberID: string,
	notification: Notification,
	vapid: VapidKeys,
	nowInSeconds: number
): Promise<Reached> {
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
	return { reached, pruned };
}

export async function webPushDevices(client: SupabaseClient, memberID: string): Promise<PushTarget[]> {
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
