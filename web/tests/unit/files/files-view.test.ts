import { describe, expect, test } from 'bun:test';
import { fileVisual, formatFileSize } from '../../../src/lib/files/view';
import { previewKindOf } from '../../../src/lib/files/preview/kind';

describe('formatFileSize', () => {
	test('keeps bytes under one kilobyte', () => {
		expect(formatFileSize(612)).toBe('612 B');
	});

	test('rounds kilobytes with one decimal under ten', () => {
		expect(formatFileSize(4213)).toBe('4.1 KB');
	});

	test('rounds larger units to whole numbers', () => {
		expect(formatFileSize(18244)).toBe('18 KB');
		expect(formatFileSize(5 * 1024 * 1024)).toBe('5 MB');
	});
});

describe('fileVisual', () => {
	const plainTile = fileVisual('unknown.bin').tileClass;

	test('gives every format the preview can draw a tile of its own', () => {
		const drawn = ['a.pdf', 'a.png', 'a.avif', 'a.xlsx', 'a.xlsm', 'a.ods', 'a.csv', 'a.pptx', 'a.docx', 'a.html', 'a.md', 'a.txt', 'a.log', 'a.json'];
		for (const name of drawn) {
			expect(previewKindOf(name)).not.toBe('none');
			expect(fileVisual(name).tileClass).not.toBe(plainTile);
		}
	});

	test('tells video, sound and archives apart from an unknown file', () => {
		const tiles = ['clip.mp4', 'memo.m4a', 'backup.zip'].map((name) => fileVisual(name).tileClass);
		expect(new Set([...tiles, plainTile]).size).toBe(4);
	});
});
