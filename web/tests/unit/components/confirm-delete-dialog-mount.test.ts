import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const rootLayout = readFileSync(new URL('../../../src/routes/+layout.svelte', import.meta.url), 'utf8');
const appRail = readFileSync(new URL('../../../src/lib/components/app-rail.svelte', import.meta.url), 'utf8');
const routePages = {
	task: readFileSync(new URL('../../../src/routes/task/+page.svelte', import.meta.url), 'utf8'),
	memory: readFileSync(new URL('../../../src/routes/memory/+page.svelte', import.meta.url), 'utf8'),
	runs: readFileSync(new URL('../../../src/routes/runs/+page.svelte', import.meta.url), 'utf8')
};

function countDialogMounts(source: string): number {
	return (source.match(/<ConfirmDeleteDialog\s*\/>/g) ?? []).length;
}

describe('the delete confirmation dialog mounts once per route', () => {
	test('the root layout mounts the app rail, which owns the one dialog instance', () => {
		expect(rootLayout).toContain('<AppRail');
		expect(countDialogMounts(appRail)).toBe(1);
	});

	for (const [routeName, pageSource] of Object.entries(routePages)) {
		test(`${routeName} does not mount its own copy of the dialog`, () => {
			expect(countDialogMounts(pageSource)).toBe(0);
		});

		test(`${routeName}'s rendered tree carries exactly one dialog element, so opening the shared store opens exactly one`, () => {
			const renderedTreeSource = rootLayout + appRail + pageSource;
			expect(countDialogMounts(renderedTreeSource)).toBe(1);
		});
	}
});
