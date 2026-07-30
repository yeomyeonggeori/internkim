import { colorPickerPalette, normalizeColor } from '$lib/color-picker-palette';
import type { FlowDefinitions } from './flow-types';

const unknownDefinitionColor = '#64748b';

export function flowBusinessColor(business: string, definitions: FlowDefinitions): string {
	return definitionColor(business, definitions.categories, definitions.categoryColors);
}

export function flowTaskTypeColor(type: string, definitions: FlowDefinitions): string {
	return definitionColor(type, definitions.types, definitions.typeColors);
}

export function flowDefinitionPaletteColor(index: number): string {
	return colorPickerPalette[Math.abs(index) % colorPickerPalette.length];
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
