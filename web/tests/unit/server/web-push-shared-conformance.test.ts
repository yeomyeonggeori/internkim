import { describe, expect, test } from 'bun:test';
import { decodeBase64URL, encodeBase64URL } from '../../../src/lib/notifications/base64url';
import {
	encryptForSubscription as webEncrypt,
	isUsableSubscriptionKey as webIsUsable
} from '../../../src/lib/server/web-push-encrypt';
import { signVapidToken as webSignVapidToken } from '../../../src/lib/server/web-push-vapid';
import { outcomeOfStatus as webOutcomeOfStatus, sendWebPush as webSendWebPush } from '../../../src/lib/server/web-push';
import {
	encryptForSubscription as sharedEncrypt,
	isUsableSubscriptionKey as sharedIsUsable
} from '../../../../supabase/functions/_shared/web-push-encrypt.ts';
import { signVapidToken as sharedSignVapidToken } from '../../../../supabase/functions/_shared/web-push-vapid.ts';
import {
	outcomeOfStatus as sharedOutcomeOfStatus,
	sendWebPush as sharedSendWebPush
} from '../../../../supabase/functions/_shared/web-push.ts';

const endpoint = 'https://push.example.com/send/a-subscription';
const now = 1_756_000_000;

async function aVapidPair() {
	const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, ['sign', 'verify']);
	const publicRaw = await crypto.subtle.exportKey('raw', pair.publicKey);
	const privateJWK = await crypto.subtle.exportKey('jwk', pair.privateKey);
	return {
		keys: {
			publicKey: encodeBase64URL(publicRaw),
			privateKey: privateJWK.d ?? '',
			subject: 'mailto:push@example.com'
		},
		verifyKey: pair.publicKey
	};
}

async function verifies(token: string, verifyKey: CryptoKey): Promise<boolean> {
	const [header, payload, signature] = token.split('.');
	return crypto.subtle.verify(
		{ name: 'ECDSA', hash: 'SHA-256' },
		verifyKey,
		decodeBase64URL(signature),
		new TextEncoder().encode(`${header}.${payload}`)
	);
}

async function aSubscription() {
	const pair = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveBits']);
	const publicRaw = new Uint8Array(await crypto.subtle.exportKey('raw', pair.publicKey));
	const authSecret = crypto.getRandomValues(new Uint8Array(16));
	return {
		keys: { p256dh: encodeBase64URL(publicRaw.buffer), auth: encodeBase64URL(authSecret.buffer) },
		privateKey: pair.privateKey,
		publicRaw,
		authSecret
	};
}

function join(...parts: Uint8Array[]): Uint8Array<ArrayBuffer> {
	const joined = new Uint8Array(parts.reduce((total, part) => total + part.length, 0));
	let written = 0;
	for (const part of parts) {
		joined.set(part, written);
		written += part.length;
	}
	return joined;
}

function labeled(text: string): Uint8Array<ArrayBuffer> {
	return join(new TextEncoder().encode(text), new Uint8Array([0]));
}

async function hkdf(
	material: Uint8Array<ArrayBuffer>,
	salt: Uint8Array<ArrayBuffer>,
	info: Uint8Array<ArrayBuffer>,
	byteLength: number
): Promise<Uint8Array<ArrayBuffer>> {
	const key = await crypto.subtle.importKey('raw', material, 'HKDF', false, ['deriveBits']);
	const bits = await crypto.subtle.deriveBits({ name: 'HKDF', hash: 'SHA-256', salt, info }, key, byteLength * 8);
	return new Uint8Array(bits);
}

async function openSealed(
	sealed: Uint8Array<ArrayBuffer>,
	subscription: Awaited<ReturnType<typeof aSubscription>>
): Promise<string> {
	const salt = sealed.slice(0, 16);
	const senderKeyLength = sealed[20];
	const senderPublic = sealed.slice(21, 21 + senderKeyLength);
	const ciphertext = sealed.slice(21 + senderKeyLength);

	const sender = await crypto.subtle.importKey('raw', senderPublic, { name: 'ECDH', namedCurve: 'P-256' }, false, []);
	const shared = new Uint8Array(
		await crypto.subtle.deriveBits({ name: 'ECDH', public: sender }, subscription.privateKey, 256)
	);
	const keyInfo = join(labeled('WebPush: info'), subscription.publicRaw, senderPublic);
	const pseudoRandomKey = await hkdf(shared, subscription.authSecret, keyInfo, 32);
	const contentKey = await hkdf(pseudoRandomKey, salt, labeled('Content-Encoding: aes128gcm'), 16);
	const nonce = await hkdf(pseudoRandomKey, salt, labeled('Content-Encoding: nonce'), 12);

	const key = await crypto.subtle.importKey('raw', contentKey, { name: 'AES-GCM' }, false, ['decrypt']);
	const padded = new Uint8Array(await crypto.subtle.decrypt({ name: 'AES-GCM', iv: nonce }, key, ciphertext));
	expect(padded[padded.length - 1]).toBe(0x02);
	return new TextDecoder().decode(padded.slice(0, padded.length - 1));
}

