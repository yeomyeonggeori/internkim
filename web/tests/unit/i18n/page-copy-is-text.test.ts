import { describe, expect, test } from 'bun:test';
import { Glob } from 'bun';

// createPageText answers with '' for anything that is not a string, an array or
// another tree, so copy written as a function reads as empty and throws where it
// is rendered. A value with something to interpolate is a template the caller
// replaces into.
function offendingLeaves(tree: unknown, path: string[]): string[] {
	if (typeof tree === 'string') return [];
	if (Array.isArray(tree)) {
		return tree.flatMap((entry, index) => offendingLeaves(entry, [...path, String(index)]));
	}
	if (tree && typeof tree === 'object') {
		return Object.entries(tree).flatMap(([key, value]) => offendingLeaves(value, [...path, key]));
	}
	return [path.join('.')];
}

describe('page copy', () => {
	test('is text the localizer can hand back, in every locale of every screen', async () => {
		const offenders: string[] = [];
		for await (const file of new Glob('src/**/text.ts').scan('.')) {
			const module = (await import(`../../../${file}`)) as Record<string, unknown>;
			for (const [name, messages] of Object.entries(module)) {
				offenders.push(...offendingLeaves(messages, [file, name]));
			}
		}
		expect(offenders).toEqual([]);
	});
});
