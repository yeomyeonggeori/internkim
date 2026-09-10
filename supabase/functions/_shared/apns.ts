import { pkcs8FromPEM, signedJWT } from './jwt.ts';
import { sayPushNotDelivered } from './push-diagnostics.ts';
import type { Notification, PushOutcome } from './push-vocabulary.ts';

export type ApnsKey = {
	keyID: string;
	teamID: string;
	bundleID: string;
	privateKey: string;
	environment: 'production' | 'sandbox';
};

const halfAnHour = 1800;

let held: { token: string; issuedAt: number; keyID: string } | null = null;

export function apnsHostOf(key: ApnsKey): string {
	return key.environment === 'sandbox' ? 'api.sandbox.push.apple.com' : 'api.push.apple.com';
}

export function outcomeOfApnsAnswer(status: number, reason: string): PushOutcome {
	if (status >= 200 && status < 300) return 'delivered';
	if (status === 410) return 'gone';
	if (status === 400 && (reason === 'BadDeviceToken' || reason === 'DeviceTokenNotForTopic')) return 'gone';
	return 'refused';
}

export function apnsPayload(notification: Notification): Record<string, unknown> {
	return {
		aps: {
			alert: { title: notification.title, body: notification.body },
			sound: 'default',
			'thread-id': notification.tag
		},
		openPath: notification.openPath
	};
}

export async function apnsAuthorization(key: ApnsKey, nowInSeconds: number): Promise<string> {
	if (held && held.keyID === key.keyID && nowInSeconds - held.issuedAt < halfAnHour) {
		return `bearer ${held.token}`;
	}
	const signingKey = await crypto.subtle.importKey(
		'pkcs8',
		pkcs8FromPEM(key.privateKey) as BufferSource,
		{ name: 'ECDSA', namedCurve: 'P-256' },
		false,
		['sign']
	);
	const token = await signedJWT(
		{ alg: 'ES256', kid: key.keyID },
		{ iss: key.teamID, iat: nowInSeconds },
		signingKey,
		{ name: 'ECDSA', hash: 'SHA-256' }
	);
	held = { token, issuedAt: nowInSeconds, keyID: key.keyID };
	return `bearer ${token}`;
}

export function forgetApnsAuthorization(): void {
	held = null;
}

export async function sendApns(
	deviceToken: string,
	notification: Notification,
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
				'apns-push-type': 'alert',
				'apns-priority': '10',
				'content-type': 'application/json'
			},
			body: JSON.stringify(apnsPayload(notification))
		});
		const reason = await refusalReason(response);
		const outcome = outcomeOfApnsAnswer(response.status, reason);
		if (outcome !== 'delivered') {
			sayPushNotDelivered({
				channel: 'apns',
				address: deviceToken,
				stage: 'answer',
				outcome,
				status: response.status,
				reason
			});
		}
		return outcome;
	} catch (failure) {
		sayPushNotDelivered({ channel: 'apns', address: deviceToken, stage: 'request', outcome: 'refused', failure });
		return 'refused';
	}
}

async function refusalReason(response: Response): Promise<string> {
	if (response.ok) return '';
	const answered = (await response.json().catch(() => null)) as { reason?: unknown } | null;
	return typeof answered?.reason === 'string' ? answered.reason : '';
}
