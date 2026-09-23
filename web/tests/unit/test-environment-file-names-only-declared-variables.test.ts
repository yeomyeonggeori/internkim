import { existsSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, test } from 'bun:test';

const repositoryRoot = join(import.meta.dir, '..', '..', '..');
const testEnvironmentPath = join(repositoryRoot, 'web', '.env.test');
const declarationsPath = join(repositoryRoot, 'tools', 'environment.json');

function namesIn(environmentFileText: string): string[] {
	return environmentFileText
		.split('\n')
		.map((line) => line.trim())
		.filter((line) => line && !line.startsWith('#'))
		.map((line) => line.split('=')[0]?.trim())
		.filter((name): name is string => Boolean(name));
}

describe('web/.env.test', () => {
	test('names only variables tools/environment.json declares', () => {
		if (!existsSync(testEnvironmentPath)) {
			// Gitignored local state (AGENTS.md: it blanks the central plane so
			// unit tests cannot reach it) — nothing to check when a developer or
			// CI has not created one.
			return;
		}
		const declarations = JSON.parse(readFileSync(declarationsPath, 'utf8')) as Record<string, unknown>;
		const names = namesIn(readFileSync(testEnvironmentPath, 'utf8'));
		const undeclared = names.filter((name) => !(name in declarations));
		expect(
			undeclared,
			`web/.env.test names ${undeclared.join(', ')}, which tools/environment.json does not declare — ` +
				'a variable no reader has is not one a test file should blank'
		).toEqual([]);
	});
});
