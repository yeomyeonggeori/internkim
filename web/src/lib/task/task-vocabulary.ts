import { taskSizes } from '$lib/task/task-sizes';
import type { TaskDefinitions } from '../../routes/task/task-types';

export type NamedColour = { name: string; color?: string };

export type TaskVocabulary = {
	businesses?: NamedColour[];
	types?: NamedColour[];
	etcBusinessColor?: string;
	etcTypeColor?: string;
};

export function colourOf(entry: NamedColour): string {
	if (entry.color) return entry.color;
	let hash = 0;
	for (const character of entry.name) hash = (hash * 31 + character.codePointAt(0)!) % 360;
	return `hsl(${hash} 62% 45%)`;
}

export function taskDefinitionsOf(vocabulary: TaskVocabulary): TaskDefinitions {
	const businesses = vocabulary.businesses ?? [];
	const types = vocabulary.types ?? [];
	return {
		categories: businesses.map((business) => business.name),
		categoryColors: coloursOf(businesses),
		types: types.map((type) => type.name),
		typeColors: coloursOf(types),
		etcBusinessColor: vocabulary.etcBusinessColor,
		etcTypeColor: vocabulary.etcTypeColor,
		sizes: taskSizes()
	};
}

function coloursOf(entries: NamedColour[]): Record<string, string> {
	return Object.fromEntries(entries.flatMap((entry) => (entry.color ? [[entry.name, entry.color]] : [])));
}

export function taskVocabularyOfDefinitions(definitions: Omit<TaskDefinitions, 'sizes'>): TaskVocabulary {
	return {
		businesses: definitions.categories.map((name) => namedColourFor(name, definitions.categoryColors)),
		types: definitions.types.map((name) => namedColourFor(name, definitions.typeColors)),
		...(definitions.etcBusinessColor ? { etcBusinessColor: definitions.etcBusinessColor } : {}),
		...(definitions.etcTypeColor ? { etcTypeColor: definitions.etcTypeColor } : {})
	};
}

function namedColourFor(name: string, colors: Record<string, string> | undefined): NamedColour {
	const color = colors?.[name];
	return color ? { name, color } : { name };
}

export function vocabularyOf(value: unknown): TaskVocabulary {
	if (typeof value !== 'object' || value === null) return {};
	const record = value as Record<string, unknown>;
	return {
		businesses: namedColoursOf(record.businesses),
		types: namedColoursOf(record.types),
		etcBusinessColor: colourValueOf(record.etcBusinessColor),
		etcTypeColor: colourValueOf(record.etcTypeColor)
	};
}

function colourValueOf(value: unknown): string | undefined {
	return typeof value === 'string' && value.trim() ? value.trim() : undefined;
}

function namedColoursOf(value: unknown): NamedColour[] {
	if (!Array.isArray(value)) return [];
	return value.filter((entry): entry is NamedColour => {
		return typeof entry === 'object' && entry !== null && typeof (entry as NamedColour).name === 'string';
	});
}
