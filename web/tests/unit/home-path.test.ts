import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { homePath } from '../../src/lib/home-path';

const manifest = JSON.parse(readFileSync(new URL('../../static/manifest.webmanifest', import.meta.url), 'utf8')) as {
	start_url: string;
};

describe('where the app starts', () => {
	test('the installed app opens the same address the browser lands on', () => {
		expect(manifest.start_url).toBe(homePath);
	});

	test('the address names a section, so the app shell and the auth gate recognise it', () => {
		expect(homePath.startsWith('/')).toBe(true);
		expect(homePath.endsWith('/')).toBe(true);
	});
});
