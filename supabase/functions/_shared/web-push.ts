import { encryptForSubscription, isUsableSubscriptionKey, type SubscriptionKeys } from './web-push-encrypt.ts';
import { vapidAuthorization, type VapidKeys } from './web-push-vapid.ts';
import { sayPushNotDelivered } from './push-diagnostics.ts';
import type { PushOutcome } from './push-vocabulary.ts';

export type PushTarget = {
	address: string;
	keys: SubscriptionKeys;
};

export type { PushOutcome };

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
	if (!isAddressable(target.address) || !hasUsableKeys(target.keys)) {
		sayPushNotDelivered({ channel: 'web-push', address: target.address, stage: 'subscription', outcome: 'gone' });
		return 'gone';
	}
	const payload = JSON.stringify(notification);

	let sealed: Uint8Array<ArrayBuffer>;
	try {
		sealed = await encryptForSubscription(payload, target.keys);
	} catch (failure) {
		sayPushNotDelivered({ channel: 'web-push', address: target.address, stage: 'encryption', outcome: 'gone', failure });
		return 'gone';
	}

	let authorization: string;
	try {
		authorization = await vapidAuthorization(target.address, vapid, nowInSeconds);
	} catch (failure) {
		sayPushNotDelivered({ channel: 'web-push', address: target.address, stage: 'authorization', outcome: 'refused', failure });
		return 'refused';
	}

	try {
		const response = await fetch(target.address, {
			method: 'POST',
			headers: {
				Authorization: authorization,
				'Content-Encoding': 'aes128gcm',
				'Content-Type': 'application/octet-stream',
				TTL: String(oneDayInSeconds)
			},
			body: sealed
		});
		const outcome = outcomeOfStatus(response.status);
		if (outcome !== 'delivered') {
			sayPushNotDelivered({
				channel: 'web-push',
				address: target.address,
				stage: 'answer',
				outcome,
				status: response.status
			});
		}
		return outcome;
	} catch (failure) {
		sayPushNotDelivered({ channel: 'web-push', address: target.address, stage: 'request', outcome: 'refused', failure });
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
