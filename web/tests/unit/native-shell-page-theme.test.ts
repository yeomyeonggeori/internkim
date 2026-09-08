import { describe, expect, test } from 'bun:test';
import { hexOfComputedColor } from '$lib/native-shell/page-theme';

describe('hexOfComputedColor', () => {
	test('turns a computed rgb() colour into upper-case hex', () => {
		expect(hexOfComputedColor('rgb(9, 9, 11)')).toBe('#09090B');
		expect(hexOfComputedColor('rgb(255, 255, 255)')).toBe('#FFFFFF');
	});

	test('ignores an alpha channel', () => {
		expect(hexOfComputedColor('rgba(9, 9, 11, 0.5)')).toBe('#09090B');
	});

	test('answers null for a value it cannot read', () => {
		expect(hexOfComputedColor('transparent')).toBeNull();
		expect(hexOfComputedColor('')).toBeNull();
	});
});
