import { describe, expect, test } from 'bun:test';
import {
	boundedGrid,
	columnLabel,
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
	});

	test('names no preview for a format it cannot draw', () => {
		expect(previewKindOf('contract.docx')).toBe('none');
		expect(previewKindOf('README')).toBe('none');
		expect(previewKindOf('.gitignore')).toBe('none');
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
