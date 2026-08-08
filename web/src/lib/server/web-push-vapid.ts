import { decodeBase64URL, encodeBase64URL } from '$lib/notifications/base64url';

export type VapidKeys = {
	publicKey: string;
	privateKey: string;
	subject: string;
};

const twelveHours = 12 * 60 * 60;

export function audienceOf(endpoint: string): string {
	return new URL(endpoint).origin;
}

export async function vapidAuthorization(endpoint: string, keys: VapidKeys, nowInSeconds: number): Promise<string> {
	const token = await signVapidToken(endpoint, keys, nowInSeconds);
	return `vapid t=${token}, k=${keys.publicKey}`;
}

export async function signVapidToken(endpoint: string, keys: VapidKeys, nowInSeconds: number): Promise<string> {
	const header = encodeJSON({ typ: 'JWT', alg: 'ES256' });
	const payload = encodeJSON({
		aud: audienceOf(endpoint),
		exp: nowInSeconds + twelveHours,
		sub: keys.subject
	});
	const signed = `${header}.${payload}`;

	const signingKey = await importSigningKey(keys);
	const signature = await crypto.subtle.sign(
		{ name: 'ECDSA', hash: 'SHA-256' },
		signingKey,
		new TextEncoder().encode(signed)
	);
	return `${signed}.${encodeBase64URL(signature)}`;
}

function encodeJSON(value: Record<string, unknown>): string {
	return encodeBase64URL(new TextEncoder().encode(JSON.stringify(value)).buffer);
}

async function importSigningKey(keys: VapidKeys): Promise<CryptoKey> {
	const point = decodeBase64URL(keys.publicKey);
	if (point.length !== 65 || point[0] !== 4) {
		throw new Error('the VAPID public key is not an uncompressed P-256 point');
	}
	return crypto.subtle.importKey(
		'jwk',
		{
			kty: 'EC',
			crv: 'P-256',
			d: keys.privateKey,
			x: encodeBase64URL(point.slice(1, 33).buffer),
			y: encodeBase64URL(point.slice(33, 65).buffer),
			ext: true
		},
		{ name: 'ECDSA', namedCurve: 'P-256' },
		false,
		['sign']
	);
}
