import { expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

test('shared loading primitives honor reduced motion without changing their default feedback', () => {
	const skeleton = readFileSync(new URL('../../../src/lib/components/ui/skeleton/skeleton.svelte', import.meta.url), 'utf8');
	const spinner = readFileSync(new URL('../../../src/lib/components/ui/spinner/spinner.svelte', import.meta.url), 'utf8');
	expect(skeleton).toContain('animate-pulse motion-reduce:animate-none');
	expect(spinner).toContain('animate-spin motion-reduce:animate-none');
	expect(skeleton).toContain('data-slot="skeleton"');
	expect(spinner).toContain("role = 'status'");
});
