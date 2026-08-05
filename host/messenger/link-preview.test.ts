import { describe, expect, test } from 'bun:test';
import { reachableAddress } from './link-preview';

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
