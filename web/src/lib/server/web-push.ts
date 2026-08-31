import { encryptForSubscription, isUsableSubscriptionKey, type SubscriptionKeys } from './web-push-encrypt';
import { vapidAuthorization, type VapidKeys } from './web-push-vapid';

export type PushTarget = {
	address: string;
	keys: SubscriptionKeys;
};

export type PushOutcome = 'delivered' | 'gone' | 'refused';

const oneDayInSeconds = 86_400;

export function outcomeOfStatus(status: number): PushOutcome {
	if (status === 404 || status === 410) return 'gone';
	if (status >= 200 && status < 300) return 'delivered';
	return 'refused';
}

export async function sendWebPush(
	target: PushTarget,
	notification: unknown,
	vapid: VapidKeys,
	nowInSeconds: number
): Promise<PushOutcome> {
	if (!isAddressable(target.address) || !hasUsableKeys(target.keys)) return 'gone';
	const payload = JSON.stringify(notification);

	let sealed: Uint8Array<ArrayBuffer>;
	try {
		sealed = await encryptForSubscription(payload, target.keys);
	} catch {
		return 'gone';
	}

	try {
		const response = await fetch(target.address, {
			method: 'POST',
			headers: {
				Authorization: await vapidAuthorization(target.address, vapid, nowInSeconds),
				'Content-Encoding': 'aes128gcm',
				'Content-Type': 'application/octet-stream',
				TTL: String(oneDayInSeconds)
			},
			body: sealed
		});
		return outcomeOfStatus(response.status);
	} catch {
		return 'refused';
	}
}

function isAddressable(address: string): boolean {
	try {
		new URL(address);
		return true;
	} catch {
		return false;
	}
}

function hasUsableKeys(keys: SubscriptionKeys): boolean {
	try {
		return isUsableSubscriptionKey(keys);
	} catch {
		return false;
	}
}
