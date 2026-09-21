import { describe, expect, test } from 'bun:test';
import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { homePath } from '../../src/lib/home-path';
import { returnPathOf, withReturnPath } from '../../src/lib/return-path';

const thisApp = 'https://intern.kim';
const signInGate = readFileSync('src/lib/components/web-auth-gate.svelte', 'utf8');
const claimPage = readFileSync('src/routes/auth/claim/+page.svelte', 'utf8');
const rootLayout = readFileSync('src/routes/+layout.ts', 'utf8');
const startPage = readFileSync('src/routes/start/+page.svelte', 'utf8');

const nextStop = (link: string, fallback = '') => returnPathOf(new URL(link, thisApp)) || fallback;

function everySourceFile(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const path = join(directory, entry.name);
		if (entry.isDirectory()) return everySourceFile(path);
		return path.endsWith('.ts') || path.endsWith('.svelte') ? [path] : [];
	});
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

describe('the pages that make up that path', () => {
	const linesCarrying = (source: string, needle: string) =>
		source.split('\n').filter((line) => line.includes(needle));

	test('the sign-in gate builds both sign-up links with the return path', () => {
		expect(linesCarrying(signInGate, "withReturnPath('/auth/claim', returnPath)")).toHaveLength(1);
		expect(linesCarrying(signInGate, "withReturnPath('/auth/claim?new-company=1', returnPath)")).toHaveLength(1);
		expect(linesCarrying(signInGate, 'href={claimURL}')).toHaveLength(1);
		expect(linesCarrying(signInGate, 'href={startCompanyURL}')).toHaveLength(1);
	});

	test('nothing under src writes the parameter beside the one place that names it', () => {
		const written = everySourceFile('src')
			.filter((path) => path !== 'src/lib/return-path.ts')
			.filter((path) => linesCarrying(readFileSync(path, 'utf8'), '?return=').length > 0);
		expect(written).toEqual([]);
	});

	test('the claim page leaves for the return path and falls back to home', () => {
		expect(linesCarrying(claimPage, 'returnPathOf(page.url) || homePath')).toHaveLength(1);
		expect(linesCarrying(claimPage, 'goto(homePath)')).toHaveLength(0);
	});

	test('the bounce to founding remembers where it interrupted', () => {
		expect(linesCarrying(rootLayout, "withReturnPath('/start', returnPath)")).toHaveLength(1);
		expect(linesCarrying(rootLayout, "redirect(307, '/start')")).toHaveLength(0);
	});

	test('founding a company returns to it', () => {
		expect(linesCarrying(startPage, 'if (whereTheyWereGoing) await goto(whereTheyWereGoing)')).toHaveLength(1);
		expect(linesCarrying(startPage, 'goto(whereTheyWereGoing || homePath)')).toHaveLength(1);
	});
});
