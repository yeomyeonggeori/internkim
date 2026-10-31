import { describe, expect, test } from 'bun:test';
import { Glob } from 'bun';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const endToEndDirectory = fileURLToPath(new URL('../e2e/', import.meta.url));
const numericWidthLoop = /for\s*\(\s*const\s+width\s+of\s+\[\s*\d/g;

function lineOf(source: string, index: number): number {
	return source.slice(0, index).split('\n').length;
}

function handWrittenWidthLoops(): string[] {
	const found: string[] = [];
	for (const path of new Glob('**/*.spec.ts').scanSync(endToEndDirectory)) {
		const source = readFileSync(`${endToEndDirectory}${path}`, 'utf8');
		for (const loop of source.matchAll(numericWidthLoop)) found.push(`${path}:${lineOf(source, loop.index)}`);
	}
	return found;
}

describe('screen widths', () => {
	test('a spec loops over the widths named in support/screen-widths.ts, never a list of its own', () => {
		expect(handWrittenWidthLoops()).toEqual([]);
	});
});
