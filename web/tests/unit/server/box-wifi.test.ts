import { describe, expect, test } from 'bun:test';
import type { BoxRow } from '$lib/server/box';
import { nearbyNetworksDiffer, pendingWifiChangeOf, wifiChangeFetchSchema, wifiChangeRequestSchema, wifiOutcomeReportSchema } from '$lib/server/box-wifi';

const sealed = {
	version: 1 as const,
	recipient: 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM',
	enc: 'e06Qm75__kTEZaIgA31gjuNYl9Me-XLwf3SJLLD3PxM',
	ciphertext: 'c2VhbGVk'
};

const requestID = '00000000-0000-4000-8000-0000000000f1';

function aBox(wifiChange?: { requestID: string; sealed: typeof sealed; requestedAt: string }): BoxRow {
	return {
		companyID: 'company-a',
		publicKey: sealed.recipient,
		settings: { encryptionKey: sealed.recipient, ...(wifiChange ? { wifiChange } : {}) }
	};
}

describe('the Wi-Fi change request a browser sends', () => {
	test('is accepted with a request id and a sealed network', () => {
		expect(wifiChangeRequestSchema.safeParse({ requestID, sealed }).success).toBe(true);
	});

	test('is refused without a request id, with an invalid one, or with anything extra', () => {
		expect(wifiChangeRequestSchema.safeParse({ sealed }).success).toBe(false);
		expect(wifiChangeRequestSchema.safeParse({ requestID: 'not-a-uuid', sealed }).success).toBe(false);
		expect(wifiChangeRequestSchema.safeParse({ requestID, sealed, extra: true }).success).toBe(false);
	});
});

describe('the Wi-Fi outcome a box reports', () => {
	test('is accepted with a request id and a result of joined or failed', () => {
		expect(wifiOutcomeReportSchema.safeParse({ requestID, result: 'joined' }).success).toBe(true);
		expect(wifiOutcomeReportSchema.safeParse({ requestID, result: 'failed' }).success).toBe(true);
	});

	test('is refused for a result the runtime does not know', () => {
		expect(wifiOutcomeReportSchema.safeParse({ requestID, result: 'connected' }).success).toBe(false);
	});
});

describe('the Wi-Fi change pending for a box', () => {
	test('is named when the box holds one', () => {
		const requestedAt = '2026-09-08T09:00:00.000Z';
		expect(pendingWifiChangeOf(aBox({ requestID, sealed, requestedAt }))).toEqual({ requestID, sealed });
	});

	test('is null when the box holds none', () => {
		expect(pendingWifiChangeOf(aBox())).toBeNull();
	});
});

describe('the nearby networks a box reports while fetching a Wi-Fi change', () => {
	const network = { ssid: 'Sample Office', signalPercent: 72, isSecured: true };

	test('is accepted with up to fifty networks, or with nothing at all', () => {
		expect(wifiChangeFetchSchema.safeParse({ nearbyNetworks: [network] }).success).toBe(true);
		expect(wifiChangeFetchSchema.safeParse({ nearbyNetworks: Array.from({ length: 50 }, (_, index) => ({ ...network, ssid: `Sample Office ${index}` })) }).success).toBe(true);
		expect(wifiChangeFetchSchema.safeParse({}).success).toBe(true);
	});

	test('is refused with more than fifty networks', () => {
		expect(wifiChangeFetchSchema.safeParse({ nearbyNetworks: Array.from({ length: 51 }, (_, index) => ({ ...network, ssid: `Sample Office ${index}` })) }).success).toBe(false);
	});

	test('is refused for a name over 32 characters, an empty name, or a signal outside 0 to 100', () => {
		expect(wifiChangeFetchSchema.safeParse({ nearbyNetworks: [{ ...network, ssid: 'a'.repeat(33) }] }).success).toBe(false);
		expect(wifiChangeFetchSchema.safeParse({ nearbyNetworks: [{ ...network, ssid: '' }] }).success).toBe(false);
		expect(wifiChangeFetchSchema.safeParse({ nearbyNetworks: [{ ...network, signalPercent: 101 }] }).success).toBe(false);
	});

	test('is refused with extra keys on the body or on a network', () => {
		expect(wifiChangeFetchSchema.safeParse({ nearbyNetworks: [network], extra: true }).success).toBe(false);
		expect(wifiChangeFetchSchema.safeParse({ nearbyNetworks: [{ ...network, bssid: 'x' }] }).success).toBe(false);
	});
});

describe('whether the nearby networks a box reports differ from the stored ones', () => {
	const office = { ssid: 'Sample Office', signalPercent: 72, isSecured: true };
	const guest = { ssid: 'Sample Guest', signalPercent: 40, isSecured: false };

	test('is false when only signal strength or order changed', () => {
		expect(nearbyNetworksDiffer([office, guest], [{ ...guest, signalPercent: 90 }, { ...office, signalPercent: 10 }])).toBe(false);
	});

	test('treats a missing connected flag as not connected', () => {
		expect(nearbyNetworksDiffer([office], [{ ...office, isConnected: false }])).toBe(false);
	});

	test('is true when a network appears, disappears, is secured differently or becomes the connected one', () => {
		expect(nearbyNetworksDiffer([office], [office, guest])).toBe(true);
		expect(nearbyNetworksDiffer([office, guest], [office])).toBe(true);
		expect(nearbyNetworksDiffer([office], [{ ...office, isSecured: false }])).toBe(true);
		expect(nearbyNetworksDiffer([office], [{ ...office, isConnected: true }])).toBe(true);
	});

	test('is false for two empty lists and true when the first scan finds none after networks were stored', () => {
		expect(nearbyNetworksDiffer([], [])).toBe(false);
		expect(nearbyNetworksDiffer([office], [])).toBe(true);
	});
});
