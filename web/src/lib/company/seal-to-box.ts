import { Aes256Gcm, CipherSuite, HkdfSha256 } from '@hpke/core';
import { DhkemX25519HkdfSha256 } from '@hpke/dhkem-x25519';
import { sealedSecretVersion, type SealedSecret } from './box';

export const modelKeySealInformation = 'internkim model key';

export const boxSealingSuite = new CipherSuite({
	kem: new DhkemX25519HkdfSha256(),
	kdf: new HkdfSha256(),
	aead: new Aes256Gcm()
});

export type SealPurpose = {
	information: string;
	additionalData: string;
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

export async function sealToBox(plaintext: string, boxEncryptionKey: string, purpose: SealPurpose): Promise<SealedSecret> {
	const recipientPublicKey = await boxSealingSuite.kem.deserializePublicKey(bytesOfBase64URL(boxEncryptionKey));
	const sealed = await boxSealingSuite.seal(
		{ recipientPublicKey, info: new TextEncoder().encode(purpose.information) },
		new TextEncoder().encode(plaintext),
		new TextEncoder().encode(purpose.additionalData)
	);
	return {
		version: sealedSecretVersion,
		recipient: boxEncryptionKey,
		enc: base64URLOf(new Uint8Array(sealed.enc)),
		ciphertext: base64URLOf(new Uint8Array(sealed.ct))
	};
}

export function sealModelKey(modelKey: string, boxEncryptionKey: string): Promise<SealedSecret> {
	return sealToBox(modelKey, boxEncryptionKey, { information: modelKeySealInformation, additionalData: '' });
}
