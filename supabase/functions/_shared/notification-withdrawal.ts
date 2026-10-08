import type { SupabaseClient } from './service-client.ts';
import type { ApnsKey } from './apns.ts';
import { sendApnsBackground } from './apns-background.ts';

export function withdrawalPayload(messageID: string): Record<string, unknown> {
	return { aps: { 'content-available': 1 }, withdrawnMessageIDs: [messageID] };
}

export async function withdrawFromMemberPhones(
	client: SupabaseClient,
	memberID: string,
	messageID: string,
	key: ApnsKey,
	nowInSeconds: number
): Promise<number> {
	const { data, error } = await client
		.from('push_device')
		.select('address')
		.eq('member_id', memberID)
		.eq('kind', 'apns')
		.returns<{ address: string }[]>();
	if (error) throw new Error(`the phones of member ${memberID} could not be read: ${error.message}`);

	let reached = 0;
	for (const device of data ?? []) {
		if ((await sendApnsBackground(device.address, withdrawalPayload(messageID), key, nowInSeconds)) === 'delivered') {
			reached += 1;
		}
	}
	return reached;
}
