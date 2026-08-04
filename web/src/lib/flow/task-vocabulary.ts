import type { FlowDefinitions, FlowSizeDefinition } from '../../routes/flow/flow-types';

export type NamedColour = { name: string; color?: string };
export type SizeDefinition = NamedColour & { maxHours?: number; score?: number; note?: string };

export type TaskVocabulary = {
	businesses?: NamedColour[];
	types?: NamedColour[];
	sizes?: SizeDefinition[];
};

// A colour is optional in the vocabulary, so one is derived from the name when
// none was chosen. The same name always lands on the same hue, which is what
// makes a board readable before anyone has picked anything.
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
		sizes: (vocabulary.sizes ?? []).map(sizeOf)
	};
}

function coloursOf(entries: NamedColour[]): Record<string, string> {
	return Object.fromEntries(entries.map((entry) => [entry.name, colourOf(entry)]));
}

function sizeOf(size: SizeDefinition): FlowSizeDefinition {
	return {
		name: size.name,
		label: size.name,
		distanceKm: 0,
		maxHours: size.maxHours ?? 0,
		score: size.score ?? 0,
		developmentExample: '',
		otherExample: '',
		note: size.note ?? ''
	};
}

export function vocabularyOf(value: unknown): TaskVocabulary {
	if (typeof value !== 'object' || value === null) return {};
	const record = value as Record<string, unknown>;
	return {
		businesses: namedColoursOf(record.businesses),
		types: namedColoursOf(record.types),
		sizes: namedColoursOf(record.sizes) as SizeDefinition[]
	};
}

function namedColoursOf(value: unknown): NamedColour[] {
	if (!Array.isArray(value)) return [];
	return value.filter((entry): entry is NamedColour => {
		return typeof entry === 'object' && entry !== null && typeof (entry as NamedColour).name === 'string';
	});
}
