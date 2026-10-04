import { describe, expect, test } from 'bun:test';
import { boxStepOf, shortBoxName } from '../../../src/routes/settings/setup/box-step';

const publicKey = 'oYwaGe-VYDGlZRd4ButkPDL65UP6m3BBNTUfLugolZI';
const connected = { companyID: 'company-a', publicKey, encryptionKey: publicKey, lastSeenAt: null, hasModelKey: false, hasAdminAccount: false };
const announced = { publicKey, announcedAt: '2026-09-27T00:00:00.000Z', hostName: null, pairingPageAddresses: [] };

describe('the company computer step', () => {
	test('waits for a box when none is announcing on this network', () => {
		expect(boxStepOf({ connected: null, empty: [] })).toBe('searching');
	});

	test('offers the boxes announcing on this network', () => {
		expect(boxStepOf({ connected: null, empty: [announced] })).toBe('choosing');
	});

	test('asks for the model key once a box is connected', () => {
		expect(boxStepOf({ connected, empty: [announced] })).toBe('givingModelKey');
	});

	test('is done once the connected box has its model key', () => {
		expect(boxStepOf({ connected: { ...connected, hasModelKey: true }, empty: [] })).toBe('connected');
	});
});

test('a box is named by the last four characters of its public key, the ones its own page shows', () => {
	expect(shortBoxName(publicKey)).toBe('…olZI');
});
