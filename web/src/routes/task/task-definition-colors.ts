import { normalizeColor, paletteColorAt } from '$lib/color-picker-palette';
import type { TaskDefinitions } from './task-types';

export const unknownDefinitionColor = '#64748b';

export function taskBusinessColor(business: string | null, definitions: TaskDefinitions): string {
	if (business === null) return definitions.etcBusinessColor ?? unknownDefinitionColor;
	return definitionColor(business, definitions.categories, definitions.categoryColors);
}

export function taskTypeColor(type: string | null, definitions: TaskDefinitions): string {
	if (type === null) return definitions.etcTypeColor ?? unknownDefinitionColor;
	return definitionColor(type, definitions.types, definitions.typeColors);
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
	return paletteColorAt(index);
}
