import { decodeBase64URL } from '$lib/notifications/base64url';

export type SubscriptionKeys = {
	p256dh: string;
	auth: string;
};

const recordSize = 4096;
const publicKeyLength = 65;
const lastRecord = 0x02;
const authSecretLength = 16;

export function isUsableSubscriptionKey(keys: SubscriptionKeys): boolean {
	const point = decodeBase64URL(keys.p256dh);
	if (point.length !== publicKeyLength || point[0] !== 4) return false;
	return decodeBase64URL(keys.auth).length === authSecretLength;
}

export async function encryptForSubscription(
	plaintext: string,
	keys: SubscriptionKeys
): Promise<Uint8Array<ArrayBuffer>> {
	const clientPublicKey = decodeBase64URL(keys.p256dh);
	const authSecret = decodeBase64URL(keys.auth);

	const sender = await crypto.subtle.generateKey({ name: 'ECDH', namedCurve: 'P-256' }, true, ['deriveBits']);
	const senderPublicKey = new Uint8Array(await crypto.subtle.exportKey('raw', sender.publicKey));
	const shared = await sharedSecret(sender.privateKey, clientPublicKey);

	const pseudoRandomKey = await derive(shared, authSecret, keyInfo(clientPublicKey, senderPublicKey), 32);
	const salt = crypto.getRandomValues(new Uint8Array(16));
	const contentKey = await derive(pseudoRandomKey, salt, label('Content-Encoding: aes128gcm'), 16);
	const nonce = await derive(pseudoRandomKey, salt, label('Content-Encoding: nonce'), 12);

	const sealed = await seal(contentKey, nonce, plaintext);
	return assemble(salt, senderPublicKey, sealed);
}

async function sharedSecret(senderPrivateKey: CryptoKey, clientPublicKey: Uint8Array<ArrayBuffer>): Promise<Uint8Array<ArrayBuffer>> {
	const client = await crypto.subtle.importKey('raw', clientPublicKey, { name: 'ECDH', namedCurve: 'P-256' }, false, []);
	const bits = await crypto.subtle.deriveBits({ name: 'ECDH', public: client }, senderPrivateKey, 256);
	return new Uint8Array(bits);
}

async function derive(
	material: Uint8Array<ArrayBuffer>,
	salt: Uint8Array<ArrayBuffer>,
	info: Uint8Array<ArrayBuffer>,
	byteLength: number
): Promise<Uint8Array<ArrayBuffer>> {
	const key = await crypto.subtle.importKey('raw', material, 'HKDF', false, ['deriveBits']);
	const bits = await crypto.subtle.deriveBits({ name: 'HKDF', hash: 'SHA-256', salt, info }, key, byteLength * 8);
	return new Uint8Array(bits);
}

function label(text: string): Uint8Array<ArrayBuffer> {
	return join(new TextEncoder().encode(text), new Uint8Array([0]));
}

function keyInfo(clientPublicKey: Uint8Array<ArrayBuffer>, senderPublicKey: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer> {
	return join(label('WebPush: info'), clientPublicKey, senderPublicKey);
}

async function seal(contentKey: Uint8Array<ArrayBuffer>, nonce: Uint8Array<ArrayBuffer>, plaintext: string): Promise<Uint8Array<ArrayBuffer>> {
	const key = await crypto.subtle.importKey('raw', contentKey, { name: 'AES-GCM' }, false, ['encrypt']);
	const padded = join(new TextEncoder().encode(plaintext), new Uint8Array([lastRecord]));
	return new Uint8Array(await crypto.subtle.encrypt({ name: 'AES-GCM', iv: nonce }, key, padded));
}

function assemble(salt: Uint8Array<ArrayBuffer>, senderPublicKey: Uint8Array<ArrayBuffer>, sealed: Uint8Array<ArrayBuffer>): Uint8Array<ArrayBuffer> {
	const header = new Uint8Array(5);
	new DataView(header.buffer).setUint32(0, recordSize);
	header[4] = publicKeyLength;
	return join(salt, header, senderPublicKey, sealed);
}

function join(...parts: Uint8Array<ArrayBuffer>[]): Uint8Array<ArrayBuffer> {
	const joined = new Uint8Array(parts.reduce((total, part) => total + part.length, 0));
	let written = 0;
	for (const part of parts) {
		joined.set(part, written);
		written += part.length;
	}
	return joined;
}
