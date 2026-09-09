import type { SupabaseClient } from './service-client.ts';
import type { ApnsKey } from './apns.ts';
import type { FcmKey } from './fcm.ts';
import type { VapidKeys } from './web-push-vapid.ts';

export type PushKeys = {
	vapid: VapidKeys | null;
	apns: ApnsKey | null;
	fcm: FcmKey | null;
};

export const noPushKeys: PushKeys = { vapid: null, apns: null, fcm: null };

export function reachesSomeDevice(keys: PushKeys): boolean {
	return keys.vapid !== null || keys.apns !== null || keys.fcm !== null;
}

export function pushKeysOf(carried: unknown): PushKeys {
	if (typeof carried !== 'object' || carried === null) return noPushKeys;
	const read = carried as { vapid?: unknown; apns?: unknown; fcm?: unknown };
	return { vapid: vapidOf(read.vapid), apns: apnsOf(read.apns), fcm: fcmOf(read.fcm) };
}

export async function pushKeysFromVault(client: SupabaseClient): Promise<PushKeys> {
	const { data, error } = await client.rpc('push_keys_read');
	if (error) return noPushKeys;
	return pushKeysOf(data);
}

function textOf(carried: Record<string, unknown>, field: string): string {
	const value = carried[field];
	return typeof value === 'string' ? value.trim() : '';
}

function vapidOf(carried: unknown): VapidKeys | null {
	if (typeof carried !== 'object' || carried === null) return null;
	const read = carried as Record<string, unknown>;
	const publicKey = textOf(read, 'publicKey');
	const privateKey = textOf(read, 'privateKey');
	const subject = textOf(read, 'subject');
	if (!publicKey || !privateKey || !subject) return null;
	return { publicKey, privateKey, subject };
}

function apnsOf(carried: unknown): ApnsKey | null {
	if (typeof carried !== 'object' || carried === null) return null;
	const read = carried as Record<string, unknown>;
	const keyID = textOf(read, 'keyID');
	const teamID = textOf(read, 'teamID');
	const bundleID = textOf(read, 'bundleID');
	const privateKey = textOf(read, 'privateKey');
	if (!keyID || !teamID || !bundleID || !privateKey) return null;
	const environment = textOf(read, 'environment') === 'sandbox' ? 'sandbox' : 'production';
	return { keyID, teamID, bundleID, privateKey, environment };
}

function fcmOf(carried: unknown): FcmKey | null {
	if (typeof carried !== 'object' || carried === null) return null;
	const read = carried as Record<string, unknown>;
	const projectID = textOf(read, 'projectID');
	const clientEmail = textOf(read, 'clientEmail');
	const privateKey = textOf(read, 'privateKey');
	if (!projectID || !clientEmail || !privateKey) return null;
	return { projectID, clientEmail, privateKey };
}
