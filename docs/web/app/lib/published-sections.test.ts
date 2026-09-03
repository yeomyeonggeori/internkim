import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

// fumadocs-mdx rejects a non-literal `files` in a macro, so the two lists cannot
// be one import.
function sectionsIn(path: string, variableName: string): string[] {
  const source = readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8');
  const list = source.match(new RegExp(`${variableName}[^[]*\\[([^\\]]*)\\]`));
  if (!list) throw new Error(`${path} no longer spells ${variableName} as a literal array`);
  return [...list[1].matchAll(/'([^']+)'/g)].map((match) => match[1]);
}

describe('the sections the docs site publishes', () => {
  test('are the same list in source.ts and react-router.config.ts', () => {
    const indexed = sectionsIn('./source.ts', 'files:').filter((entry) => entry.endsWith('{md,mdx}'));
    const prerendered = sectionsIn('../../react-router.config.ts', 'publishedSections');
    expect(prerendered).toEqual(indexed);
  });
});
