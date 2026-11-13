import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';
import { backgroundTokenOf, type ThemeSelector } from './background-token';

type Mode = { selector: ThemeSelector; colorsFile: string };

const modes: Mode[] = [
	{ selector: ':root', colorsFile: 'android/app/src/main/res/values/colors.xml' },
	{ selector: '.dark', colorsFile: 'android/app/src/main/res/values-night/colors.xml' }
];

function statusBarColourOf(xml: string): string {
	const match = xml.match(/<color name="status_bar">#([0-9A-Fa-f]{6})<\/color>/);
	if (!match) throw new Error('no status_bar colour');
	return match[1].toUpperCase();
}

describe('the Android status bar colour', () => {
	const css = readFileSync('src/theme.css', 'utf8');

	for (const mode of modes) {
		test(`matches the web --background token under ${mode.selector}`, () => {
			expect(statusBarColourOf(readFileSync(mode.colorsFile, 'utf8'))).toBe(
				backgroundTokenOf(css, mode.selector)
			);
		});
	}
});
