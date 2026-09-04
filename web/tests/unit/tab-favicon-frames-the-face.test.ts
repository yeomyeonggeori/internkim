import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');
const vectorMaster = readFileSync(join(repositoryRoot, 'assets', 'internkim.square.svg'), 'utf8');
const tabFavicon = readFileSync(join(repositoryRoot, 'web', 'static', 'logo.svg'), 'utf8');

function openingTag(svg: string): string {
	const opening = svg.match(/<svg\b[^>]*>/);
	expect(opening).not.toBeNull();
	return (opening as RegExpMatchArray)[0];
}

describe('the tab favicon is the square master framed on the face', () => {
	test('it is the master with only a viewBox added', () => {
		const framed = openingTag(tabFavicon);
		const master = openingTag(vectorMaster);
		expect(framed.replace(/ viewBox="[^"]*"/, '')).toBe(master);
		expect(tabFavicon.replace(framed, master)).toBe(vectorMaster);
	});

	test('the viewBox frames the face rather than the whole square', () => {
		const viewBox = openingTag(tabFavicon).match(/viewBox="([^"]*)"/);
		expect(viewBox).not.toBeNull();
		const [left, top, width, height] = (viewBox as RegExpMatchArray)[1].split(' ').map(Number);
		expect(width).toBe(height);
		expect(width).toBeLessThan(1024);
		expect(left + width).toBeLessThanOrEqual(1024);
		expect(top + height).toBeLessThanOrEqual(1024);
	});
});
