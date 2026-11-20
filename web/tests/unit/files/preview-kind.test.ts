import { describe, expect, test } from 'bun:test';
import {
	boundedGrid,
	cardAspectRatioOf,
	cardPreviewOf,
	columnLabel,
	isTooLargeToPreview,
	maxPreviewColumns,
	maxPreviewRows,
	previewKindOf
} from '../../../src/lib/files/preview/kind';

describe('previewKindOf', () => {
	test('reads the kind from the extension regardless of case', () => {
		expect(previewKindOf('audit.PDF')).toBe('pdf');
		expect(previewKindOf('logo.png')).toBe('image');
		expect(previewKindOf('cap-table.xlsx')).toBe('spreadsheet');
		expect(previewKindOf('ledger.csv')).toBe('spreadsheet');
		expect(previewKindOf('content.txt')).toBe('text');
		expect(previewKindOf('README.md')).toBe('markdown');
		expect(previewKindOf('config.json')).toBe('code');
		expect(previewKindOf('server.go')).toBe('code');
		expect(previewKindOf('ir-deck.pptx')).toBe('presentation');
		expect(previewKindOf('contract.docx')).toBe('word');
		expect(previewKindOf('report.HTM')).toBe('html');
	});

	test('names no preview for a format it cannot draw', () => {
		expect(previewKindOf('contract.doc')).toBe('none');
		expect(previewKindOf('contract.hwp')).toBe('none');
		expect(previewKindOf('legacy-deck.ppt')).toBe('none');
		expect(previewKindOf('README')).toBe('none');
		expect(previewKindOf('.gitignore')).toBe('none');
	});
});

describe('isTooLargeToPreview', () => {
	test('holds text to a smaller ceiling than a document drawn from bytes', () => {
		const threeMegabytes = 3 * 1024 * 1024;
		expect(isTooLargeToPreview('notes.md', threeMegabytes)).toBeTrue();
		expect(isTooLargeToPreview('report.pdf', threeMegabytes)).toBeFalse();
		expect(isTooLargeToPreview('report.pdf', 60 * 1024 * 1024)).toBeTrue();
	});
});

describe('columnLabel', () => {
	test('counts columns the way a spreadsheet does', () => {
		expect(columnLabel(0)).toBe('A');
		expect(columnLabel(25)).toBe('Z');
		expect(columnLabel(26)).toBe('AA');
		expect(columnLabel(27)).toBe('AB');
		expect(columnLabel(701)).toBe('ZZ');
		expect(columnLabel(702)).toBe('AAA');
	});
});

describe('boundedGrid', () => {
	test('writes empty cells as empty text and widens to the longest row', () => {
		expect(boundedGrid([['a', null], [1, undefined, true]])).toEqual({
			rows: [['a', ''], ['1', '', 'true']],
			columnCount: 3,
			isTruncated: false
		});
	});

	test('says when it cut rows or columns', () => {
		const tall = Array.from({ length: maxPreviewRows + 1 }, () => ['x']);
		const wide = [Array.from({ length: maxPreviewColumns + 1 }, () => 'x')];
		expect(boundedGrid(tall)).toMatchObject({ isTruncated: true });
		expect(boundedGrid(tall).rows).toHaveLength(maxPreviewRows);
		expect(boundedGrid(wide)).toMatchObject({ isTruncated: true, columnCount: maxPreviewColumns });
	});

	test('answers an empty sheet with no columns', () => {
		expect(boundedGrid([])).toEqual({ rows: [], columnCount: 0, isTruncated: false });
	});
});

describe('cardPreviewOf', () => {
	test('draws a thumbnail for the kinds that have a cheap first page', () => {
		expect(cardPreviewOf('audit.pdf')).toBe('thumbnail');
		expect(cardPreviewOf('ledger.csv', 4_096)).toBe('thumbnail');
	});

	test('shows the document itself for every other kind it can draw', () => {
		for (const name of ['notes.md', 'memo.txt', 'config.json', 'report.html', 'contract.docx', 'ir-deck.pptx']) {
			expect(cardPreviewOf(name, 4_096)).toBe('document');
		}
	});

	test('shows nothing for a format without a preview or a file too large to fetch for a card', () => {
		expect(cardPreviewOf('contract.hwp')).toBe('none');
		expect(cardPreviewOf('server.log', 3 * 1024 * 1024)).toBe('none');
		expect(cardPreviewOf('ir-deck.pptx', 21 * 1024 * 1024)).toBe('none');
	});
});

describe('cardAspectRatioOf', () => {
	test('follows the document between a portrait page and a three-to-one strip', () => {
		expect(cardAspectRatioOf(16 / 9)).toBeCloseTo(16 / 9);
		expect(cardAspectRatioOf(0.2)).toBeCloseTo(Math.SQRT1_2);
		expect(cardAspectRatioOf(8)).toBe(3);
	});
});
