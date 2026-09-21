import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { homePath } from '../../src/lib/home-path';
import { returnPathOf, withReturnPath } from '../../src/lib/return-path';

const thisApp = 'https://intern.kim';

const nextStop = (link: string, fallback = '') => returnPathOf(new URL(link, thisApp)) || fallback;

function everySourceFile(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const path = join(directory, entry.name);
		if (entry.isDirectory()) return everySourceFile(path);
		return path.endsWith('.ts') || path.endsWith('.svelte') ? [path] : [];
	});
}

function filesMatching(pattern: RegExp): string[] {
	return everySourceFile('src')
		.filter((path) => path !== 'src/lib/return-path.ts')
		.filter((path) => pattern.test(readFileSync(path, 'utf8')));
}

describe('an MCP client sends somebody with no account to consent', () => {
	const consent = '/oauth/consent?authorization_id=abc123';

	test('the authorization survives founding a company and comes back', () => {
		const gateLink = withReturnPath('/auth/claim?new-company=1', consent);
		const afterClaiming = nextStop(gateLink, homePath);
		const bouncedToStart = withReturnPath('/start', afterClaiming);
		const afterFounding = nextStop(bouncedToStart);

		expect(afterClaiming).toBe(consent);
		expect(afterFounding).toBe(consent);
	});

	test('somebody who was invited instead lands back on consent the same way', () => {
		const gateLink = withReturnPath('/auth/claim', consent);
		expect(nextStop(gateLink, homePath)).toBe(consent);
	});

	test('with nothing to come back to, every hop keeps its own default', () => {
		expect(withReturnPath('/auth/claim', '')).toBe('/auth/claim');
		expect(nextStop('/auth/claim', homePath)).toBe(homePath);
		expect(nextStop('/start')).toBe('');
	});
});

describe('the one place that decides what a return path may be', () => {
	test('nothing else under src writes the parameter', () => {
		expect(filesMatching(/[?&]return=/)).toEqual([]);
	});

	test('nothing else under src judges a path by the slashes it starts with', () => {
		expect(filesMatching(/startsWith\(['"]\/\/['"]\)/)).toEqual([]);
	});
});
