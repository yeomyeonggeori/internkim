import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, test } from 'bun:test';

const copies = [
	['base64url.ts', 'src/lib/notifications/base64url.ts'],
	['same-secret.ts', 'src/lib/server/same-secret.ts'],
	['who-answers.ts', 'src/lib/server/who-answers.ts']
] as const;

function bodyOf(path: string): string {
	return readFileSync(path, 'utf8').replace(/^(?:import[^\n]*\n)+/, '');
}

function sharedBody(name: string): string {
	return bodyOf(join(import.meta.dir, '../../../../supabase/functions/_shared', name));
}

function webBody(path: string): string {
	return bodyOf(join(import.meta.dir, '../../..', path));
}

describe('the functions copies stay word for word the web ones', () => {
	for (const [name, path] of copies) {
		test(`${name} says what ${path} says`, () => {
			expect(sharedBody(name)).toContain('export');
			expect(sharedBody(name)).toBe(webBody(path));
		});
	}
});
