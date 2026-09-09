import type { SupabaseClient } from './service-client.ts';
import { sendApns } from './apns.ts';
import { sendFcm } from './fcm.ts';
import { sendWebPush, type PushTarget } from './web-push.ts';
import type { PushKeys } from './push-keys.ts';
import type { Notification, PushOutcome } from './push-vocabulary.ts';

export type { Notification };

export type Reached = {
	reached: number;
	pruned: number;
};

type DeviceRow = { kind: string; address: string; keys: unknown };

export async function pushToMemberDevices(
	client: SupabaseClient,
	memberID: string,
	notification: Notification,
	keys: PushKeys,
	nowInSeconds: number
): Promise<Reached> {
	const devices = await memberDevices(client, memberID);
	let reached = 0;
	let pruned = 0;
	for (const device of devices) {
		const outcome = await sendToDevice(device, notification, keys, nowInSeconds);
		if (outcome === 'delivered') reached += 1;
		if (outcome === 'gone') {
			await forget(client, device.kind, device.address);
			pruned += 1;
		}
	}
	return { reached, pruned };
}

export async function memberDevices(client: SupabaseClient, memberID: string): Promise<DeviceRow[]> {
	const { data, error } = await client
		.from('push_device')
		.select('kind, address, keys')
		.eq('member_id', memberID)
		.returns<DeviceRow[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}

async function sendToDevice(
	device: DeviceRow,
	notification: Notification,
	keys: PushKeys,
	nowInSeconds: number
): Promise<PushOutcome> {
	if (device.kind === 'web-push') {
		const target = asWebPushTarget(device);
		if (!target || !keys.vapid) return 'refused';
		return sendWebPush(target, notification, keys.vapid, nowInSeconds);
	}
	if (device.kind === 'apns') {
		if (!keys.apns) return 'refused';
		return sendApns(device.address, notification, keys.apns, nowInSeconds);
	}
	if (device.kind === 'fcm') {
		if (!keys.fcm) return 'refused';
		return sendFcm(device.address, notification, keys.fcm, nowInSeconds);
	}
	return 'refused';
}

function asWebPushTarget(row: DeviceRow): PushTarget | null {
	if (typeof row.keys !== 'object' || row.keys === null) return null;
	const { p256dh, auth } = row.keys as { p256dh?: unknown; auth?: unknown };
	if (typeof p256dh !== 'string' || typeof auth !== 'string') return null;
	return { address: row.address, keys: { p256dh, auth } };
}

async function forget(client: SupabaseClient, kind: string, address: string): Promise<void> {
	const { error } = await client.from('push_device').delete().eq('kind', kind).eq('address', address);
	if (error) throw new Error(error.message);
}
