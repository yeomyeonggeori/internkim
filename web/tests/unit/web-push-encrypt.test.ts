import { describe, expect, test } from 'bun:test';
import { encryptForSubscription } from '../../src/lib/server/web-push-encrypt';
import { decodeBase64URL, encodeBase64URL } from '../../src/lib/notifications/base64url';

type Recipient = {
	keys: { p256dh: string; auth: string };
	privateKey: CryptoKey;
	publicKey: Uint8Array<ArrayBuffer>;
};

async function aBrowserThatSubscribed(): Promise<Recipient> {
	const pair = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveBits']);
	const publicKey = new Uint8Array(await crypto.subtle.exportKey('raw', pair.publicKey));
	const authSecret = crypto.getRandomValues(new Uint8Array(16));
	return {
		keys: { p256dh: encodeBase64URL(publicKey.buffer), auth: encodeBase64URL(authSecret.buffer) },
		privateKey: pair.privateKey,
		publicKey
	};
}

async function decryptAsTheBrowserWould(body: Uint8Array<ArrayBuffer>, recipient: Recipient): Promise<string> {
	const salt = body.subarray(0, 16);
	const senderKeyLength = body[20];
	const senderPublicKey = body.subarray(21, 21 + senderKeyLength);
	const sealed = body.subarray(21 + senderKeyLength);

	const sender = await crypto.subtle.importKey(
		'raw',
		senderPublicKey,
		{ name: 'ECDH', namedCurve: 'P-256' },
		false,
		[]
	);
	const shared = new Uint8Array(
		await crypto.subtle.deriveBits({ name: 'ECDH', public: sender }, recipient.privateKey, 256)
	);

	const authSecret = decodeBase64URL(recipient.keys.auth);
	const keyInfo = concatenate(
		new TextEncoder().encode('WebPush: info'),
		new Uint8Array([0]),
		recipient.publicKey,
		senderPublicKey
	);
	const material = await derive(shared, authSecret, keyInfo, 32);
	const contentKey = await derive(material, salt, labelled('Content-Encoding: aes128gcm'), 16);
	const nonce = await derive(material, salt, labelled('Content-Encoding: nonce'), 12);

	const key = await crypto.subtle.importKey('raw', contentKey, { name: 'AES-GCM' }, false, ['decrypt']);
	const opened = new Uint8Array(await crypto.subtle.decrypt({ name: 'AES-GCM', iv: nonce }, key, sealed));
	return new TextDecoder().decode(opened.subarray(0, opened.length - 1));
}

async function derive(material: Uint8Array<ArrayBuffer>, salt: Uint8Array<ArrayBuffer>, info: Uint8Array<ArrayBuffer>, byteLength: number) {
	const key = await crypto.subtle.importKey('raw', material, 'HKDF', false, ['deriveBits']);
	return new Uint8Array(
		await crypto.subtle.deriveBits({ name: 'HKDF', hash: 'SHA-256', salt, info }, key, byteLength * 8)
	);
}

function labelled(text: string): Uint8Array<ArrayBuffer> {
	return concatenate(new TextEncoder().encode(text), new Uint8Array([0]));
}

function concatenate(...parts: Uint8Array<ArrayBuffer>[]): Uint8Array<ArrayBuffer> {
	const joined = new Uint8Array(parts.reduce((total, part) => total + part.length, 0));
	let written = 0;
	for (const part of parts) {
		joined.set(part, written);
		written += part.length;
	}
	return joined;
}

describe('encryptForSubscription', () => {
	test('the browser that subscribed can read what we sent it', async () => {
		const recipient = await aBrowserThatSubscribed();
		const notification = JSON.stringify({ title: '이샘플', body: '오늘 회의 30분 미뤄도 될까요' });

		const sealed = await encryptForSubscription(notification, recipient.keys);

		expect(await decryptAsTheBrowserWould(sealed, recipient)).toBe(notification);
	});

	test('a browser that did not subscribe cannot', async () => {
		const recipient = await aBrowserThatSubscribed();
		const eavesdropper = await aBrowserThatSubscribed();

		const sealed = await encryptForSubscription('a private thing', recipient.keys);

		await expect(decryptAsTheBrowserWould(sealed, eavesdropper)).rejects.toThrow();
	});

	test('the header says how to read the rest of it', async () => {
		const recipient = await aBrowserThatSubscribed();

		const sealed = await encryptForSubscription('x', recipient.keys);

		expect(new DataView(sealed.buffer).getUint32(16)).toBe(4096);
		expect(sealed[20]).toBe(65);
		expect(sealed.length > 21 + 65).toBe(true);
	});

	test('two sends of the same words look nothing alike', async () => {
		const recipient = await aBrowserThatSubscribed();

		const first = await encryptForSubscription('same words', recipient.keys);
		const second = await encryptForSubscription('same words', recipient.keys);

		expect(encodeBase64URL(first.buffer)).not.toBe(encodeBase64URL(second.buffer));
	});
});
