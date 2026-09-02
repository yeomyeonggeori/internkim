import { describe, expect, test } from 'bun:test';
import { isShippedFile } from '../../src/lib/offline-shell';

const appOrigin = 'https://acme.example.test';
const shipped = new Set(['/_app/immutable/entry/app.CZVK1F5r.js', '/icon-192.png', '/flow/']);

function get(url: string) {
	return { method: 'GET', url };
}

describe('isShippedFile', () => {
	test('a file that shipped with this build is served from the cache', () => {
		expect(isShippedFile(get(`${appOrigin}/icon-192.png`), appOrigin, shipped)).toBe(true);
		expect(
			isShippedFile(get(`${appOrigin}/_app/immutable/entry/app.CZVK1F5r.js`), appOrigin, shipped)
		).toBe(true);
	});

	test('an answer meant for one member is never cached', () => {
		expect(isShippedFile(get(`${appOrigin}/api/member/me`), appOrigin, shipped)).toBe(false);
		expect(isShippedFile(get(`${appOrigin}/attendance/api/summary`), appOrigin, shipped)).toBe(false);
	});

	test('a query string means the answer depends on more than the path', () => {
		expect(isShippedFile(get(`${appOrigin}/flow/?taskID=7`), appOrigin, shipped)).toBe(false);
	});

	test('nothing another host serves is touched', () => {
		expect(isShippedFile(get('https://mattermost.acme.test/icon-192.png'), appOrigin, shipped)).toBe(false);
		expect(isShippedFile(get(`${appOrigin.replace('acme', 'other')}/icon-192.png`), appOrigin, shipped)).toBe(false);
	});

	test('only a read can come from the cache', () => {
		expect(isShippedFile({ method: 'POST', url: `${appOrigin}/flow/` }, appOrigin, shipped)).toBe(false);
		expect(isShippedFile({ method: 'HEAD', url: `${appOrigin}/icon-192.png` }, appOrigin, shipped)).toBe(false);
	});

	test('an address that is not one is not a file', () => {
		expect(isShippedFile(get('icon-192.png'), appOrigin, shipped)).toBe(false);
	});
});
