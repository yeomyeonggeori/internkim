import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';
import { cn } from '../../../src/lib/utils';

type Side = 'top' | 'right' | 'bottom' | 'left';

const sheetContentSource = readFileSync(
	new URL('../../../src/lib/components/ui/sheet/sheet-content.svelte', import.meta.url),
	'utf8'
);

function extractQuotedLiteral(source: string, marker: string): string {
	const afterMarker = source.split(marker)[1];
	if (afterMarker === undefined) throw new Error(`marker not found: ${marker}`);
	const match = afterMarker.match(/"([^"]+)"/);
	if (!match) throw new Error(`no quoted literal after marker: ${marker}`);
	return match[1];
}

function extractTernaryConsequent(source: string, marker: string): string {
	const afterMarker = source.split(marker)[1];
	if (afterMarker === undefined) throw new Error(`marker not found: ${marker}`);
	const match = afterMarker.match(/\?\s*"([^"]+)"\s*:\s*""/);
	if (!match) throw new Error(`no ternary consequent after marker: ${marker}`);
	return match[1];
}

const baseClass = extractQuotedLiteral(sheetContentSource, 'class={cn(');
const sideWidthLiteral = extractTernaryConsequent(sheetContentSource, 'sideWidthClass = $derived(');

function mergedClassName(side: Side, callSiteClass: string | undefined): string {
	const sideWidthClass = side === 'left' || side === 'right' ? sideWidthLiteral : '';
	return cn(baseClass, sideWidthClass, callSiteClass);
}

function widthTokens(className: string): string[] {
	return className.split(/\s+/).filter((token) => /(^|:)w-/.test(token) && !/max-w-/.test(token));
}

function maxWidthTokens(className: string): string[] {
	return className.split(/\s+/).filter((token) => /(^|:)max-w-/.test(token));
}

describe('sheet-content width resolution', () => {
	test('a right sheet with no call site width keeps the component default', () => {
		const merged = mergedClassName('right', undefined);
		expect(widthTokens(merged)).toEqual(['w-3/4']);
		expect(maxWidthTokens(merged)).toEqual(['sm:max-w-sm']);
	});

	test('a right sheet declaring w-full and sm:max-w-xl renders at exactly that width', () => {
		const merged = mergedClassName('right', 'w-full gap-0 overflow-hidden p-0 sm:max-w-xl');
		expect(widthTokens(merged)).toEqual(['w-full']);
		expect(maxWidthTokens(merged)).toEqual(['sm:max-w-xl']);
	});

	test('a right sheet declaring only w-full keeps the component default max-width', () => {
		const merged = mergedClassName('right', 'w-full gap-0 p-0');
		expect(widthTokens(merged)).toEqual(['w-full']);
		expect(maxWidthTokens(merged)).toEqual(['sm:max-w-sm']);
	});

	test('a left sheet declaring an arbitrary width keeps the component default max-width', () => {
		const merged = mergedClassName('left', 'grid w-[min(20rem,90vw)] grid-rows-[auto_minmax(0,1fr)] gap-0 p-0');
		expect(widthTokens(merged)).toEqual(['w-[min(20rem,90vw)]']);
		expect(maxWidthTokens(merged)).toEqual(['sm:max-w-sm']);
	});

	test('a bottom sheet never receives a width or max-width default', () => {
		const merged = mergedClassName('bottom', 'max-h-[92vh] gap-0 rounded-t-xl p-0');
		expect(widthTokens(merged)).toEqual([]);
		expect(maxWidthTokens(merged)).toEqual([]);
	});

	test('the merged class set never carries two competing width or max-width declarations', () => {
		const cases: Array<[Side, string | undefined]> = [
			['right', 'w-full sm:max-w-2xl'],
			['left', undefined],
			['right', undefined],
			['top', 'max-h-[92vh]'],
			['bottom', undefined]
		];
		for (const [side, callSiteClass] of cases) {
			const merged = mergedClassName(side, callSiteClass);
			expect(widthTokens(merged).length).toBeLessThanOrEqual(1);
			expect(maxWidthTokens(merged).length).toBeLessThanOrEqual(1);
		}
	});
});
