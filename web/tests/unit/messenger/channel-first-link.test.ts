import { describe, expect, test } from 'bun:test';
import { firstLinkIn } from '../../../src/lib/components/channel/channel-link';

describe('firstLinkIn', () => {
	test('finds the link somebody pasted', () => {
		expect(firstLinkIn('이거 봐 https://example.com/article 재밌음')).toBe('https://example.com/article');
	});

	test('takes only the first of several', () => {
		expect(firstLinkIn('https://one.example https://two.example')).toBe('https://one.example');
	});

	test('leaves the sentence punctuation behind', () => {
		expect(firstLinkIn('읽어봐 https://example.com/글.')).toBe('https://example.com/글');
	});

	test('a message without a link has none', () => {
		expect(firstLinkIn('그냥 안녕')).toBe('');
		expect(firstLinkIn('ftp://example.com/file')).toBe('');
	});
});
