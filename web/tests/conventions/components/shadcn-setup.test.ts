import { describe, expect, test } from 'bun:test';
import { readFileSync } from 'node:fs';

const appStyles = readFileSync(new URL('../../../src/app.css', import.meta.url), 'utf8');

const requiredCustomVariants = ['data-open', 'data-closed', 'data-checked', 'data-unchecked', 'data-disabled', 'data-active', 'data-horizontal', 'data-vertical'];
const stateCustomVariants = ['data-open', 'data-closed', 'data-checked', 'data-unchecked', 'data-active'];

describe('shadcn-svelte stylesheet setup', () => {
	test('app.css declares every custom variant the registry components style with', () => {
		const missingVariants = requiredCustomVariants.filter((variantName) => !appStyles.includes(`@custom-variant ${variantName} {`));
		expect(missingVariants).toEqual([]);
	});

	test('state variants match both the bits-ui state attribute and the boolean attribute', () => {
		const brokenVariants = stateCustomVariants.filter((variantName) => {
			const declaration = readVariantBody(appStyles, variantName);
			const stateValue = variantName.replace('data-', '');
			return !declaration.includes(`[data-state="${stateValue}"]`) || !declaration.includes(`[${variantName}]:not([${variantName}="false"])`);
		});
		expect(brokenVariants).toEqual([]);
	});

	test('app.css declares the no-scrollbar utility used by the sidebar and command list', () => {
		expect(appStyles.includes('@utility no-scrollbar {')).toBe(true);
	});

	test('app.css declares the accordion keyframes', () => {
		expect(appStyles.includes('@keyframes accordion-down')).toBe(true);
		expect(appStyles.includes('@keyframes accordion-up')).toBe(true);
	});
});

function readVariantBody(styles: string, variantName: string): string {
	const start = styles.indexOf(`@custom-variant ${variantName} {`);
	const end = styles.indexOf('\n}', start);
	return styles.slice(start, end);
}
