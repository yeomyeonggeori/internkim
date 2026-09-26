import { x25519 } from '@noble/curves/ed25519.js';
import type { SealedModelKey } from './box';

export const modelKeySealInformation = 'internkim model key v1';

const nonceLength = 12;

export type SealingMaterial = {
	ephemeralSecretKey: Uint8Array;
	nonce: Uint8Array<ArrayBuffer>;
};

export function base64URLOf(bytes: Uint8Array): string {
	let binary = '';
	for (const byte of bytes) binary += String.fromCharCode(byte);
	return btoa(binary).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '');
}

export function bytesOfBase64URL(encoded: string): Uint8Array<ArrayBuffer> {
	const padded = encoded.replaceAll('-', '+').replaceAll('_', '/').padEnd(Math.ceil(encoded.length / 4) * 4, '=');
	return Uint8Array.from(atob(padded), (character) => character.charCodeAt(0));
}

export function freshSealingMaterial(): SealingMaterial {
	return {
		ephemeralSecretKey: x25519.utils.randomSecretKey(),
		nonce: crypto.getRandomValues(new Uint8Array(nonceLength))
	};
}

export async function modelKeySealingKey(
	sharedSecret: Uint8Array,
	ephemeralPublicKey: Uint8Array,
	boxEncryptionKey: Uint8Array
): Promise<CryptoKey> {
	const secret = await crypto.subtle.importKey('raw', new Uint8Array(sharedSecret), 'HKDF', false, ['deriveKey']);
	return crypto.subtle.deriveKey(
		{
			name: 'HKDF',
			hash: 'SHA-256',
			salt: new Uint8Array([...ephemeralPublicKey, ...boxEncryptionKey]),
			info: new TextEncoder().encode(modelKeySealInformation)
		},
		secret,
		{ name: 'AES-GCM', length: 256 },
		false,
		['encrypt', 'decrypt']
	);
}

export async function sealModelKey(
	modelKey: string,
	boxEncryptionKey: string,
	material: SealingMaterial
): Promise<SealedModelKey> {
	const boxKeyBytes = bytesOfBase64URL(boxEncryptionKey);
	const ephemeralPublicKey = x25519.getPublicKey(material.ephemeralSecretKey);
	const sharedSecret = x25519.getSharedSecret(material.ephemeralSecretKey, boxKeyBytes);
	const sealingKey = await modelKeySealingKey(sharedSecret, ephemeralPublicKey, boxKeyBytes);
	const ciphertext = await crypto.subtle.encrypt(
		{ name: 'AES-GCM', iv: material.nonce },
		sealingKey,
		new TextEncoder().encode(modelKey)
	);
	return {
		ephemeralPublicKey: base64URLOf(ephemeralPublicKey),
		nonce: base64URLOf(material.nonce),
		ciphertext: base64URLOf(new Uint8Array(ciphertext))
	};
}
