import { encodeBase64URL } from '../../src/lib/notifications/base64url';

export type Subscriber = {
	privateKey: CryptoKey;
	publicKey: Uint8Array<ArrayBuffer>;
	keys: { p256dh: string; auth: string };
	authSecret: Uint8Array<ArrayBuffer>;
};

export async function aBrowserThatSubscribed(): Promise<Subscriber> {
	const pair = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveBits']);
	const publicKey = new Uint8Array(await crypto.subtle.exportKey('raw', pair.publicKey));
	const authSecret = crypto.getRandomValues(new Uint8Array(16));
	return {
		privateKey: pair.privateKey,
		publicKey,
		keys: { p256dh: encodeBase64URL(publicKey.buffer), auth: encodeBase64URL(authSecret.buffer) },
		authSecret
	};
}

export async function readAsTheBrowserWould(
	body: Uint8Array<ArrayBuffer>,
	subscriber: Subscriber
): Promise<string> {
	const salt = body.subarray(0, 16);
	const senderPublicKey = body.subarray(21, 21 + body[20]);
	const sealed = body.subarray(21 + body[20]);

	const sender = await crypto.subtle.importKey('raw', senderPublicKey, { name: 'ECDH', namedCurve: 'P-256' }, false, []);
	const shared = new Uint8Array(
		await crypto.subtle.deriveBits({ name: 'ECDH', public: sender }, subscriber.privateKey, 256)
	);
	const material = await derive(
		shared,
		subscriber.authSecret,
		concatenate(labelled('WebPush: info'), subscriber.publicKey, senderPublicKey),
		32
	);
	const contentKey = await derive(material, salt, labelled('Content-Encoding: aes128gcm'), 16);
	const nonce = await derive(material, salt, labelled('Content-Encoding: nonce'), 12);

	const key = await crypto.subtle.importKey('raw', contentKey, { name: 'AES-GCM' }, false, ['decrypt']);
	const opened = new Uint8Array(await crypto.subtle.decrypt({ name: 'AES-GCM', iv: nonce }, key, sealed));
	return new TextDecoder().decode(opened.subarray(0, opened.length - 1));
}

async function derive(
	material: Uint8Array<ArrayBuffer>,
	salt: Uint8Array<ArrayBuffer>,
	info: Uint8Array<ArrayBuffer>,
	byteLength: number
): Promise<Uint8Array<ArrayBuffer>> {
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
