import type { SupabaseClient } from './service-client.ts';
import { sendApnsBackground } from './apns-background.ts';
import { sendFcmReadClearing } from './fcm.ts';
import { memberDevices } from './push-to-member-devices.ts';
import type { PushKeys } from './push-keys.ts';
import { readTagKey, type PushOutcome } from './push-vocabulary.ts';

export function readClearingPayload(tag: string): Record<string, unknown> {
	return { aps: { 'content-available': 1 }, [readTagKey]: tag };
}

export async function isMemberOfCompany(client: SupabaseClient, memberID: string, companyID: string): Promise<boolean> {
	const { data, error } = await client
		.from('member')
		.select('id')
		.eq('id', memberID)
		.eq('company_id', companyID)
		.maybeSingle<{ id: string }>();
	if (error) throw new Error(`member ${memberID} could not be read: ${error.message}`);
	return data !== null;
}

export async function clearReadOnMemberDevices(
	client: SupabaseClient,
	memberID: string,
	tag: string,
	keys: PushKeys,
	nowInSeconds: number
): Promise<number> {
	let reached = 0;
	for (const device of await memberDevices(client, memberID)) {
		if ((await clearOnDevice(device, tag, keys, nowInSeconds)) === 'delivered') reached += 1;
	}
	return reached;
}

function clearOnDevice(device: { kind: string; address: string }, tag: string, keys: PushKeys, nowInSeconds: number): Promise<PushOutcome> {
	if (device.kind === 'apns' && keys.apns) return sendApnsBackground(device.address, readClearingPayload(tag), keys.apns, nowInSeconds);
	if (device.kind === 'fcm' && keys.fcm) return sendFcmReadClearing(device.address, tag, keys.fcm, nowInSeconds);
	return Promise.resolve('refused');
}
