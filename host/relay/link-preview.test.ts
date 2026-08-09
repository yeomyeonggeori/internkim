import { describe, expect, test } from 'bun:test';
import { decodePage, reachableAddress } from './link-preview';

describe('reachableAddress', () => {
	test('an ordinary web address is fetched', () => {
		expect(reachableAddress('https://example.com/article')?.hostname).toBe('example.com');
		expect(reachableAddress('http://example.com')?.hostname).toBe('example.com');
	});

	test('only the web is fetched', () => {
		expect(reachableAddress('ftp://example.com/file')).toBeNull();
		expect(reachableAddress('file:///etc/passwd')).toBeNull();
		expect(reachableAddress('not a url')).toBeNull();
	});

	test('the company network is not fetched on a stranger behalf', () => {
		for (const link of [
			'http://localhost:8080/admin',
			'http://127.0.0.1/admin',
			'http://10.0.0.5/',
			'http://192.168.1.1/',
			'http://172.20.3.4/',
			'http://169.254.169.254/latest/meta-data',
			'http://postgres.internal/',
			'http://[::1]:8080/'
		]) {
			expect(reachableAddress(link)).toBeNull();
		}
	});
});

describe('decodePage', () => {
	const euckrHangul = new Uint8Array([0xc7, 0xd1, 0xb1, 0xb9, 0xbe, 0xee]);

	test('reads the charset the page declares in its header', () => {
		expect(decodePage(euckrHangul, 'text/html; charset=euc-kr')).toBe('한국어');
	});

	test('falls back to the charset in the document when the header is silent', () => {
		const document = new TextEncoder().encode('<meta charset="utf-8"><title>한국어</title>');
		expect(decodePage(document, 'text/html')).toContain('한국어');
	});

	test('an unknown charset does not throw', () => {
		expect(decodePage(new TextEncoder().encode('hi'), 'text/html; charset=nonsense-9')).toBe('hi');
	});
});
