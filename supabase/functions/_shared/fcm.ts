import { pkcs8FromPEM, signedJWT } from './jwt.ts';
import { sayPushNotDelivered } from './push-diagnostics.ts';
import type { Notification, PushOutcome } from './push-vocabulary.ts';

export type FcmKey = {
	projectID: string;
	clientEmail: string;
	privateKey: string;
};

const tokenEndpoint = 'https://oauth2.googleapis.com/token';
const messagingScope = 'https://www.googleapis.com/auth/firebase.messaging';
const oneHour = 3600;
const aMinute = 60;

let held: { token: string; expiresAt: number; clientEmail: string } | null = null;

export function outcomeOfFcmAnswer(status: number, errorCode: string): PushOutcome {
	if (status >= 200 && status < 300) return 'delivered';
	if (status === 404 || errorCode === 'UNREGISTERED') return 'gone';
	return 'refused';
}

export function fcmMessage(deviceToken: string, notification: Notification): Record<string, unknown> {
	return {
		message: {
			token: deviceToken,
			notification: { title: notification.title, body: notification.body },
			data: { openPath: notification.openPath, tag: notification.tag },
			android: { notification: { tag: notification.tag } }
		}
	};
}

export async function fcmAccessToken(key: FcmKey, nowInSeconds: number): Promise<string> {
	if (held && held.clientEmail === key.clientEmail && nowInSeconds < held.expiresAt - aMinute) {
		return held.token;
	}
	const assertion = await accessAssertion(key, nowInSeconds);
	const response = await fetch(tokenEndpoint, {
		method: 'POST',
		headers: { 'content-type': 'application/x-www-form-urlencoded' },
		body: new URLSearchParams({
			grant_type: 'urn:ietf:params:oauth:grant-type:jwt-bearer',
			assertion
		})
	});
	const answered = (await response.json().catch(() => null)) as { access_token?: unknown } | null;
	if (!response.ok || typeof answered?.access_token !== 'string') {
		throw new Error(`google refused the messaging token with ${response.status}`);
	}
	held = { token: answered.access_token, expiresAt: nowInSeconds + oneHour, clientEmail: key.clientEmail };
	return held.token;
}

export function forgetFcmAccessToken(): void {
	held = null;
}

export async function sendFcm(
	deviceToken: string,
	notification: Notification,
	key: FcmKey,
	nowInSeconds: number
): Promise<PushOutcome> {
	if (deviceToken === '') return 'gone';

	let accessToken: string;
	try {
		accessToken = await fcmAccessToken(key, nowInSeconds);
	} catch (failure) {
		sayPushNotDelivered({ channel: 'fcm', address: deviceToken, stage: 'authorization', outcome: 'refused', failure });
		return 'refused';
	}

	try {
		const response = await fetch(`https://fcm.googleapis.com/v1/projects/${key.projectID}/messages:send`, {
			method: 'POST',
			headers: { authorization: `Bearer ${accessToken}`, 'content-type': 'application/json' },
			body: JSON.stringify(fcmMessage(deviceToken, notification))
		});
		const reason = await refusalCode(response);
		const outcome = outcomeOfFcmAnswer(response.status, reason);
		if (outcome !== 'delivered') {
			sayPushNotDelivered({
				channel: 'fcm',
				address: deviceToken,
				stage: 'answer',
				outcome,
				status: response.status,
				reason
			});
		}
		return outcome;
	} catch (failure) {
		sayPushNotDelivered({ channel: 'fcm', address: deviceToken, stage: 'request', outcome: 'refused', failure });
		return 'refused';
	}
}

async function accessAssertion(key: FcmKey, nowInSeconds: number): Promise<string> {
	const signingKey = await crypto.subtle.importKey(
		'pkcs8',
		pkcs8FromPEM(key.privateKey) as BufferSource,
		{ name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256' },
		false,
		['sign']
	);
	return signedJWT(
		{ alg: 'RS256', typ: 'JWT' },
		{
			iss: key.clientEmail,
			scope: messagingScope,
			aud: tokenEndpoint,
			iat: nowInSeconds,
			exp: nowInSeconds + oneHour
		},
		signingKey,
		{ name: 'RSASSA-PKCS1-v1_5' }
	);
}

async function refusalCode(response: Response): Promise<string> {
	if (response.ok) return '';
	const answered = (await response.json().catch(() => null)) as {
		error?: { details?: { errorCode?: unknown }[] };
	} | null;
	const carried = answered?.error?.details ?? [];
	for (const detail of carried) {
		if (typeof detail?.errorCode === 'string') return detail.errorCode;
	}
	return '';
}