describe('the shared web-push copy stays interchangeable with the web one', () => {
	test('both sign VAPID tokens with identical claims that the public key verifies', async () => {
		const { keys, verifyKey } = await aVapidPair();
		const webToken = await webSignVapidToken(endpoint, keys, now);
		const sharedToken = await sharedSignVapidToken(endpoint, keys, now);

		const [webHeader, webPayload] = webToken.split('.');
		const [sharedHeader, sharedPayload] = sharedToken.split('.');
		expect(sharedHeader).toBe(webHeader);
		expect(sharedPayload).toBe(webPayload);
		expect(await verifies(webToken, verifyKey)).toBe(true);
		expect(await verifies(sharedToken, verifyKey)).toBe(true);
	});

	test('both seal payloads one RFC 8291 reference decryptor opens', async () => {
		const subscription = await aSubscription();
		const plaintext = JSON.stringify({ title: '알림', body: '동등성 확인' });

		const webSealed = await webEncrypt(plaintext, subscription.keys);
		const sharedSealed = await sharedEncrypt(plaintext, subscription.keys);

		expect(await openSealed(webSealed, subscription)).toBe(plaintext);
		expect(await openSealed(sharedSealed, subscription)).toBe(plaintext);
	});

	test('both judge subscription keys and push statuses the same way', async () => {
		const subscription = await aSubscription();
		const truncated = { p256dh: subscription.keys.p256dh.slice(0, 10), auth: subscription.keys.auth };

		expect(sharedIsUsable(subscription.keys)).toBe(webIsUsable(subscription.keys));
		expect(sharedIsUsable(truncated)).toBe(webIsUsable(truncated));
		for (const status of [200, 201, 204, 299, 400, 403, 404, 410, 429, 500]) {
			expect(sharedOutcomeOfStatus(status)).toBe(webOutcomeOfStatus(status));
		}
	});
});

describe('a single unreachable device never fails the whole notification', () => {
	const copies: { name: string; send: typeof webSendWebPush }[] = [
		{ name: 'web', send: webSendWebPush },
		{ name: 'shared', send: sharedSendWebPush }
	];
	const notification = { title: '알림', body: '전송 실패 경로' };

	async function withFetch<T>(answer: () => Promise<Response>, body: () => Promise<T>): Promise<T> {
		const original = globalThis.fetch;
		globalThis.fetch = Object.assign(() => answer(), { preconnect: original.preconnect }) as typeof fetch;
		try {
			return await body();
		} finally {
			globalThis.fetch = original;
		}
	}

	function undialled(): Promise<Response> {
		return Promise.reject(new Error('the address should never have been dialled'));
	}

	for (const { name, send } of copies) {
		test(`${name} calls an address no URL parser accepts gone`, async () => {
			const subscription = await aSubscription();
			const { keys: vapid } = await aVapidPair();
			const reached = await withFetch(undialled, () =>
				send({ address: 'push.example.com/no-scheme', keys: subscription.keys }, notification, vapid, now)
			);
			expect(reached).toBe('gone');
		});

		test(`${name} calls keys that are not base64 gone`, async () => {
			const { keys: vapid } = await aVapidPair();
			const reached = await withFetch(undialled, () =>
				send({ address: endpoint, keys: { p256dh: '!!! not base64 !!!', auth: '!!!' } }, notification, vapid, now)
			);
			expect(reached).toBe('gone');
		});

		test(`${name} calls a subscription whose point is off the curve gone`, async () => {
			const subscription = await aSubscription();
			const offCurve = new Uint8Array(subscription.publicRaw);
			offCurve[64] ^= 0xff;
			const { keys: vapid } = await aVapidPair();
			const reached = await withFetch(undialled, () =>
				send(
					{ address: endpoint, keys: { p256dh: encodeBase64URL(offCurve.buffer), auth: subscription.keys.auth } },
					notification,
					vapid,
					now
				)
			);
			expect(reached).toBe('gone');
		});

		test(`${name} calls a transport that never connects refused`, async () => {
			const subscription = await aSubscription();
			const { keys: vapid } = await aVapidPair();
			const reached = await withFetch(
				() => Promise.reject(new TypeError('dns lookup failed')),
				() => send({ address: endpoint, keys: subscription.keys }, notification, vapid, now)
			);
			expect(reached).toBe('refused');
		});

		test(`${name} still delivers over a transport that answers`, async () => {
			const subscription = await aSubscription();
			const { keys: vapid } = await aVapidPair();
			const reached = await withFetch(
				() => Promise.resolve(new Response(null, { status: 201 })),
				() => send({ address: endpoint, keys: subscription.keys }, notification, vapid, now)
			);
			expect(reached).toBe('delivered');
		});
	}
});
