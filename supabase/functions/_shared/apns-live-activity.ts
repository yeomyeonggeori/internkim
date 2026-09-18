import { apnsAuthorization, apnsHostOf, outcomeOfApnsAnswer, type ApnsKey } from './apns.ts';
import { sayPushNotDelivered } from './push-diagnostics.ts';
import type { PushOutcome } from './push-vocabulary.ts';

export const attendanceActivityAttributesType = 'AttendanceActivityAttributes';

export type AttendanceActivityState = { startedAt: number; location: string };

export type AttendanceActivityChange =
	| { event: 'start'; state: AttendanceActivityState; alert: { title: string; body: string } }
	| { event: 'end'; state: AttendanceActivityState };

export function liveActivityTopicOf(key: ApnsKey): string {
	return `${key.bundleID}.push-type.liveactivity`;
}

export function liveActivityPayload(
	change: AttendanceActivityChange,
	nowInSeconds: number
): Record<string, unknown> {
	const aps: Record<string, unknown> = {
		timestamp: nowInSeconds,
		event: change.event,
		'content-state': change.state
	};
	if (change.event === 'start') {
		aps['attributes-type'] = attendanceActivityAttributesType;
		aps.attributes = {};
		aps.alert = change.alert;
	}
	if (change.event === 'end') aps['dismissal-date'] = nowInSeconds;
	return { aps };
}

export async function sendLiveActivity(
	activityToken: string,
	change: AttendanceActivityChange,
	key: ApnsKey,
	nowInSeconds: number
): Promise<PushOutcome> {
	if (activityToken === '') return 'gone';

	let authorization: string;
	try {
		authorization = await apnsAuthorization(key, nowInSeconds);
	} catch (failure) {
		sayPushNotDelivered({ channel: 'apns', address: activityToken, stage: 'authorization', outcome: 'refused', failure });
		return 'refused';
	}

	try {
		const response = await fetch(`https://${apnsHostOf(key)}/3/device/${activityToken}`, {
			method: 'POST',
			headers: {
				authorization,
				'apns-topic': liveActivityTopicOf(key),
				'apns-push-type': 'liveactivity',
				'apns-priority': '10',
				'content-type': 'application/json'
			},
			body: JSON.stringify(liveActivityPayload(change, nowInSeconds))
		});
		const answered = response.ok ? null : ((await response.json().catch(() => null)) as { reason?: unknown } | null);
		const reason = typeof answered?.reason === 'string' ? answered.reason : '';
		const outcome = outcomeOfApnsAnswer(response.status, reason);
		if (outcome !== 'delivered') {
			sayPushNotDelivered({
				channel: 'apns',
				address: activityToken,
				stage: 'answer',
				outcome,
				status: response.status,
				reason
			});
		}
		return outcome;
	} catch (failure) {
		sayPushNotDelivered({ channel: 'apns', address: activityToken, stage: 'request', outcome: 'refused', failure });
		return 'refused';
	}
}
