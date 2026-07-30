import { describe, expect, test } from 'bun:test';
import { colorPickerPalette, fallbackPickerColor, normalizeColor } from '../../src/lib/color-picker-palette';

describe('color picker palette', () => {
	test('every color keeps white text readable', () => {
		const unreadableColors = [...colorPickerPalette, fallbackPickerColor].filter(
			(color) => whiteTextContrastRatio(color) < 4.5
		);

		expect(unreadableColors).toEqual([]);
	});

	test('offers a wide choice without duplicates', () => {
		expect(colorPickerPalette.length > 24).toBe(true);
		expect(new Set(colorPickerPalette).size).toBe(colorPickerPalette.length);
	});

	test('normalizes shorthand and invalid colors', () => {
		expect(normalizeColor('#ABC')).toBe('#aabbcc');
		expect(normalizeColor(' #2563EB ')).toBe('#2563eb');
		expect(normalizeColor('blue')).toBe(fallbackPickerColor);
	});
});

function whiteTextContrastRatio(color: string): number {
	return 1.05 / (relativeLuminance(color) + 0.05);
}

function relativeLuminance(color: string): number {
	const [red, green, blue] = [1, 3, 5].map((offset) => {
		const channel = Number.parseInt(color.slice(offset, offset + 2), 16) / 255;
		return channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
	});
	return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
}
