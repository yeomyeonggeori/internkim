import { describe, expect, test } from 'bun:test';
import { x25519 } from '@noble/curves/ed25519.js';
import { readFileSync } from 'node:fs';
import {
	sealedSecretSchema,
	wifiChangeStatusSchema,
	wifiOutcomeResultSchema,
	type SealedSecret
} from '../../../src/lib/company/box';
import {
	adminPasswordPurpose,
	base64URLOf,
	boxSealingSuite,
	bytesOfBase64URL,
	modelKeyPurpose,
	sealAdminPassword,
	sealModelKey,
	sealWifiNetwork,
	wifiNetworkPurpose
} from '../../../src/lib/company/seal-to-box';
import { openedBy } from './open-from-box';

const companyID = '00000000-0000-4000-8000-00000000000a';

type RFC9180Vector = { info: string; ikmE: string; skRm: string; pkRm: string; enc: string; aad: string; pt: string; ct: string };
type ModelKeyFixture = { modelKey: string; boxSecretKey: string; companyID: string; sealed: SealedSecret };
type AdminPasswordFixture = { password: string; boxSecretKey: string; companyID: string; settingID: string; sealed: SealedSecret };
type WifiNetworkFixture = {
	ssid: string;
	password: string;
	boxSecretKey: string;
	companyID: string;
	requestID: string;
	sealed: SealedSecret;
};

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

		const sealed = await sealModelKey('sk-or-v1-example', { companyID, encryptionKey: boxEncryptionKey });

		expect(sealedSecretSchema.safeParse(sealed).success).toBe(true);
		expect(await openedBy(boxSecretKey, sealed, modelKeyPurpose(companyID, boxEncryptionKey))).toBe('sk-or-v1-example');
	});

	test('a key sealed for one company does not open for another', async () => {
		const boxSecretKey = x25519.utils.randomSecretKey();
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(boxSecretKey));
		const sealed = await sealModelKey('sk-or-v1-example', { companyID, encryptionKey: boxEncryptionKey });

		await expect(
			openedBy(boxSecretKey, sealed, modelKeyPurpose('00000000-0000-4000-8000-00000000000c', boxEncryptionKey))
		).rejects.toThrow();
	});

	test('a key sealed to one box does not open on another', async () => {
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(x25519.utils.randomSecretKey()));
		const sealed = await sealModelKey('sk-or-v1-example', { companyID, encryptionKey: boxEncryptionKey });

		await expect(
			openedBy(x25519.utils.randomSecretKey(), sealed, modelKeyPurpose(companyID, boxEncryptionKey))
		).rejects.toThrow();
	});

	test('the model key the box side opens in its tests opens here for the same purpose', async () => {
		const fixture: ModelKeyFixture = JSON.parse(testdata('sealed-model-key.json'));

		const purpose = modelKeyPurpose(fixture.companyID, fixture.sealed.recipient);

		expect(await openedBy(bytesOfBase64URL(fixture.boxSecretKey), fixture.sealed, purpose)).toBe(fixture.modelKey);
	});
});

