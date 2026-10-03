import { describe, expect, test } from 'bun:test';
import { serviceFilePath, serviceFileVersionAt } from '../../../src/lib/data-room/service-files';

const companyID = '11111111-1111-4111-8111-111111111111';
const documentID = '22222222-2222-4222-8222-222222222222';

describe('a service file version', () => {
	test('sits in its category under its fixed name, the day it took effect, its document and its extension', () => {
		expect(serviceFilePath(companyID, { name: 'seal', date: '2026-10-04', documentID, extension: 'png' }))
			.toBe(`${companyID}/dataroom/C/CR/seal.2026-10-04.${documentID}.png`);
		expect(serviceFilePath(companyID, { name: 'logo', date: '2026-10-04', documentID, extension: 'webp' }))
			.toBe(`${companyID}/dataroom/S/SM/logo.2026-10-04.${documentID}.webp`);
	});

	test('is read back from the path it was kept at', () => {
		expect(serviceFileVersionAt(companyID, `${companyID}/dataroom/C/CR/seal.2026-10-04.${documentID}.jpg`))
			.toEqual({ name: 'seal', date: '2026-10-04', documentID, extension: 'jpg' });
	});

	test('is not read from a path in another category, another company, or without its day', () => {
		const otherCompany = '33333333-3333-4333-8333-333333333333';
		expect(serviceFileVersionAt(companyID, `${companyID}/dataroom/S/SM/seal.2026-10-04.${documentID}.png`)).toBeUndefined();
		expect(serviceFileVersionAt(companyID, `${otherCompany}/dataroom/C/CR/seal.2026-10-04.${documentID}.png`)).toBeUndefined();
		expect(serviceFileVersionAt(companyID, `${companyID}/dataroom/C/CR/seal.${documentID}.png`)).toBeUndefined();
		expect(serviceFileVersionAt(companyID, `${companyID}/dataroom/C/CR/seal-2026-10-04.${documentID}.png`)).toBeUndefined();
	});
});
