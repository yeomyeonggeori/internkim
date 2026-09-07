import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');
const faviconMaster = readFileSync(join(repositoryRoot, 'assets', 'internkim.favicon.svg'), 'utf8');
const tabFavicon = readFileSync(join(repositoryRoot, 'web', 'static', 'logo.svg'), 'utf8');

describe('the tab favicon is the favicon master', () => {
	test('it is the master byte for byte', () => {
		expect(tabFavicon).toBe(faviconMaster);
	});

	test('the master is a vector that scales to 16 pixels', () => {
		const opening = tabFavicon.match(/<svg\b[^>]*>/);
		expect(opening).not.toBeNull();
		expect((opening as RegExpMatchArray)[0]).toContain('viewBox=');
	});
});
