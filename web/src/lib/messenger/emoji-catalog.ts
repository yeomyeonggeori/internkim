import { emojiEntries, glyphOfEmojiName } from './emoji-glyph';
import { emojiPickerOrderLines } from './emoji-picker-order.generated';

export type CatalogEmoji = {
	name: string;
	glyph: string;
};

export const quickEmojiNames: readonly string[] = ['+1', 'heart', 'joy', 'tada', 'cry', 'eyes', 'pray', 'fire'];

export function catalogEmoji(name: string): CatalogEmoji | undefined {
	const glyph = glyphOfEmojiName(name);
	return glyph === undefined ? undefined : { name, glyph };
}

function withUnifiedSeparators(value: string): string {
	return value.replaceAll('-', '_');
}

type SearchCandidate = CatalogEmoji & { normalizedName: string };

export function searchEmoji(query: string, limit: number): CatalogEmoji[] {
	const normalizedQuery = withUnifiedSeparators(query.trim().toLowerCase());
	if (!normalizedQuery) return [];

	const seenGlyphs = new Set<string>();
	const matches: SearchCandidate[] = [];
	for (const [name, glyph] of emojiEntries()) {
		if (name.endsWith('_skin_tone')) continue;
		if (seenGlyphs.has(glyph)) continue;
		const normalizedName = withUnifiedSeparators(name.toLowerCase());
		if (!normalizedName.includes(normalizedQuery)) continue;
		seenGlyphs.add(glyph);
		matches.push({ name, glyph, normalizedName });
	}

	const startsWithQuery = (candidate: SearchCandidate): boolean =>
		candidate.normalizedName.startsWith(normalizedQuery);
	const orderedMatches = [
		...matches.filter(startsWithQuery),
		...matches.filter((candidate) => !startsWithQuery(candidate))
	];
	return orderedMatches.slice(0, limit).map(({ name, glyph }) => ({ name, glyph }));
}

export type EmojiCategory = {
	name: string;
	emoji: CatalogEmoji[];
};

const categories: EmojiCategory[] = emojiPickerOrderLines.split('\n').map((line) => {
	const [name, names] = line.split('\t');
	return {
		name,
		emoji: names
			.split(' ')
			.map((emojiName) => catalogEmoji(emojiName))
			.filter((emoji): emoji is CatalogEmoji => emoji !== undefined)
	};
});

export function emojiCategories(): readonly EmojiCategory[] {
	return categories;
}
