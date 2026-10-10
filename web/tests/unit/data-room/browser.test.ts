import { describe, expect, test } from 'bun:test';
import { dataRoomFolderEntries } from '../../../src/lib/data-room/browser';
import { dataRoomTemplate } from '../../../src/lib/data-room/model';
import { companyDocumentResultSchema } from '../../../src/lib/data-room/schemas';

const document = companyDocumentResultSchema.parse({
	documentID: '63000000-0000-0000-0000-000000000020', documentNumber: null,
	kind: 'document', documentType: 'report', title: 'Sample statement', counterpart: null,
	language: null, filePath: null, summary: null, requesterID: null, createdAt: '2026-10-02T09:00:00Z', updatedAt: '2026-10-03T09:00:00Z',
	categoryCode: 'FS', date: null, period: null, status: null,
	supersedes: null, sha256: null, tags: [], published: null,
	storagePath: 'company/dataroom/F/FS/statement.63000000-0000-0000-0000-000000000020.pdf', sizeBytes: 1536
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
	test('a document row shows its size and when it was created and changed, and a folder shows none', () => {
		const leaf = dataRoomFolderEntries([document], dataRoomTemplate.categories, 'FS', 'en');
		expect(leaf[0].cells[0]).toBe('1.5 KB');
		expect(leaf[0].cells).toHaveLength(3);
		const folders = dataRoomFolderEntries([document], dataRoomTemplate.categories, 'F', 'en');
		expect(folders.every((entry) => entry.cells.length === 0)).toBe(true);
	});
	test('a folder counts the documents filed anywhere under it', () => {
		const rootEntries = dataRoomFolderEntries([document], dataRoomTemplate.categories, '', 'en');
		expect(rootEntries.find((entry) => entry.id === 'F')?.entryCount).toBe(1);
		expect(rootEntries.find((entry) => entry.id === 'C')?.entryCount).toBe(0);
		const financeEntries = dataRoomFolderEntries([document], dataRoomTemplate.categories, 'F', 'en');
		expect(financeEntries.find((entry) => entry.id === 'FS')?.entryCount).toBe(1);
		expect(financeEntries.find((entry) => entry.id === 'FT')?.entryCount).toBe(0);
	});
});
