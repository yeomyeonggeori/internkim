import { describe, expect, test } from 'bun:test';
import {
	documentsInFolder,
	guestFolders,
	matchingDocuments,
	sharedFileName,
	type SharedRoom
} from '../../../src/lib/data-room/guest-room';

const category = (code: string, parent: string | null, name: string, nameKO: string) => ({
	code,
	parent,
	name,
	name_ko: nameKO,
	description: '',
	slug: code.toLowerCase()
});

const document = (id: string, categoryCode: string, title: string, extension: string | null) => ({
	id,
	title,
	summary: null,
	category_code: categoryCode,
	document_date: null,
	status: null,
	extension
});

const room: SharedRoom = {
	categories: [
		category('F', null, 'Finance', '재무'),
		category('FS', 'F', 'Financial statements', '재무제표'),
		category('FT', 'F', 'Tax', '세무'),
		category('H', null, 'People', '인사'),
		category('HR', 'H', 'Hiring', '채용')
	],
	documents: [
		document('audit', 'FS', '2025 감사보고서', 'pdf'),
		document('return', 'FT', '법인세 신고서', null)
	]
};

describe('guestFolders', () => {
	test('lists each parent before its children and leaves out folders holding nothing', () => {
		expect(guestFolders(room, 'ko')).toEqual([
			{ code: 'F', name: '재무', isChild: false, documentCount: 2 },
			{ code: 'FS', name: '재무제표', isChild: true, documentCount: 1 },
			{ code: 'FT', name: '세무', isChild: true, documentCount: 1 }
		]);
	});
});

describe('documentsInFolder', () => {
	test('a parent holds what its children hold, and no folder holds everything', () => {
		expect(documentsInFolder(room, 'F').map((shared) => shared.id)).toEqual(['audit', 'return']);
		expect(documentsInFolder(room, 'FS').map((shared) => shared.id)).toEqual(['audit']);
		expect(documentsInFolder(room, '')).toHaveLength(2);
	});
});

describe('matchingDocuments', () => {
	test('finds a document by a partly typed Korean title', () => {
		expect(matchingDocuments(room.documents, '감사ㅂ').map((shared) => shared.id)).toEqual(['audit']);
	});
});

describe('sharedFileName', () => {
	test('names a document by its title and the original file type, or nothing without a file', () => {
		expect(sharedFileName(room.documents[0])).toBe('2025 감사보고서.pdf');
		expect(sharedFileName(room.documents[1])).toBeNull();
	});
});
