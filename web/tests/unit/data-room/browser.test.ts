import { describe, expect, test } from 'bun:test';
import { dataRoomFolderEntries } from '../../../src/lib/data-room/browser';
import { dataRoomTemplate } from '../../../src/lib/data-room/model';
import { companyDocumentResultSchema } from '../../../src/lib/data-room/schemas';

const document = companyDocumentResultSchema.parse({
	documentID: '63000000-0000-0000-0000-000000000020', documentNumber: null,
	kind: 'document', documentType: 'report', title: 'Sample statement', counterpart: null,
	language: null, filePath: null, summary: null, requesterID: null, issuedAt: '2026-10-02',
	categoryCode: 'FS', clearance: 0, domain: null, date: null, period: null, status: null,
	supersedes: null, sha256: null, tags: [], published: null,
	storagePath: 'company/dataroom/F/FS/statement.63000000-0000-0000-0000-000000000020.pdf'
});

describe('data room folder browser', () => {
	test('the root lists parent folders without flattening the archive', () => {
		const entries = dataRoomFolderEntries([document], dataRoomTemplate.categories, '', 'en');
		expect(entries.every((entry) => entry.isDirectory)).toBe(true);
		expect(entries.map((entry) => entry.id)).toContain('F');
		expect(entries.map((entry) => entry.id)).not.toContain('FS');
	});
	test('a parent lists its intermediate folders', () => {
		const entries = dataRoomFolderEntries([document], dataRoomTemplate.categories, 'F', 'en');
		expect(entries.map((entry) => entry.id)).toContain('FS');
		expect(entries.every((entry) => entry.isDirectory)).toBe(true);
	});
	test('a leaf lists its actual stored file name', () => {
		const entries = dataRoomFolderEntries([document], dataRoomTemplate.categories, 'FS', 'en');
		expect(entries.map((entry) => entry.name)).toEqual(['statement.63000000-0000-0000-0000-000000000020.pdf']);
		expect(entries[0].isDirectory).toBeUndefined();
	});
});
