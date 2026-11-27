import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const repositoryRoot = new URL('../../../../', import.meta.url);
const appRepository = new URL('.dependency/internkim-app/', repositoryRoot);

const copies = [
	{ original: 'internal/desktop/commands.json', copy: 'internal/desktop/commands.json' },
	{ original: 'assets/internkim.square.png', copy: 'assets/internkim.square.png' },
	{ original: 'web/tests/fixtures/attendance-today-cases.json', copy: 'tests/fixtures/attendance-today-cases.json' }
];

describe('a file the app repository copies from this one', () => {
	for (const { original, copy } of copies) {
		test(`${copy} is ${original}, byte for byte`, () => {
			const copied = readFileSync(new URL(copy, appRepository));
			expect(copied.equals(readFileSync(new URL(original, repositoryRoot)))).toBe(true);
		});
	}
});