describe('sealing a Wi-Fi network to a box', () => {
	const requestID = '00000000-0000-4000-8000-0000000000f1';

	test('the box opens what the browser sealed to it', async () => {
		const boxSecretKey = x25519.utils.randomSecretKey();
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(boxSecretKey));
		const network = { ssid: 'InternKim Office', password: 'correct-horse-battery-staple' };

		const sealed = await sealWifiNetwork(network, { companyID, encryptionKey: boxEncryptionKey }, requestID);

		expect(sealedSecretSchema.safeParse(sealed).success).toBe(true);
		const opened = await openedBy(boxSecretKey, sealed, wifiNetworkPurpose(companyID, boxEncryptionKey, requestID));
		expect(JSON.parse(opened)).toEqual(network);
	});

	test('a network sealed for one request does not open for another', async () => {
		const boxSecretKey = x25519.utils.randomSecretKey();
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(boxSecretKey));
		const network = { ssid: 'InternKim Office', password: 'correct-horse-battery-staple' };
		const sealed = await sealWifiNetwork(network, { companyID, encryptionKey: boxEncryptionKey }, requestID);

		await expect(
			openedBy(boxSecretKey, sealed, wifiNetworkPurpose(companyID, boxEncryptionKey, '00000000-0000-4000-8000-0000000000f2'))
		).rejects.toThrow();
	});

	test('a network sealed to one box does not open on another', async () => {
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(x25519.utils.randomSecretKey()));
		const network = { ssid: 'InternKim Office', password: 'correct-horse-battery-staple' };
		const sealed = await sealWifiNetwork(network, { companyID, encryptionKey: boxEncryptionKey }, requestID);

		await expect(
			openedBy(x25519.utils.randomSecretKey(), sealed, wifiNetworkPurpose(companyID, boxEncryptionKey, requestID))
		).rejects.toThrow();
	});

	test('the Wi-Fi network the box side opens in its tests opens here for the same purpose', async () => {
		const fixture: WifiNetworkFixture = JSON.parse(testdata('sealed-wifi-network.json'));

		const purpose = wifiNetworkPurpose(fixture.companyID, fixture.sealed.recipient, fixture.requestID);

		const opened = await openedBy(bytesOfBase64URL(fixture.boxSecretKey), fixture.sealed, purpose);
		expect(JSON.parse(opened)).toEqual({ ssid: fixture.ssid, password: fixture.password });
	});
});

describe('sealing an admin password to a box', () => {
	const settingID = '00000000-0000-4000-8000-0000000000a1';

	test('the box opens what the browser sealed to it', async () => {
		const boxSecretKey = x25519.utils.randomSecretKey();
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(boxSecretKey));

		const sealed = await sealAdminPassword('correct horse battery', { companyID, encryptionKey: boxEncryptionKey }, settingID);

		expect(sealedSecretSchema.safeParse(sealed).success).toBe(true);
		expect(await openedBy(boxSecretKey, sealed, adminPasswordPurpose(companyID, boxEncryptionKey, settingID))).toBe('correct horse battery');
	});

	test('a password sealed for one setting does not open for another', async () => {
		const boxSecretKey = x25519.utils.randomSecretKey();
		const boxEncryptionKey = base64URLOf(x25519.getPublicKey(boxSecretKey));
		const sealed = await sealAdminPassword('correct horse battery', { companyID, encryptionKey: boxEncryptionKey }, settingID);

		await expect(
			openedBy(boxSecretKey, sealed, adminPasswordPurpose(companyID, boxEncryptionKey, '00000000-0000-4000-8000-0000000000a2'))
		).rejects.toThrow();
	});

	test('the admin password the box side opens in its tests opens here for the same purpose', async () => {
		const fixture: AdminPasswordFixture = JSON.parse(testdata('sealed-admin-password.json'));

		const purpose = adminPasswordPurpose(fixture.companyID, fixture.sealed.recipient, fixture.settingID);

		expect(await openedBy(bytesOfBase64URL(fixture.boxSecretKey), fixture.sealed, purpose)).toBe(fixture.password);
	});
});

describe('the status schemas a Wi-Fi change is reported with', () => {
	test('accepts the three outcome results a box reports', () => {
		expect(wifiOutcomeResultSchema.safeParse('joined').success).toBe(true);
		expect(wifiOutcomeResultSchema.safeParse('failed').success).toBe(true);
		expect(wifiOutcomeResultSchema.safeParse('connected').success).toBe(false);
	});

	test('a status with no pending change and no outcome is valid', () => {
		expect(wifiChangeStatusSchema.safeParse({ pendingRequestID: null, outcome: null, nearbyNetworks: [], scannedAt: null }).success).toBe(true);
	});

	test('a status carries the reported outcome by request id', () => {
		const status = {
			pendingRequestID: null,
			outcome: { requestID: '00000000-0000-4000-8000-0000000000f1', result: 'joined', reportedAt: '2026-09-08T09:00:00.000Z' },
			nearbyNetworks: [{ ssid: 'Sample Office', signalPercent: 70, isSecured: true }],
			scannedAt: '2026-09-08T08:59:00.000Z'
		};
		expect(wifiChangeStatusSchema.safeParse(status).success).toBe(true);
	});
});
