import { describe, expect, test } from 'bun:test';
import { x25519 } from '@noble/curves/ed25519.js';
import { readFileSync } from 'node:fs';
import { sealedModelKeySchema, type SealedModelKey } from '../../../src/lib/company/box';
import {
	base64URLOf,
	bytesOfBase64URL,
	freshSealingMaterial,
	modelKeySealingKey,
	sealModelKey
} from '../../../src/lib/company/seal-model-key';

const vectorPath = new URL('../../../../internal/box/testdata/sealed-model-key.json', import.meta.url);

type SealingVector = {
	modelKey: string;
	boxSecretKey: string;
	boxEncryptionKey: string;
	ephemeralSecretKey: string;
	nonce: string;
	sealed: SealedModelKey;
};

async function openedBy(boxSecretKey: Uint8Array, sealed: SealedModelKey): Promise<string> {
	const ephemeralPublicKey = bytesOfBase64URL(sealed.ephemeralPublicKey);
	const sharedSecret = x25519.getSharedSecret(boxSecretKey, ephemeralPublicKey);
	const openingKey = await modelKeySealingKey(sharedSecret, ephemeralPublicKey, x25519.getPublicKey(boxSecretKey));
	const opened = await crypto.subtle.decrypt(
		{ name: 'AES-GCM', iv: bytesOfBase64URL(sealed.nonce) },
		openingKey,
		bytesOfBase64URL(sealed.ciphertext)
	);
	return new TextDecoder().decode(opened);
}

describe('sealing a model key to a box', () => {
	test('the box opens what the browser sealed to it', async () => {
		const boxSecretKey = x25519.utils.randomSecretKey();
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(boxSecretKey));

		const sealed = await sealModelKey('sk-or-v1-example', boxEncryptionKey, freshSealingMaterial());

		expect(sealedModelKeySchema.safeParse(sealed).success).toBe(true);
		expect(await openedBy(boxSecretKey, sealed)).toBe('sk-or-v1-example');
	});

	test('a key sealed to one box does not open on another', async () => {
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(x25519.utils.randomSecretKey()));
		const sealed = await sealModelKey('sk-or-v1-example', boxEncryptionKey, freshSealingMaterial());

		await expect(openedBy(x25519.utils.randomSecretKey(), sealed)).rejects.toThrow();
	});

	test('the vector the box side opens is exactly what sealing produces', async () => {
		const vector: SealingVector = JSON.parse(readFileSync(vectorPath, 'utf8'));
		const boxSecretKey = bytesOfBase64URL(vector.boxSecretKey);
		expect(base64URLOf(x25519.getPublicKey(boxSecretKey))).toBe(vector.boxEncryptionKey);

		const sealed = await sealModelKey(vector.modelKey, vector.boxEncryptionKey, {
			ephemeralSecretKey: bytesOfBase64URL(vector.ephemeralSecretKey),
			nonce: bytesOfBase64URL(vector.nonce)
		});

		expect(sealed).toEqual(vector.sealed);
		expect(await openedBy(boxSecretKey, vector.sealed)).toBe(vector.modelKey);
	});
});
