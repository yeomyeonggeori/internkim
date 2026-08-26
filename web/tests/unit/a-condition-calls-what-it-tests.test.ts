import { describe, expect, test } from 'bun:test';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

// `{#if !isSupabaseConfigured}` reads like a question and is not one: the name is
// a function, so the branch was dead in every build for months and the section
// behind it was invisible everywhere. A page that calls a name with parentheses
// somewhere and tests it bare somewhere else is asking that same question.
//
// A snippet is the exception, and the only one: `{#if header}{@render header()}`
// asks whether the caller passed it, which is a real question with two answers.
function svelteFilesUnder(directory: string): string[] {
	const found: string[] = [];
	for (const entry of readdirSync(directory)) {
		const path = join(directory, entry);
		if (statSync(path).isDirectory()) {
			if (path.includes('components/ui')) continue;
			found.push(...svelteFilesUnder(path));
			continue;
		}
		if (path.endsWith('.svelte')) found.push(path);
	}
	return found;
}

function namesTestedBare(source: string): string[] {
	const names: string[] = [];
	const condition = /\{#if\s+!?\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}/g;
	for (const found of source.matchAll(condition)) names.push(found[1]);
	return names;
}

function isSnippet(source: string, name: string): boolean {
	if (new RegExp(`\\{#snippet\\s+${name}\\b`).test(source)) return true;
	return new RegExp(`\\b${name}\\??\\s*:\\s*Snippet\\b`).test(source);
}

describe('a condition calls what it tests', () => {
	test('no page tests a name for truth that it calls elsewhere', () => {
		const asking: string[] = [];
		for (const path of svelteFilesUnder('src')) {
			const source = readFileSync(path, 'utf8');
			for (const name of namesTestedBare(source)) {
				if (isSnippet(source, name)) continue;
				const isCalled = new RegExp(`\\b${name}\\s*\\(`).test(source);
				if (isCalled) asking.push(`${path}: {#if ${name}} — ${name}() is called in the same file`);
			}
		}
		expect(asking).toEqual([]);
	});
});
