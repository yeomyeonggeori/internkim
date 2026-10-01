import { describe, expect, test } from 'bun:test';
import { x25519 } from '@noble/curves/ed25519.js';
import { readFileSync } from 'node:fs';
import { sealedSecretSchema, type SealedSecret } from '../../../src/lib/company/box';
import {
	base64URLOf,
	boxSealingSuite,
	bytesOfBase64URL,
	modelKeySealInformation,
	sealModelKey
} from '../../../src/lib/company/seal-to-box';
import { openedBy } from './open-from-box';

const modelKeyPurpose = { information: modelKeySealInformation, additionalData: '' };

type RFC9180Vector = { info: string; ikmE: string; skRm: string; pkRm: string; enc: string; aad: string; pt: string; ct: string };
type ModelKeyFixture = { modelKey: string; boxSecretKey: string; sealed: SealedSecret };

function testdata(name: string): string {
	return readFileSync(new URL(`../../../../internal/box/testdata/${name}`, import.meta.url), 'utf8');
}

function hexBytes(encoded: string): Uint8Array<ArrayBuffer> {
	return Uint8Array.from(encoded.match(/../g) ?? [], (pair) => parseInt(pair, 16));
}

function hexOf(bytes: ArrayBuffer): string {
	return [...new Uint8Array(bytes)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

describe('the suite the web app seals with', () => {
	test('reproduces the RFC 9180 vector for DHKEM(X25519, HKDF-SHA256), HKDF-SHA256, AES-256-GCM', async () => {
		const vector: RFC9180Vector = JSON.parse(testdata('hpke-rfc9180-base-x25519-sha256-aes256gcm.json'));

		const sealed = await boxSealingSuite.seal(
			{
				recipientPublicKey: await boxSealingSuite.kem.deserializePublicKey(hexBytes(vector.pkRm)),
				info: hexBytes(vector.info),
				ekm: hexBytes(vector.ikmE)
			},
			hexBytes(vector.pt),
			hexBytes(vector.aad)
		);

		expect(hexOf(sealed.enc)).toBe(vector.enc);
		expect(hexOf(sealed.ct)).toBe(vector.ct);
	});
});

describe('sealing a model key to a box', () => {
	test('the box opens what the browser sealed to it', async () => {
		const boxSecretKey = x25519.utils.randomSecretKey();
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(boxSecretKey));

		const sealed = await sealModelKey('sk-or-v1-example', boxEncryptionKey);

		expect(sealedSecretSchema.safeParse(sealed).success).toBe(true);
		expect(await openedBy(boxSecretKey, sealed, modelKeyPurpose)).toBe('sk-or-v1-example');
	});

	test('a key sealed to one box does not open on another', async () => {
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(x25519.utils.randomSecretKey()));
		const sealed = await sealModelKey('sk-or-v1-example', boxEncryptionKey);

		await expect(openedBy(x25519.utils.randomSecretKey(), sealed, modelKeyPurpose)).rejects.toThrow();
	});

	test('the model key the box side opens in its tests opens here for the same purpose', async () => {
		const fixture: ModelKeyFixture = JSON.parse(testdata('sealed-model-key.json'));

		expect(await openedBy(bytesOfBase64URL(fixture.boxSecretKey), fixture.sealed, modelKeyPurpose)).toBe(fixture.modelKey);
	});
});
