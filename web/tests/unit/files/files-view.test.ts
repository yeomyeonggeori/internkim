import { describe, expect, test } from 'bun:test';
import { formatFileSize, parseDelimitedText } from '../../../src/lib/files/view';

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

describe('parseDelimitedText', () => {
	test('splits comma rows into a grid', () => {
		expect(parseDelimitedText('name,amount\n예산,1000', ',')).toEqual([
			['name', 'amount'],
			['예산', '1000']
		]);
	});

	test('ignores trailing newlines and carriage returns', () => {
		expect(parseDelimitedText('a\tb\r\nc\td\n\n', '\t')).toEqual([
			['a', 'b'],
			['c', 'd']
		]);
	});
});
