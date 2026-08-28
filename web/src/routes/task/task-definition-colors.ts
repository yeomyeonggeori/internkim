import { colorPickerPalette, normalizeColor } from '$lib/color-picker-palette';
import type { TaskDefinitions } from './task-types';

const unknownDefinitionColor = '#64748b';

export function taskBusinessColor(business: string, definitions: TaskDefinitions): string {
	return definitionColor(business, definitions.categories, definitions.categoryColors);
}

export function taskTypeColor(type: string, definitions: TaskDefinitions): string {
	return definitionColor(type, definitions.types, definitions.typeColors);
}

const paletteHueStride = 11;

export function taskDefinitionPaletteColor(index: number): string {
	const strideIndex = (Math.abs(index) * paletteHueStride) % colorPickerPalette.length;
	return colorPickerPalette[strideIndex];
}

export function taskDefinitionOutlineBadgeStyle(color: string): string {
	if (!color) return '';
	return `color: ${color}; border-color: ${color}66; background: ${color}0f;`;
}

export function taskDefinitionBadgeStyle(color: string): string {
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
	return taskDefinitionPaletteColor(index);
}
