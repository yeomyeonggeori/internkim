import { describe, expect, test } from 'bun:test';
import { averageSaturation, hexToHSL, hslToHex } from '../../src/lib/color-hsl';
import { colorPickerPalette } from '../../src/lib/color-picker-palette';

describe('reading a hex colour as hue, saturation and lightness', () => {
	test('reads the primaries', () => {
		expect(hexToHSL('#ff0000')).toEqual({ hue: 0, saturation: 100, lightness: 50 });
		expect(hexToHSL('#00ff00')).toEqual({ hue: 120, saturation: 100, lightness: 50 });
		expect(hexToHSL('#0000ff')).toEqual({ hue: 240, saturation: 100, lightness: 50 });
	});

	test('reads greys as unsaturated', () => {
		expect(hexToHSL('#000000')).toEqual({ hue: 0, saturation: 0, lightness: 0 });
		expect(hexToHSL('#ffffff')).toEqual({ hue: 0, saturation: 0, lightness: 100 });
		expect(hexToHSL('#808080').saturation).toBe(0);
	});
});

describe('writing hue, saturation and lightness back as hex', () => {
	test('writes the primaries', () => {
		expect(hslToHex({ hue: 0, saturation: 100, lightness: 50 })).toBe('#ff0000');
		expect(hslToHex({ hue: 120, saturation: 100, lightness: 50 })).toBe('#00ff00');
		expect(hslToHex({ hue: 240, saturation: 100, lightness: 50 })).toBe('#0000ff');
	});

	test('wraps a hue past the circle', () => {
		expect(hslToHex({ hue: 360, saturation: 100, lightness: 50 })).toBe('#ff0000');
		expect(hslToHex({ hue: -120, saturation: 100, lightness: 50 })).toBe('#0000ff');
	});

	test('returns every palette colour within one step per channel, the most integer hue and lightness can carry', () => {
		for (const paletteColor of colorPickerPalette) {
			const roundTripped = hslToHex(hexToHSL(paletteColor));
			for (const [channel, value] of channelsOf(roundTripped).entries()) {
				expect(Math.abs(value - channelsOf(paletteColor)[channel])).toBeLessThanOrEqual(2);
			}
		}
	});
});

function channelsOf(hex: string): number[] {
	return [1, 3, 5].map((offset) => parseInt(hex.slice(offset, offset + 2), 16));
}

describe('the saturation the curated palette sits at', () => {
	test('is the mean of its colours', () => {
		expect(averageSaturation(['#ff0000', '#808080'])).toBe(50);
	});

	test('is a vivid but not maximal level for the palette', () => {
		const paletteLevel = averageSaturation(colorPickerPalette);
		expect(paletteLevel).toBeGreaterThan(40);
		expect(paletteLevel).toBeLessThan(90);
	});
});
