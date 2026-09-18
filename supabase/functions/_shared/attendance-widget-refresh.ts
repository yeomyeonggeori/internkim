import type { SupabaseClient } from './service-client.ts';
import { apnsAuthorization, apnsHostOf, outcomeOfApnsAnswer, type ApnsKey } from './apns.ts';
import { sayPushNotDelivered } from './push-diagnostics.ts';
import type { PushKeys } from './push-keys.ts';
import type { PushOutcome } from './push-vocabulary.ts';

export function widgetRefreshPayload(clock: string): Record<string, unknown> {
	return { aps: { 'content-available': 1 }, widget: 'attendance', clock };
}

export async function sendWidgetRefresh(
	deviceToken: string,
	clock: string,
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
			body: JSON.stringify(widgetRefreshPayload(clock))
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

export async function refreshOwnWidgets(
	record: SupabaseClient,
	memberID: string,
	clock: string,
	pushKeys: PushKeys,
	nowInSeconds: number
): Promise<number> {
	if (!pushKeys.apns) return 0;
	const { data, error } = await record
		.from('push_device')
		.select('address')
		.eq('member_id', memberID)
		.eq('kind', 'apns')
		.returns<{ address: string }[]>();
	if (error) throw new Error(error.message);

	let reached = 0;
	for (const device of data ?? []) {
		if ((await sendWidgetRefresh(device.address, clock, pushKeys.apns, nowInSeconds)) === 'delivered') reached += 1;
	}
	return reached;
}
