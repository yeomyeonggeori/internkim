import { describe, expect, test } from 'bun:test';
import { boxStepOf, shortBoxName } from '../../../src/routes/settings/setup/box-step';

const publicKey = 'oYwaGe-VYDGlZRd4ButkPDL65UP6m3BBNTUfLugolZI';
const connected = {
	companyID: 'company-a',
	publicKey,
	encryptionKey: publicKey,
	role: 'service' as const,
	hostName: null,
	site: '',
	lastSeenAt: null,
	holdsCompanyKey: true,
	hasAdminAccount: false
};
const announced = { publicKey, announcedAt: '2026-09-27T00:00:00.000Z', hostName: null, pairingPageAddresses: [] };
const noBoxes = { boxes: [], sealingKey: null, hasModelKey: false, newestBackupAt: null, takeOver: { isAutomatic: false, lapseSeconds: 300 } };

describe('the company computer step', () => {
	test('waits for a box when none is announcing on this network', () => {
		expect(boxStepOf({ ...noBoxes, empty: [] })).toBe('searching');
	});

	test('offers the boxes announcing on this network', () => {
		expect(boxStepOf({ ...noBoxes, empty: [announced] })).toBe('choosing');
	});

	test('asks for the model key once a box is connected', () => {
		expect(boxStepOf({ boxes: [connected], sealingKey: publicKey, hasModelKey: false, newestBackupAt: null, takeOver: { isAutomatic: false, lapseSeconds: 300 }, empty: [announced] })).toBe('givingModelKey');
	});

	test('is done once the company has its model key, however many boxes it runs on', () => {
		const standby = { ...connected, publicKey: 'p'.repeat(43), role: 'standby' as const, holdsCompanyKey: false };
		expect(boxStepOf({ boxes: [connected, standby], sealingKey: publicKey, hasModelKey: true, newestBackupAt: null, takeOver: { isAutomatic: false, lapseSeconds: 300 }, empty: [] })).toBe('connected');
	});
});

test('a box is named by the last four characters of its public key, the ones its own page shows', () => {
	expect(shortBoxName(publicKey)).toBe('…olZI');
});
