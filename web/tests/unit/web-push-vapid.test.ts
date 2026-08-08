import { describe, expect, test } from 'bun:test';
import { audienceOf, signVapidToken, vapidAuthorization } from '../../src/lib/server/web-push-vapid';
import { decodeBase64URL } from '../../src/lib/notifications/base64url';

const keys = {
	publicKey: 'BG0w6CuCogoJKa593BzjeAk_VAOmSYtz4Crk7OBQPEYa3_peOcMJEln_GG6LyW-0nl82LPHDClzU8_0nB4Z5dcs',
	privateKey: 'NNK7ZJuRBBHnpKs9X0R0aM4Tff6BaVUfPwmnYTdPuWA',
	subject: 'mailto:support@example.com'
};

const endpoint = 'https://fcm.googleapis.com/fcm/send/abc123';
const noon = 1_786_000_000;

function readPart(token: string, index: number): Record<string, unknown> {
	const part = token.split('.')[index];
	return JSON.parse(new TextDecoder().decode(decodeBase64URL(part))) as Record<string, unknown>;
}

async function verifyWithThePublicKey(token: string): Promise<boolean> {
	const point = decodeBase64URL(keys.publicKey);
	const publicKey = await crypto.subtle.importKey(
		'raw',
		point,
		{ name: 'ECDSA', namedCurve: 'P-256' },
		false,
		['verify']
	);
	const [header, payload, signature] = token.split('.');
	return crypto.subtle.verify(
		{ name: 'ECDSA', hash: 'SHA-256' },
		publicKey,
		decodeBase64URL(signature),
		new TextEncoder().encode(`${header}.${payload}`)
	);
}

describe('signVapidToken', () => {
	test('the push service can check the signature against the key the browser subscribed with', async () => {
		expect(await verifyWithThePublicKey(await signVapidToken(endpoint, keys, noon))).toBe(true);
	});

	test('a token signed for one push service does not verify as another', async () => {
		const token = await signVapidToken(endpoint, keys, noon);
		const [header, , signature] = token.split('.');
		const elsewhere = readPart(token, 1);
		elsewhere.aud = 'https://updates.push.services.mozilla.com';
		const swapped = `${header}.${btoa(JSON.stringify(elsewhere)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')}.${signature}`;

		expect(await verifyWithThePublicKey(swapped)).toBe(false);
	});

	test('it names the push service it is meant for, and expires', async () => {
		const token = await signVapidToken(endpoint, keys, noon);

		expect(readPart(token, 0)).toEqual({ typ: 'JWT', alg: 'ES256' });
		expect(readPart(token, 1)).toEqual({
			aud: 'https://fcm.googleapis.com',
			exp: noon + 43_200,
			sub: 'mailto:support@example.com'
		});
	});

	test('a public key that is not a P-256 point is refused rather than signed with', async () => {
		await expect(signVapidToken(endpoint, { ...keys, publicKey: 'AQID' }, noon)).rejects.toThrow(
			'uncompressed P-256 point'
		);
	});
});

describe('audienceOf', () => {
	test('only the origin, because that is what the push service checks', () => {
		expect(audienceOf('https://fcm.googleapis.com/fcm/send/abc123')).toBe('https://fcm.googleapis.com');
		expect(audienceOf('https://web.push.apple.com/QF...long/path?query=1')).toBe('https://web.push.apple.com');
	});
});

describe('vapidAuthorization', () => {
	test('the header carries the token and the key beside it', async () => {
		const authorization = await vapidAuthorization(endpoint, keys, noon);

		expect(authorization.startsWith('vapid t=')).toBe(true);
		expect(authorization.endsWith(`, k=${keys.publicKey}`)).toBe(true);
	});
});
