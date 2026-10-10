import { describe, expect, test } from 'bun:test';
import { formatFileSize } from '../../../src/lib/files/view';

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
