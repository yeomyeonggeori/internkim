import { describe, expect, test } from 'bun:test';
import { dataRoomFolder, derivedPath, originalPath } from '../../../src/lib/data-room/storage-path';

const companyID = '11111111-1111-4111-8111-111111111111';
const documentID = '22222222-2222-4222-8222-222222222222';

describe('a data room file path', () => {
	test('sits under its parent letter and then its own code', () => {
		expect(dataRoomFolder(companyID, 'FS')).toBe(`${companyID}/dataroom/F/FS`);
	});

	test('sits directly under a category that has no children', () => {
		expect(dataRoomFolder(companyID, 'X')).toBe(`${companyID}/dataroom/X`);
	});

	test('names the original by its file name, its document and its extension', () => {
		const path = originalPath({ companyID, categoryCode: 'FS', documentID }, '2025 Audit Report.PDF', []);
		expect(path).toBe(`${companyID}/dataroom/F/FS/2025-audit-report.${documentID}.pdf`);
	});

	test('falls back to a name the storage key can hold when the file name has none', () => {
		const path = originalPath({ companyID, categoryCode: 'FS', documentID }, '감사보고서.pdf', ['감사보고서', 'financial-statement']);
		expect(path).toBe(`${companyID}/dataroom/F/FS/financial-statement.${documentID}.pdf`);
	});

	test('keeps a file without an extension as bin', () => {
		const path = originalPath({ companyID, categoryCode: 'X', documentID }, 'scan', []);
		expect(path).toBe(`${companyID}/dataroom/X/scan.${documentID}.bin`);
	});

	test('puts a derived file beside the original under the same document', () => {
		const original = `${companyID}/dataroom/F/FS/audit.${documentID}.pdf`;
		expect(derivedPath(original, documentID, 'content.txt')).toBe(`${companyID}/dataroom/F/FS/audit.${documentID}.content.txt`);
	});
});
