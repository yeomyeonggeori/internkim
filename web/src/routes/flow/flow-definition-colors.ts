import { colorPickerPalette, normalizeColor } from '$lib/color-picker-palette';
import type { FlowDefinitions } from './flow-types';

const unknownDefinitionColor = '#64748b';

export function flowBusinessColor(business: string, definitions: FlowDefinitions): string {
	return definitionColor(business, definitions.categories, definitions.categoryColors);
}

export function flowTaskTypeColor(type: string, definitions: FlowDefinitions): string {
	return definitionColor(type, definitions.types, definitions.typeColors);
}

const paletteHueStride = 11;

export function flowDefinitionPaletteColor(index: number): string {
	const strideIndex = (Math.abs(index) * paletteHueStride) % colorPickerPalette.length;
	return colorPickerPalette[strideIndex];
}

export function flowDefinitionOutlineBadgeStyle(color: string): string {
	if (!color) return '';
	return `color: ${color}; border-color: ${color}66; background: ${color}0f;`;
}

export function flowDefinitionBadgeStyle(color: string): string {
	if (!color) return '';
	return `background: ${color}; color: #ffffff;`;
}

function definitionColor(
	value: string,
	values: string[],
	storedColors: Record<string, string> | undefined
): string {
	const trimmedValue = value.trim();
	const storedColor = storedColors?.[trimmedValue];
	if (storedColor) return normalizeColor(storedColor);
	const index = values.indexOf(trimmedValue);
	if (index < 0) return unknownDefinitionColor;
	return flowDefinitionPaletteColor(index);
}
