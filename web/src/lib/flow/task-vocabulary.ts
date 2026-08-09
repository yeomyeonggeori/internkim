import { taskSizes } from '$lib/flow/task-sizes';
import type { FlowDefinitions } from '../../routes/flow/flow-types';

export type NamedColour = { name: string; color?: string };

export type TaskVocabulary = {
	businesses?: NamedColour[];
	types?: NamedColour[];
};

export function colourOf(entry: NamedColour): string {
	if (entry.color) return entry.color;
	let hash = 0;
	for (const character of entry.name) hash = (hash * 31 + character.codePointAt(0)!) % 360;
	return `hsl(${hash} 62% 45%)`;
}

export function flowDefinitionsOf(vocabulary: TaskVocabulary): FlowDefinitions {
	const businesses = vocabulary.businesses ?? [];
	const types = vocabulary.types ?? [];
	return {
		categories: businesses.map((business) => business.name),
		categoryColors: coloursOf(businesses),
		types: types.map((type) => type.name),
		typeColors: coloursOf(types),
		sizes: taskSizes()
	};
}

function coloursOf(entries: NamedColour[]): Record<string, string> {
	return Object.fromEntries(entries.map((entry) => [entry.name, colourOf(entry)]));
}

export function vocabularyOf(value: unknown): TaskVocabulary {
	if (typeof value !== 'object' || value === null) return {};
	const record = value as Record<string, unknown>;
	return {
		businesses: namedColoursOf(record.businesses),
		types: namedColoursOf(record.types)
	};
}

function namedColoursOf(value: unknown): NamedColour[] {
	if (!Array.isArray(value)) return [];
	return value.filter((entry): entry is NamedColour => {
		return typeof entry === 'object' && entry !== null && typeof (entry as NamedColour).name === 'string';
	});
}
