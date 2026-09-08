import { readFileSync } from 'node:fs';
import { describe, expect, test } from 'bun:test';

type Mode = { selector: string; colorsFile: string };

const modes: Mode[] = [
	{ selector: ':root', colorsFile: 'android/app/src/main/res/values/colors.xml' },
	{ selector: '.dark', colorsFile: 'android/app/src/main/res/values-night/colors.xml' }
];

function backgroundTokenOf(css: string, selector: string): string {
	const block = css.slice(css.indexOf(`${selector} {`));
	const match = block.match(/--background:\s*([\d.]+)\s+([\d.]+)%\s+([\d.]+)%/);
	if (!match) throw new Error(`no --background token under ${selector}`);
	return hslToHex(Number(match[1]), Number(match[2]) / 100, Number(match[3]) / 100);
}

function hslToHex(hue: number, saturation: number, lightness: number): string {
	const chroma = (1 - Math.abs(2 * lightness - 1)) * saturation;
	const sector = (hue / 60) % 2;
	const secondary = chroma * (1 - Math.abs(sector - 1));
	const offset = lightness - chroma / 2;
	const sectors: [number, number, number][] = [
		[chroma, secondary, 0],
		[secondary, chroma, 0],
		[0, chroma, secondary],
		[0, secondary, chroma],
		[secondary, 0, chroma],
		[chroma, 0, secondary]
	];
	const [red, green, blue] = sectors[Math.floor(hue / 60) % 6];
	return [red, green, blue]
		.map((channel) => Math.round((channel + offset) * 255).toString(16).padStart(2, '0'))
		.join('')
		.toUpperCase();
}

function statusBarColourOf(xml: string): string {
	const match = xml.match(/<color name="status_bar">#([0-9A-Fa-f]{6})<\/color>/);
	if (!match) throw new Error('no status_bar colour');
	return match[1].toUpperCase();
}

describe('the Android status bar colour', () => {
	const css = readFileSync('src/app.css', 'utf8');

	for (const mode of modes) {
		test(`matches the web --background token under ${mode.selector}`, () => {
			expect(statusBarColourOf(readFileSync(mode.colorsFile, 'utf8'))).toBe(
				backgroundTokenOf(css, mode.selector)
			);
		});
	}
});
