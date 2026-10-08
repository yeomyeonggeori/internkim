import { apnsAuthorization, apnsHostOf, outcomeOfApnsAnswer, type ApnsKey } from './apns.ts';
import { sayPushNotDelivered } from './push-diagnostics.ts';
import type { PushOutcome } from './push-vocabulary.ts';

export async function sendApnsBackground(
	deviceToken: string,
	payload: Record<string, unknown>,
	key: ApnsKey,
	nowInSeconds: number
): Promise<PushOutcome> {
	if (deviceToken === '') return 'gone';

	let authorization: string;
	try {
		authorization = await apnsAuthorization(key, nowInSeconds);
	} catch (failure) {
		sayPushNotDelivered({ channel: 'apns', address: deviceToken, stage: 'authorization', outcome: 'refused', failure });
		return 'refused';
	}

	try {
		const response = await fetch(`https://${apnsHostOf(key)}/3/device/${deviceToken}`, {
			method: 'POST',
			headers: {
				authorization,
				'apns-topic': key.bundleID,
				'apns-push-type': 'background',
				'apns-priority': '5',
				'content-type': 'application/json'
			},
			body: JSON.stringify(payload)
		});
		const answered = response.ok ? null : ((await response.json().catch(() => null)) as { reason?: unknown } | null);
		const reason = typeof answered?.reason === 'string' ? answered.reason : '';
		const outcome = outcomeOfApnsAnswer(response.status, reason);
		if (outcome !== 'delivered') {
			sayPushNotDelivered({ channel: 'apns', address: deviceToken, stage: 'answer', outcome, status: response.status, reason });
		}
		return outcome;
	} catch (failure) {
		sayPushNotDelivered({ channel: 'apns', address: deviceToken, stage: 'request', outcome: 'refused', failure });
		return 'refused';
	}
}
